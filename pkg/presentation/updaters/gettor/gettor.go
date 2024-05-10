// Copyright (c) 2021-2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gettor

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/resources"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/updaters/gettor"
)

const (
	downloadsURL    = "https://aus1.torproject.org/torbrowser/update_3/release/"
	updateFrequency = time.Hour
	releaseName     = "Tor Browser %s-%s"
	multilocale     = "ALL"
)

var (
	releaseBody = "These releases were uploaded to be distributed with gettor."

	versionDownloadCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gettor_updater_version_count",
			Help: "count each gettor updater version check",
		},
		[]string{"version"},
	)

	versionPerPlatform = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gettor_version_per_platform_count",
			Help: "the total number of gettor version per platform",
		},
		[]string{"version", "platform"})

	providerPerPlatform = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gettor_platform_per_provider_count",
			Help: "counts the version update per platform",
		},
		[]string{"platform", "provider"})
)

// updatedLinks keeps the links to be sent to the backend
// we want to keep them as a global variable to be able to retry if the backend fails
var updatedLinks = []*resources.TBLink{}

// platforms map the url json name to the platform name we use in gettor
var platforms = map[string]string{
	"download-android-aarch64.json": "android-aarch64",
	"download-android-armv7.json":   "android-armv7",
	"download-android-x86.json":     "android-x86",
	"download-android-x86_64.json":  "android-x86_64",
	"download-linux-i686.json":      "linux32",
	"download-linux-x86_64.json":    "linux64",
	"download-macos.json":           "macos",
	"download-windows-i686.json":    "win32",
	"download-windows-x86_64.json":  "win64",
}

type (
	uploadFileFunc func(binaryPath string, sigPath string) *resources.TBLink
	provider       interface {
		needsUpdate(platform string, version resources.Version) bool
		newRelease(platform string, version resources.Version) uploadFileFunc
	}
)

type providerExtRefreshLink interface {
	needsUpdateRefreshOnly(platform string, version resources.Version) bool
}

type downloadsLinks struct {
	Version string `json:"version"`
	Binary  string `json:"binary"`
	Sig     string `json:"sig"`
}

func InitUpdater(cfg *internal.Config) {
	updater := &gettor.GettorUpdater{}
	updater.Init(cfg)

	stop := make(chan struct{})
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT)
	signal.Notify(signalChan, syscall.SIGTERM)
	go func() {
		<-signalChan
		log.Printf("Caught SIGINT.")
		updater.Shutdown()
		close(stop)
	}()

	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(cfg.Updaters.Gettor.MetricsAddress, nil)

	gh := newGithubProvider(&cfg.Updaters.Gettor.Github)
	providers := []provider{gh}

	gl, err := newGitlabProvider(&cfg.Updaters.Gettor.Gitlab)
	if err != nil {
		log.Printf("cannot create GitLab provider: %v", err)
	} else {
		providers = append(providers, gl)
	}

	googleDrive, err := newGoogleDriveUpdater(&cfg.Updaters.Gettor.GoogleDriveUpdater)
	if err != nil {
		log.Printf("cannot create Google Drive provider: %v", err)
	} else {
		providers = append(providers, googleDrive)
	}

	for _, s3Config := range cfg.Updaters.Gettor.S3Updaters {
		s3Provider, err := newS3Updater(&s3Config)
		if err != nil {
			log.Printf("cannot create S3 provider: %v", err)
		}
		providers = append(providers, s3Provider)
	}

	updateIfNeeded(updater, providers)

	for {
		select {
		case <-stop:
			return
		case <-time.After(updateFrequency):
			updateIfNeeded(updater, providers)
		}
	}
}

func updateIfNeeded(updater *gettor.GettorUpdater, providers []provider) {
	tmpDir, err := ioutil.TempDir("", "gettor-")
	if err != nil {
		log.Println("Can't create temporary file:", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	for platformJSON, platform := range platforms {
		downloads, version, err := getDownloadLinks(platformJSON)
		if err != nil {
			log.Println("Error fetching downloads.json:", err)
			return
		}

		shouldDownload := false
		uploadFuncs := []uploadFileFunc{}
		for _, p := range providers {
			if p.needsUpdate(platform, version) {
				if refreshOnly, ok := p.(providerExtRefreshLink); ok {
					if !refreshOnly.needsUpdateRefreshOnly(platform, version) {
						shouldDownload = true
					}
				} else {
					shouldDownload = true
				}
				fn := p.newRelease(platform, version)
				if fn != nil {
					uploadFuncs = append(uploadFuncs, fn)
				}
				providerPerPlatform.WithLabelValues(platform, resources.NewTBLink().Provider).Inc()
			}
		}
		versionPerPlatform.WithLabelValues(downloads.Version, platform).Inc()

		if len(uploadFuncs) == 0 {
			continue
		}

		log.Println("Uploading to distributors", downloads.Binary)
		getAssetPath := getAsset
		if !shouldDownload {
			getAssetPath = constructAssetPath
		}
		binaryPath, err := getAssetPath(downloads.Binary, tmpDir)
		if err != nil {
			log.Println("Error getting asset:", err)
			continue
		}
		sigPath, err := getAssetPath(downloads.Sig, tmpDir)
		if err != nil {
			log.Println("Error getting asset:", err)
			continue
		}

		for _, fn := range uploadFuncs {
			link := fn(binaryPath, sigPath)
			if link != nil {
				updatedLinks = append(updatedLinks, link)
			}
		}

		os.Remove(binaryPath)
		os.Remove(sigPath)

		if len(updatedLinks) == 0 {
			return
		}

		err = updater.AddLinks(updatedLinks)
		if err != nil {
			log.Println("Error sending links to the backend:", err)
		} else {
			log.Println("Updated links for", platform, version.String(), "in the backend")
			updatedLinks = nil
		}
	}
}

func constructAssetPath(url string, tmpDir string) (filePath string, err error) {
	segments := strings.Split(url, "/")
	fileName := segments[len(segments)-1]
	filePath = path.Join(tmpDir, fileName)
	return fileName, nil
}

func getAsset(url string, tmpDir string) (filePath string, err error) {
	filePath, err = constructAssetPath(url, tmpDir)
	if err != nil {
		return
	}
	file, err := os.Create(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	_, err = io.Copy(file, resp.Body)
	return
}

func getDownloadLinks(platformJSON string) (downloads downloadsLinks, version resources.Version, err error) {
	resp, err := http.Get(downloadsURL + platformJSON)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	d := json.NewDecoder(resp.Body)
	err = d.Decode(&downloads)
	if err != nil {
		return
	}

	version, err = resources.Str2Version(downloads.Version)
	versionDownloadCount.WithLabelValues(version.String()).Inc()
	return
}

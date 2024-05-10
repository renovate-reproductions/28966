// Copyright (c) 2021-2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gettor

import (
	"bufio"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/core"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/delivery"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/delivery/mechanisms"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/resources"
)

const (
	DistName = "gettor"

	CommandHelp  = "help"
	CommandLinks = "links"
)

var (
	requestsCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gettor_request_total",
		Help: "The total number of gettor requests",
	},
		[]string{"command", "platform"},
	)

	linkResponseCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gettor_link_response_total",
		Help: "The total number of gettor link responses",
	},
		[]string{"platform"},
	)
)

var platformAliases = map[string]string{
	"linux":   "linux64",
	"lin":     "linux64",
	"windows": "win64",
	"win":     "win64",
	"osx":     "macos",
	"osx64":   "macos",
	"mac":     "macos",
	"android": "android-aarch64",
}

type GettorDistributor struct {
	ipc      delivery.Mechanism
	wg       sync.WaitGroup
	shutdown chan bool
	tblinks  TBLinkList

	// latest version of Tor Browser per platform
	version map[string]resources.Version

	mutex sync.RWMutex
}

// TBLinkList are indexed by platform
type TBLinkList map[string][]*resources.TBLink

type Command struct {
	Platform string
	Command  string
}

func (d *GettorDistributor) GetLinks(platform string) []*resources.TBLink {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	linkResponseCount.WithLabelValues(platform).Inc()
	return d.tblinks[platform]
}

func (d *GettorDistributor) GetAliasedLinks(platform string) []*resources.TBLink {
	requestsCount.WithLabelValues(CommandLinks, platform).Inc()

	p, exists := platformAliases[platform]
	if exists {
		platform = p
	}
	return d.GetLinks(platform)
}

func (d *GettorDistributor) ParseCommand(body io.Reader) *Command {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	command := Command{
		Platform: "",
		Command:  "",
	}

	scanner := bufio.NewScanner(body)
	requestedPlatform := ""
	for scanner.Scan() {
		if command.Platform != "" {
			break
		}

		line := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if len(line) == 0 || line[0] == '>' || (len(line) >= 3 && line[0:3] == "re:") {
			continue
		}

		for _, word := range strings.Fields(line) {
			platform, exists := platformAliases[word]
			if exists {
				requestedPlatform = word
				command.Platform = platform
				continue
			}

			_, exists = d.tblinks[word]
			if exists {
				requestedPlatform = word
				command.Platform = word
				continue
			}
		}
	}
	requestsCount.WithLabelValues(command.Command, requestedPlatform).Inc()

	if command.Platform != "" {
		command.Command = CommandLinks
	} else {
		command.Command = CommandHelp
	}

	return &command
}

func (d *GettorDistributor) SupportedPlatforms() []string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	platforms := make([]string, 0, len(platformAliases)+len(d.tblinks))
	for platform := range platformAliases {
		platforms = append(platforms, platform)
	}
	for platform := range d.tblinks {
		platforms = append(platforms, platform)
	}
	return platforms
}

// housekeeping listens to updates from the backend resources
func (d *GettorDistributor) housekeeping(rStream chan *core.ResourceDiff) {
	defer d.wg.Done()
	defer close(rStream)
	defer d.ipc.StopStream()

	for {
		select {
		case diff := <-rStream:
			d.applyDiff(diff)
		case <-d.shutdown:
			log.Printf("Shutting down housekeeping.")
			return
		}
	}
}

func (d *GettorDistributor) Init(cfg *internal.Config) {
	d.shutdown = make(chan bool)
	d.tblinks = make(TBLinkList)
	d.version = make(map[string]resources.Version)

	d.ipc = mechanisms.NewHttpsIpc(
		cfg.Backend.ResourceStreamURL(),
		"GET",
		cfg.Backend.ApiTokens[DistName])
	rStream := make(chan *core.ResourceDiff)
	req := core.ResourceRequest{
		RequestOrigin: DistName,
		ResourceTypes: cfg.Distributors.Gettor.Resources,
		Receiver:      rStream,
	}
	d.ipc.StartStream(&req)

	d.wg.Add(1)
	go d.housekeeping(rStream)
}

func (d *GettorDistributor) Shutdown() {
	close(d.shutdown)
	d.wg.Wait()
}

// applyDiff to tblinks. Ignore changes, links should not change, just appear new or be gone
func (d *GettorDistributor) applyDiff(diff *core.ResourceDiff) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if diff.FullUpdate {
		d.tblinks = make(TBLinkList)
		d.version = make(map[string]resources.Version)
	}

	needsCleanUp := map[string]struct{}{}
	for rType, resourceQueue := range diff.New {
		if rType != "tblink" {
			continue
		}
	processResource:
		for _, r := range resourceQueue {
			link, ok := r.(*resources.TBLink)
			if !ok {
				log.Println("Not valid tblink resource", r)
				continue
			}
			version, ok := d.version[link.Platform]
			if ok {
				switch version.Compare(link.Version) {
				case 1:
					// ignore resources with old versions
					continue
				case -1:
					d.version[link.Platform] = link.Version
					needsCleanUp[link.Platform] = struct{}{}
				}
			} else {
				d.version[link.Platform] = link.Version
			}

			for _, l := range d.tblinks[link.Platform] {
				if l.Uid() == link.Uid() {
					continue processResource
				}
			}
			d.tblinks[link.Platform] = append(d.tblinks[link.Platform], link)
		}
	}

	for rType, resourceQueue := range diff.Gone {
		if rType != "tblink" {
			continue
		}
		for _, r := range resourceQueue {
			link, ok := r.(*resources.TBLink)
			if !ok {
				log.Println("Not valid tblink resource", r)
				continue
			}
			_, ok = d.tblinks[link.Platform]
			if !ok {
				continue
			}
			for i, l := range d.tblinks[link.Platform] {
				if l.Link == link.Link {
					linklist := d.tblinks[link.Platform]
					d.tblinks[link.Platform] = append(linklist[:i], linklist[i+1:]...)
					break
				}
			}
		}
	}

	for platform := range needsCleanUp {
		d.deleteOldVersions(platform)
	}
}

// deleteOldVersions assumes that the mutex is already locked
func (d *GettorDistributor) deleteOldVersions(platform string) {
	newResources := []*resources.TBLink{}
	for _, r := range d.tblinks[platform] {
		if d.version[platform].Compare(r.Version) == 0 {
			newResources = append(newResources, r)
		}
	}

	if len(newResources) == 0 {
		delete(d.tblinks, platform)
	} else {
		d.tblinks[platform] = newResources
	}
}

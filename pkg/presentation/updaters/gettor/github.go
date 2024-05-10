// Copyright (c) 2021-2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gettor

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/google/go-github/v61/github"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/resources"
	"golang.org/x/oauth2"
)

const (
	githubPlatform = "github"
)

type githubProvider struct {
	client *github.Client
	ctx    context.Context
	cfg    *internal.Github
}

func newGithubProvider(cfg *internal.Github) *githubProvider {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cfg.AuthToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	return &githubProvider{client, ctx, cfg}
}

func (gh *githubProvider) needsUpdate(platform string, version resources.Version) bool {
	releases, err := gh.getReleases(platform)
	if err != nil {
		log.Println("[Github] Error fetching latest release:", err)
		return false
	}
	if len(releases) == 0 {
		log.Println("[Github] There is no previous releases for", platform, "let's create one")
		return true
	}

	for _, release := range releases {
		parts := strings.Split(*release.TagName, "-")
		if len(parts) != 2 {
			continue
		}

		releaseVersion, err := resources.Str2Version(parts[1])
		if err != nil {
			continue
		}

		// If the same version or higher exist we don't need to update
		if version.Compare(releaseVersion) != 1 {
			return false
		}
	}

	log.Println("[Github] needs update for", platform)
	return true
}

func (gh *githubProvider) newRelease(platform string, version resources.Version) uploadFileFunc {
	oldReleases, err := gh.getReleases(platform)
	if err != nil {
		log.Println("[Github] Error fetching releases:", err)
		return nil
	}

	tag := fmt.Sprintf("%s-%s", platform, version.String())
	name := fmt.Sprintf(releaseName, platform, version.String())
	release := github.RepositoryRelease{
		TagName: &tag,
		Name:    &name,
		Body:    &releaseBody,
	}
	repositoryRelease, _, err := gh.client.Repositories.CreateRelease(gh.ctx, gh.cfg.Owner, gh.cfg.Repo, &release)
	if err != nil {
		log.Println("[Github] Error creating repository:", err)
		return nil
	}

	for _, release := range oldReleases {
		_, err := gh.client.Repositories.DeleteRelease(gh.ctx, gh.cfg.Owner, gh.cfg.Repo, *release.ID)
		if err != nil {
			log.Println("[Github] Error deleting a release", release.TagName, ":", err)
		}
	}

	return func(binaryPath string, sigPath string) *resources.TBLink {
		link := resources.NewTBLink()

		for i, filePath := range []string{binaryPath, sigPath} {
			filename := path.Base(filePath)
			options := github.UploadOptions{
				Name: filename,
			}
			file, err := os.Open(filePath)
			if err != nil {
				log.Println("[Github] Couldn't open the file", filePath, "to upload:", err)
				return nil
			}
			defer file.Close()

			asset, _, err := gh.client.Repositories.UploadReleaseAsset(
				gh.ctx, gh.cfg.Owner, gh.cfg.Repo,
				*repositoryRelease.ID, &options, file)
			if err != nil {
				log.Println("[Github] Couldn't upload the file", filename, ":", err)
				return nil
			}

			if i == 0 {
				link.Link = *asset.BrowserDownloadURL
			} else {
				link.SigLink = *asset.BrowserDownloadURL
			}
		}

		link.Version = version
		link.Provider = githubPlatform
		link.Platform = platform
		link.FileName = path.Base(binaryPath)
		return link
	}
}

func (gh *githubProvider) getReleases(platform string) ([]*github.RepositoryRelease, error) {
	releases, _, err := gh.client.Repositories.ListReleases(gh.ctx, gh.cfg.Owner, gh.cfg.Repo, nil)
	if err != nil {
		return nil, err
	}

	platformReleases := []*github.RepositoryRelease{}
	for _, release := range releases {
		if strings.HasPrefix(*release.TagName, platform) {
			platformReleases = append(platformReleases, release)
		}
	}
	return platformReleases, nil
}

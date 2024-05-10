// Copyright (c) 2021-2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gettor

import (
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/core"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/delivery"
	"reflect"
	"strings"
	"testing"

	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/resources"
)

const (
	platform = "win64"
)

func TestDeleteOldVersion(t *testing.T) {
	lastVersion := resources.Version{1, 0, 0}
	oldVersion := resources.Version{0, 1, 0}
	newLink := "new"
	oldLink := "old"
	dist := GettorDistributor{
		version: map[string]resources.Version{
			platform: lastVersion,
		},
		tblinks: TBLinkList{
			platform: {
				&resources.TBLink{
					Link:    oldLink,
					Version: oldVersion,
				},
				&resources.TBLink{
					Link:    newLink,
					Version: lastVersion,
				},
				&resources.TBLink{
					Link:    oldLink + "1",
					Version: oldVersion,
				},
			},
		},
	}

	dist.deleteOldVersions(platform)

	if len(dist.tblinks[platform]) != 1 {
		t.Fatal("Wrong size of tblinks: ", dist.tblinks[platform])
	}
	if dist.tblinks[platform][0].Link != newLink {
		t.Error("Unexpected tblink:", dist.tblinks[platform][0])
	}
}

// TestGetTBLinks tests the GetTBLinks method of the GettorDistributor
func TestGetTBLinks(t *testing.T) {
	var tbLinks = []*resources.TBLink{{
		Platform: "win",
		Version:  resources.Version{0, 0, 1},
		Link:     "https://www.torproject.org/dist/torbrowser/10.0.10/torbrowser-install-win64-10.0.10_en-US.exe",
		Provider: "res",
	}, {
		Platform: "win",
		Version:  resources.Version{0, 0, 1},
		Link:     "https://www.torproject.org/dist/torbrowser/10.0.10/torbrowser-install-win64-10.0.10_en-US.executives",
		Provider: "resource",
	},
	}
	dist := GettorDistributor{
		tblinks: TBLinkList{
			platform: tbLinks,
		},
		version: map[string]resources.Version{},
	}
	got := dist.GetLinks(platform)
	if !reflect.DeepEqual(got, tbLinks) {
		t.Error("expected:", tbLinks, "got", got)
	}
}

// TestParseCommand tests the ParseCommand method of the GettorDistributor
func TestParseCommand(t *testing.T) {
	t.Run("check that the distributor parses the command correctly", func(t *testing.T) {
		expectedResult := &Command{
			Platform: platform,
			Command:  "links",
		}
		dist := GettorDistributor{
			tblinks: TBLinkList{
				platform: {
					&resources.TBLink{
						Link:    "link1",
						Version: resources.Version{0, 0, 1},
					},
					&resources.TBLink{
						Link:    "link2",
						Version: resources.Version{1, 0, 1},
					},
				},
			},
		}
		got := dist.ParseCommand(strings.NewReader("win"))
		if !reflect.DeepEqual(got, expectedResult) {
			t.Errorf("expected %v, got %v", expectedResult, got)
		}
	})
	t.Run("check that command locale and link is set when locales is empty in GettorDistributor", func(t *testing.T) {
		expectedResult := &Command{
			Platform: platform,
			Command:  "links",
		}
		dist := GettorDistributor{
			tblinks: TBLinkList{
				platform: {},
			},
		}
		got := dist.ParseCommand(strings.NewReader("win"))
		if !reflect.DeepEqual(got, expectedResult) {
			t.Errorf("expected %v, got %v", expectedResult, got)
		}
	})
	t.Run("check that help is sent if platform does not exist", func(t *testing.T) {
		expectedResult := &Command{
			Platform: "",
			Command:  "help",
		}
		dist := GettorDistributor{
			tblinks: TBLinkList{
				platform: {},
			},
		}
		got := dist.ParseCommand(strings.NewReader("winx"))
		if !reflect.DeepEqual(got, expectedResult) {
			t.Errorf("expected %v, got %v", expectedResult, got)
		}
	})
}

// TestApplyDiff tests the applyDiff method of the GettorDistributor
func TestApplyDiff(t *testing.T) {
	Version1 := resources.Version{1, 0, 0}
	Version2 := resources.Version{1, 1, 0}
	Version3 := resources.Version{1, 2, 0}
	link1 := "link1"
	link2 := "link2"
	link3 := "link3"

	tbLink := resources.NewTBLink()
	tbLink.Platform = platform
	tbLink.Version = Version2
	tbLink.Link = link2
	t.Run("check that version is updated to the new version and corresponding tblink is added", func(t *testing.T) {
		diff := &core.ResourceDiff{
			New: core.ResourceMap{resources.ResourceTypeTBLink: core.ResourceQueue{tbLink}},
		}

		dist := GettorDistributor{
			tblinks: TBLinkList{},
			version: map[string]resources.Version{
				platform: Version1,
			},
		}
		expectedVersion := Version2
		expectedtblinks := []*resources.TBLink{
			tbLink,
		}
		dist.applyDiff(diff)
		if dist.version[platform] != expectedVersion {
			t.Error("expected version:", expectedVersion, "got:", dist.version[platform])
		}
		if !reflect.DeepEqual(dist.tblinks[platform], expectedtblinks) {
			t.Error("expected tblinks:", expectedtblinks, "got:", dist.tblinks[platform])
		}
	})
	t.Run("check that updating to an old version is ignored", func(t *testing.T) {
		dist := GettorDistributor{
			tblinks: TBLinkList{
				platform: {
					&resources.TBLink{
						Link:    link1,
						Version: Version1,
					},
					&resources.TBLink{
						Link:    link2,
						Version: Version2,
					},
					&resources.TBLink{
						Link:    link3,
						Version: Version3,
					},
				},
			},
			version: map[string]resources.Version{
				platform: Version3,
			},
		}
		diff := &core.ResourceDiff{
			New: core.ResourceMap{resources.ResourceTypeTBLink: core.ResourceQueue{tbLink}},
		}

		expectedVersion := Version3
		expectedtblinks := []*resources.TBLink{
			&resources.TBLink{
				Link:    link1,
				Version: Version1,
			},
			&resources.TBLink{
				Link:    link2,
				Version: Version2,
			},
			&resources.TBLink{
				Link:    link3,
				Version: Version3,
			},
		}
		dist.applyDiff(diff)
		if dist.version[platform] != expectedVersion {
			t.Error("expected version:", expectedVersion, "got:", dist.version[platform])
		}
		if !reflect.DeepEqual(dist.tblinks[platform], expectedtblinks) {
			t.Error("expected tblinks:", expectedtblinks, "got:", dist.tblinks[platform])
		}
	})
	t.Run("check that gone tblink are removed from the resources", func(t *testing.T) {
		dist := GettorDistributor{
			tblinks: TBLinkList{
				platform: {
					&resources.TBLink{
						Link:    link1,
						Version: Version1,
					},
					&resources.TBLink{
						Link:    link2,
						Version: Version2,
					},
					&resources.TBLink{
						Link:    link3,
						Version: Version3,
					},
				},
			},
			version: map[string]resources.Version{
				platform: Version3,
			},
		}
		diff := &core.ResourceDiff{
			Gone: core.ResourceMap{resources.ResourceTypeTBLink: core.ResourceQueue{tbLink}},
		}
		expectedVersion := Version3
		expectedtblinks := []*resources.TBLink{
			&resources.TBLink{
				Link:    link1,
				Version: Version1,
			},
			&resources.TBLink{
				Link:    link3,
				Version: Version3,
			},
		}
		dist.applyDiff(diff)
		if dist.version[platform] != expectedVersion {
			t.Error("expected version:", expectedVersion, "got:", dist.version[platform])
		}
		if !reflect.DeepEqual(dist.tblinks[platform], expectedtblinks) {
			t.Error("expected tblinks:", expectedtblinks, "got:", dist.tblinks[platform])
		}
	})

}

// TestInit tests the Init method of the GettorDistributor
func TestInit(t *testing.T) {
	dist := GettorDistributor{
		ipc:      delivery.Mechanism(nil),
		shutdown: make(chan bool),
		tblinks:  TBLinkList{},
		version:  map[string]resources.Version{},
	}
	dist.Init(&internal.Config{})
	if dist.version == nil || dist.ipc == nil || dist.shutdown == nil || dist.tblinks == nil {
		t.Error("expected distributor fields to be initialized")
	}
}

// TestShutdown tests the Shutdown method of the GettorDistributor
func TestShutdown(t *testing.T) {
	dist := GettorDistributor{
		ipc:      delivery.Mechanism(nil),
		shutdown: make(chan bool),
		tblinks:  TBLinkList{},
		version:  map[string]resources.Version{},
	}
	dist.Shutdown()
	if _, ok := <-dist.shutdown; ok {
		t.Error("expected channel to be closed")
	}
}

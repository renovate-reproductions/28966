// Copyright (c) 2021-2024, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package resources

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/core"
)

type Version struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Patch int `json:"patch"`
}

func Str2Version(s string) (version Version, err error) {
	parts := strings.Split(s, ".")
	version.Major, err = strconv.Atoi(parts[0])
	if err != nil {
		return
	}

	if len(parts) > 1 {
		version.Minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return
		}
	}

	if len(parts) > 2 {
		version.Patch, err = strconv.Atoi(parts[2])
	}
	return
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Compare returns 1 if v version is higher than v2,
// 0 if they are equal and -1 if v version is lower than v2
func (v Version) Compare(v2 Version) int {
	if v.Major > v2.Major {
		return 1
	} else if v.Major < v2.Major {
		return -1
	}

	if v.Minor > v2.Minor {
		return 1
	} else if v.Minor < v2.Minor {
		return -1
	}

	if v.Patch > v2.Patch {
		return 1
	} else if v.Patch < v2.Patch {
		return -1
	}

	return 0
}

// TBLink stores a link to download Tor Browser for a certain platform
type TBLink struct {
	core.ResourceBase
	Platform     string         `json:"platform"`
	Version      Version        `json:"version"`
	Provider     string         `json:"provider"`
	FileName     string         `json:"file_name"`
	Link         string         `json:"link"`
	SigLink      string         `json:"sig_link"`
	CustomOid    *core.Hashkey  `json:"custom_oid"`
	CustomExpiry *time.Duration `json:"custom_expiry"`
}

// NewTBLink allocates and returns a new TBLink object.
func NewTBLink() *TBLink {
	tl := &TBLink{ResourceBase: *core.NewResourceBase()}
	tl.TestResult().State = core.StateFunctional
	ratio := 1.0
	tl.TestResult().Ratio = &ratio
	tl.SetType(ResourceTypeTBLink)
	return tl
}

func (tl *TBLink) IsValid() bool {
	return true
}

func (tl *TBLink) Oid() core.Hashkey {
	if tl.CustomOid != nil {
		return *tl.CustomOid
	}
	return core.NewHashkey(tl.Link)
}

func (tl *TBLink) Uid() core.Hashkey {
	return tl.Oid()
}

func (tl *TBLink) RelationIdentifiers() []string {
	return []string{}
}

func (tl *TBLink) Test() {
}

func (tl *TBLink) String() string {
	return tl.Link
}

// Expiry TBLinks that are older than a year, a newer version should have already being released
func (tl *TBLink) Expiry() time.Duration {
	if tl.CustomExpiry != nil {
		return *tl.CustomExpiry
	}
	return time.Duration(time.Hour * 24 * 365)
}

// Distributor set for this link
func (tl *TBLink) Distributor() string {
	return ""
}

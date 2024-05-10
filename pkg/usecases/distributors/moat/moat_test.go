// Copyright (c) 2021-2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package moat

import (
	"strings"
	"testing"

	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
)

var (
	config = internal.Config{
		Distributors: internal.Distributors{
			Moat: internal.MoatDistConfig{
				Resources: []string{"dummy", "obsf4"},
				TimeDistribution: internal.TimeDistributionConfig{
					NumBridgesPerRequest: 1,
				},
			},
		},
	}
)

const (
	circumventionMap = `
	{
		"cn": {
			"settings": [
				{"bridges": {"type": "snowflake", "source": "builtin"}}
			]
		},
		"de": {
			"settings": [
				{"bridges": {"type": "snowflake", "source": "bridgedb", "bridge_strings": ["test string"]}}
			]
		},
		"fr": {
			"settings": [
				{"bridges": {"type": "dummy",     "source": "bridgedb"}},
				{"bridges": {"type": "snowflake", "source": "builtin"}}
			]
		},
		"uk": {
			"settings": [
				{"bridges": {"type": "obfs4", "source": "bridgedb"}}
			]
		}
	}`
	dummyBridges = `obfs4 209.148.46.65:443 74FAD13168806246602538555B5521A0383A1875 cert=ssH+9rP8dG2NLDN2XuFw63hIO/9MNNinLmxQDpVa+7kTOa9/m+tGWT1SmSYpQ9uTBGa6Hw iat-mode=0`
)

func fetchBridges(url string) (map[string][]string, error) {
	bridgeLines := map[string][]string{"snowflake": {"snowflake 192.0.2.3:1 2B280B23E1107BB62ABFC40DDCC8824814F80A72"}}
	return bridgeLines, nil
}

func initDistributor() *MoatDistributor {
	d := MoatDistributor{
		FetchBridges: fetchBridges,
	}
	d.Init(&config)
	return &d
}

func TestCircumventionMap(t *testing.T) {
	d := initDistributor()
	defer d.Shutdown()

	err := d.LoadCircumventionMap(strings.NewReader(circumventionMap))
	if err != nil {
		t.Fatal("Can parse circumventionMap", err)
	}

	m := d.GetCircumventionMap()
	if len(m["cn"].Settings) != 1 {
		t.Fatal("Wrong length of 'cn' bridges")
	}
	if m["cn"].Settings[0].Bridges.Type != "snowflake" {
		t.Error("Wrong type of 'cn' bridge", m["cn"].Settings[0].Bridges.Type)
	}
}

func TestCircumventionSettings(t *testing.T) {
	d := initDistributor()
	defer d.Shutdown()

	err := d.LoadCircumventionMap(strings.NewReader(circumventionMap))
	if err != nil {
		t.Fatal("Can parse circumventionMap", err)
	}

	settings, err := d.GetCircumventionSettings("gb", []string{}, nil, "")
	if err != nil {
		t.Fatal("Can get circumvention settings for gb:", err)
	}
	if len(settings.Settings) != 0 {
		t.Error("Unexpected settins for 'gb'", settings)
	}
	if settings.Country != "gb" {
		t.Error("Unexpected country for 'gb'", settings.Country)
	}

	settings, err = d.GetCircumventionSettings("cn", []string{}, nil, "")
	if err != nil {
		t.Fatal("Can get circumvention settings for cn:", err)
	}
	if settings == nil {
		t.Fatal("No settins for 'cn'")
	}
	if settings.Settings[0].Bridges.Type != "snowflake" {
		t.Error("Wrong type of 'cn' settings bridge", settings.Settings[0].Bridges.Type)
	}

	settings, err = d.GetCircumventionSettings("fr", []string{}, nil, "")
	if err != nil {
		t.Fatal("Can get circumvention settings for fr:", err)
	}
	if settings == nil {
		t.Fatal("No settins for 'fr'")
	}
	if settings.Settings[0].Bridges.Type != "dummy" {
		t.Error("Wrong type of 'fr' settings bridge", settings.Settings[0].Bridges.Type)
	}
	if settings.Country != "fr" {
		t.Error("Unexpected country for 'fr'", settings.Country)
	}

	settings, err = d.GetCircumventionSettings("fr", []string{"snowflake"}, nil, "")
	if err != nil {
		t.Fatal("Can get circumvention settings for fr:", err)
	}
	if settings == nil {
		t.Fatal("No settins for 'fr'")
	}
	if settings.Settings[0].Bridges.Type != "snowflake" {
		t.Error("Now snowlfake type of 'fr' settings bridge", settings.Settings[0].Bridges.Type)
	}

	settings, err = d.GetCircumventionSettings("fr", []string{"snowflake", "dummy"}, nil, "")
	if err != nil {
		t.Fatal("Can get circumvention settings for fr:", err)
	}
	if settings == nil {
		t.Fatal("No settins for 'fr'")
	}
	if settings.Settings[0].Bridges.Type != "dummy" {
		t.Error("Wrong type of 'fr' settings bridge", settings.Settings[0].Bridges.Type)
	}
}

func TestCircumventionSettingsMappedBridgeSTrings(t *testing.T) {
	d := initDistributor()
	defer d.Shutdown()

	err := d.LoadCircumventionMap(strings.NewReader(circumventionMap))
	if err != nil {
		t.Fatal("Can parse circumventionMap", err)
	}

	settings, err := d.GetCircumventionSettings("de", []string{}, nil, "")
	if err != nil {
		t.Fatal("Can get circumvention settings for de:", err)
	}
	if settings == nil {
		t.Fatal("No settins for 'cn'")
	}
	if len(settings.Settings[0].Bridges.BridgeStrings) != 1 || settings.Settings[0].Bridges.BridgeStrings[0] != "test string" {
		t.Error("Wrong bridge strings for 'de':", settings.Settings[0].Bridges.BridgeStrings)
	}
}

func TestBuiltInBridges(t *testing.T) {
	d := initDistributor()
	defer d.Shutdown()

	bridges := d.GetBuiltInBridges([]string{})
	lines, ok := bridges["snowflake"]
	if !ok {
		t.Fatal("No snowflake bridges found")
	}
	if len(lines) != 1 {
		t.Fatal("Wrong lines", lines)
	}

	bridges = d.GetBuiltInBridges([]string{"dummy"})
	lines, ok = bridges["snowflake"]
	if ok {
		t.Fatal("snowflake bridges found")
	}

	bridges = d.GetBuiltInBridges([]string{"snowflake"})
	lines, ok = bridges["snowflake"]
	if !ok {
		t.Fatal("No snowflake bridges found")
	}
}

func TestDummyBridges(t *testing.T) {
	cfg := config
	cfg.Distributors.Moat.ShimTokens = map[string]string{"": "token"}
	d := MoatDistributor{
		FetchBridges: fetchBridges,
	}
	d.Init(&cfg)
	d.LoadDummyBridges(strings.NewReader(dummyBridges))
	defer d.Shutdown()

	err := d.LoadCircumventionMap(strings.NewReader(circumventionMap))
	if err != nil {
		t.Fatal("Can parse circumventionMap", err)
	}

	settings, err := d.GetCircumventionSettings("uk", []string{}, nil, "")
	if err != nil {
		t.Fatal("Can get circumvention settings for uk:", err)
	}
	if len(settings.Settings) != 1 {
		t.Error("Unexpected settins for 'uk'", settings)
	}

	bridgeStrings := settings.Settings[0].Bridges.BridgeStrings
	if len(bridgeStrings) == 0 {
		t.Fatal("No dummy bridgestrings for 'uk'")
	}
	if bridgeStrings[0] != dummyBridges {
		t.Error("unexpected bridgestring:", bridgeStrings[0])
	}

	settings, err = d.GetCircumventionSettings("uk", []string{}, nil, "token")
	if err != nil {
		t.Fatal("Can get circumvention settings for uk:", err)
	}
	if len(settings.Settings) != 1 {
		t.Error("Unexpected settins for 'uk'", settings)
	}

	bridgeStrings = settings.Settings[0].Bridges.BridgeStrings
	if len(bridgeStrings) != 0 {
		t.Fatal("Found bridgestrings for 'uk' when there are none in the collection")
	}
}

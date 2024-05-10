// Copyright (c) 2021-2023, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package moat

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"log"
	mrand "math/rand"
	"net"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/core"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/distributors/common"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/resources"
)

const (
	DistName              = "moat"
	builtinRefreshSeconds = time.Hour
)

var (
	NoTransportError = errors.New("No provided transport is available for this country")

	requestsCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "moat_request_total",
		Help: "The total number of requests",
	},
		[]string{"endpoint", "country"},
	)
)

// CircumventionMap maps countries to the CircumventionSettings that ara available on those countries
type CircumventionMap map[string]CircumventionSettings

type CircumventionSettings struct {
	Settings []Settings `json:"settings"`
	Country  string     `json:"country,omitempty"`
}

type Settings struct {
	Bridges BridgeSettings `json:"bridges"`
}

type BridgeSettings struct {
	Type          string   `json:"type"`
	Source        string   `json:"source"`
	BridgeStrings []string `json:"bridge_strings,omitempty"`
}

type MoatDistributor struct {
	timeDistribution      *common.TimeDistribution
	dummyHashring         *core.Hashring
	builtinBridges        map[string][]string
	circumventionMap      CircumventionMap
	circumventionDefaults CircumventionSettings
	cfg                   *internal.MoatDistConfig
	wg                    sync.WaitGroup
	shutdown              chan bool

	// FetchBridges gets the list of builtin bridgelines from a remote url
	// the bridgeLines map is indexed by bridge type
	FetchBridges func(url string) (bridgeLines map[string][]string, err error)
}

func (d *MoatDistributor) LoadCircumventionMap(r io.Reader) error {
	dec := json.NewDecoder(r)
	return dec.Decode(&d.circumventionMap)
}

func (d *MoatDistributor) LoadCircumventionDefaults(r io.Reader) error {
	dec := json.NewDecoder(r)
	return dec.Decode(&d.circumventionDefaults)
}

func (d *MoatDistributor) GetCircumventionMap() CircumventionMap {
	requestsCount.WithLabelValues("map", "").Inc()
	return d.circumventionMap
}

func (d *MoatDistributor) GetCircumventionSettings(country string, types []string, ip net.IP, shimToken string) (*CircumventionSettings, error) {
	requestsCount.WithLabelValues("settings", country).Inc()
	cc, ok := d.circumventionMap[country]
	cc.Country = country
	if !ok || len(cc.Settings) == 0 {
		// json.Marshal will return null for an empty slice unless we *make* it
		cc.Settings = make([]Settings, 0)
		return &cc, nil
	}
	return d.populateCircumventionSettings(&cc, types, ip, shimToken)
}

func (d *MoatDistributor) GetCircumventionDefaults(types []string, ip net.IP, shimToken string) (*CircumventionSettings, error) {
	requestsCount.WithLabelValues("defaults", "").Inc()
	return d.populateCircumventionSettings(&d.circumventionDefaults, types, ip, shimToken)
}

func (d *MoatDistributor) populateCircumventionSettings(cc *CircumventionSettings, types []string, ip net.IP, shimToken string) (*CircumventionSettings, error) {
	circumventionSettings := CircumventionSettings{
		Settings: make([]Settings, 0, len(cc.Settings)),
		Country:  cc.Country,
	}

	for _, settings := range cc.Settings {
		if len(types) != 0 {
			requestedType := false
			for _, t := range types {
				if t == settings.Bridges.Type {
					requestedType = true
					break
				}
			}

			if !requestedType {
				continue
			}
		}

		if len(settings.Bridges.BridgeStrings) == 0 {
			settings.Bridges.BridgeStrings = d.getBridges(settings.Bridges, ip, shimToken)
		}
		circumventionSettings.Settings = append(circumventionSettings.Settings, settings)
	}

	if len(circumventionSettings.Settings) == 0 {
		log.Println("Could not find the requested type of bridge", types)
		return nil, NoTransportError
	}

	return &circumventionSettings, nil
}

func (d *MoatDistributor) getBridges(bs BridgeSettings, ip net.IP, shimToken string) []string {
	switch bs.Source {
	case "builtin":
		bridges := d.getBuiltInBridges([]string{bs.Type})
		return bridges[bs.Type]

	case "bridgedb":
		if len(d.cfg.ShimTokens) == 0 {
			return d.timeDistribution.GetBridges(bs.Type, ip)
		}
		for _, token := range d.cfg.ShimTokens {
			if token == shimToken {
				return d.timeDistribution.GetBridges(bs.Type, ip)
			}
		}

		hashring := d.dummyHashring
		var resources []core.Resource
		if hashring.Len() <= d.cfg.TimeDistribution.NumBridgesPerRequest {
			resources = hashring.GetAll()
		} else {
			var err error
			resources, err = hashring.GetMany(common.IpHashkey(ip), d.cfg.TimeDistribution.NumBridgesPerRequest)
			if err != nil {
				log.Println("Error getting resources from the subhashring:", err)
			}
		}
		bridgestrings := []string{}
		for _, resource := range resources {
			bridgestrings = append(bridgestrings, resource.String())
		}
		return bridgestrings

	default:
		log.Println("Requested an unsuported bridge source:", bs.Source)
		return []string{}
	}

}

func (d *MoatDistributor) GetBridges(transport string, ip net.IP) []string {
	requestsCount.WithLabelValues("captcha", "").Inc()
	return d.timeDistribution.GetBridges(transport, ip)
}

func (d *MoatDistributor) GetBuiltInBridges(types []string) map[string][]string {
	requestsCount.WithLabelValues("builtin", "").Inc()
	return d.getBuiltInBridges(types)
}

func (d *MoatDistributor) getBuiltInBridges(types []string) map[string][]string {
	builtinBridges := map[string][]string{}
	if len(types) == 0 {
		builtinBridges = d.builtinBridges
	}

	for _, t := range types {
		bridges, ok := d.builtinBridges[t]
		if ok {
			builtinBridges[t] = bridges
		}
	}

	for _, bridges := range builtinBridges {
		mrand.Shuffle(len(bridges), func(i, j int) { bridges[i], bridges[j] = bridges[j], bridges[i] })
	}
	return builtinBridges
}

// housekeeping listens to updates from the backend resources
func (d *MoatDistributor) housekeeping() {
	defer d.wg.Done()

	ticker := time.NewTicker(builtinRefreshSeconds)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.fetchBuiltinBridges()
		case <-d.shutdown:
			log.Printf("Shutting down housekeeping.")
			return
		}
	}
}

func (d *MoatDistributor) fetchBuiltinBridges() {
	builtinBridges, err := d.FetchBridges(d.cfg.BuiltInBridgesURL)
	if err != nil {
		log.Println("Failed to fetch builtin bridges:", err)
	} else {
		d.builtinBridges = builtinBridges
	}
}

func (d *MoatDistributor) Init(cfg *internal.Config) {
	log.Printf("Initialising %s distributor.", DistName)

	d.cfg = &cfg.Distributors.Moat
	d.shutdown = make(chan bool)
	d.builtinBridges = make(map[string][]string)
	d.fetchBuiltinBridges()

	d.timeDistribution = &common.TimeDistribution{
		ResourceStreamURL: cfg.Backend.ResourceStreamURL(),
		ApiToken:          cfg.Backend.ApiTokens[DistName],
		Resources:         d.cfg.Resources,
		DistName:          "settings",
		Cfg:               &d.cfg.TimeDistribution,
	}
	d.timeDistribution.Start()

	d.wg.Add(1)
	go d.housekeeping()
}

func (d *MoatDistributor) Shutdown() {
	log.Printf("Shutting down %s distributor.", DistName)

	d.timeDistribution.Shutdown()
	close(d.shutdown)
	d.wg.Wait()
}

func (d *MoatDistributor) LoadDummyBridges(r io.Reader) error {
	hashring := core.NewHashring()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		bridgeline := scanner.Text()
		resource, err := resources.FromBridgeline(bridgeline)
		if err != nil {
			log.Println("Can't parse bridgeline", bridgeline, ":", err)
			continue
		}
		hashring.Add(resource)
	}

	d.dummyHashring = hashring
	return nil
}

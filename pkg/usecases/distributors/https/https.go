// Copyright (c) 2021-2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package https

import (
	"log"
	"net"
	"time"

	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/core"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/distributors/common"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/resources"
)

const (
	DistName             = "https"
	BridgeReloadInterval = time.Minute * 10
)

// HttpsDistributor contains all the context that the distributor needs to run.
type HttpsDistributor struct {
	timeDistribution *common.TimeDistribution

	cfg *internal.Config
}

// RequestBridges takes as tpe the type of the bridge requested,
// ip as the IP of the client, and ipv6 as whether IPv6 bridge is requested.
// and return a slice of bridge lines.
func (d *HttpsDistributor) RequestBridges(tpe string, ip net.IP, ipv6 bool) ([]string, error) {
	r := d.timeDistribution.GetFilteredBridges(tpe, ip, func(r core.Resource) bool {
		switch rTyped := r.(type) {
		case *resources.Transport:
			if !resources.ResourceMap[tpe].IsAddressDummy && ipv6 != (rTyped.Address.IP.To4() == nil) {
				return false
			}
		}
		return true
	})
	return r, nil
}

// Init initialises the given HTTPS distributor.
func (d *HttpsDistributor) Init(cfg *internal.Config) {
	log.Printf("Initialising %s distributor.", DistName)

	d.cfg = cfg
	log.Printf("Initialising resource stream.")
	d.timeDistribution = &common.TimeDistribution{
		ResourceStreamURL: cfg.Backend.ResourceStreamURL(),
		ApiToken:          cfg.Backend.ApiTokens[DistName],
		Resources:         d.cfg.Distributors.Https.Resources,
		DistName:          "https",
		Cfg:               &d.cfg.Distributors.Https.TimeDistribution,
	}
	d.timeDistribution.Start()
}

// Shutdown shuts down the given HTTPS distributor.
func (d *HttpsDistributor) Shutdown() {
	log.Printf("Shutting down %s distributor.", DistName)

	d.timeDistribution.Shutdown()
}

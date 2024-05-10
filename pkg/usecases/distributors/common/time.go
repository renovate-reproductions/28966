// Copyright (c) 2023, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common

import (
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/core"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/delivery"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/delivery/mechanisms"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/resources"
)

type TimeDistribution struct {
	ResourceStreamURL string
	ApiToken          string
	Resources         []string
	DistName          string
	Cfg               *internal.TimeDistributionConfig

	collection core.Collection
	wg         sync.WaitGroup
	shutdown   chan bool
	ipc        delivery.Mechanism
}

func (td *TimeDistribution) Start() {
	td.shutdown = make(chan bool)
	proportions := td.makeProportions()
	collectionConfig := core.CollectionConfig{
		StorageDir: td.Cfg.StorageDir,
		Types:      []core.TypeConfig{},
	}
	for _, rType := range td.Resources {
		typeConfig := core.TypeConfig{
			Type:          rType,
			NewResource:   resources.ResourceMap[rType].New,
			Unpartitioned: len(proportions) == 0,
			Proportions:   proportions,
		}
		collectionConfig.Types = append(collectionConfig.Types, typeConfig)
	}
	td.collection = core.NewCollection(&collectionConfig)

	log.Printf("Initialising resource stream.")
	td.ipc = mechanisms.NewHttpsIpc(td.ResourceStreamURL, "GET", td.ApiToken)
	rStream := make(chan *core.ResourceDiff)
	req := core.ResourceRequest{
		RequestOrigin: td.DistName,
		ResourceTypes: td.Resources,
		Receiver:      rStream,
	}
	td.ipc.StartStream(&req)

	td.wg.Add(1)
	go td.housekeeping(rStream)
}

func (td *TimeDistribution) Shutdown() {
	close(td.shutdown)
	td.wg.Wait()
}

// housekeeping listens to updates from the backend resources
func (td *TimeDistribution) housekeeping(rStream chan *core.ResourceDiff) {
	defer td.wg.Done()
	defer close(rStream)
	defer td.ipc.StopStream()

	for {
		select {
		case diff := <-rStream:
			td.collection.ApplyDiff(diff)
			td.collection.Save()
		case <-td.shutdown:
			return
		}
	}
}

func (td *TimeDistribution) GetBridges(tpe string, ip net.IP) []string {
	return td.GetFilteredBridges(tpe, ip, func(r core.Resource) bool {
		return true
	})
}

func (td *TimeDistribution) GetFilteredBridges(tpe string, ip net.IP, filter core.FilterFunc) []string {
	hashring := td.collection.GetHashring(td.getProportionIndex(), tpe)

	var resources []core.Resource
	if hashring.Len() <= td.Cfg.NumBridgesPerRequest {
		resources = hashring.GetAll()
	} else {
		var err error
		resources, err = hashring.GetManyFiltered(IpHashkey(ip), filter, td.Cfg.NumBridgesPerRequest)
		if err != nil {
			log.Println("Error getting resources from the subhashring:", err)
		}
	}
	bridgestrings := []string{}
	for _, resource := range resources {
		bridgestrings = append(bridgestrings, resource.String())
	}
	return bridgestrings
}

func (td *TimeDistribution) makeProportions() map[string]int {
	proportions := make(map[string]int)
	for i := 0; i < td.Cfg.NumPeriods; i++ {
		proportions[strconv.Itoa(i)] = 1
	}
	return proportions
}

func (td *TimeDistribution) getProportionIndex() string {
	if td.Cfg.NumPeriods == 0 || td.Cfg.RotationPeriodHours == 0 {
		return ""
	}

	now := int(time.Now().Unix() / (60 * 60))
	period := now / td.Cfg.RotationPeriodHours
	return strconv.Itoa(period % td.Cfg.NumPeriods)
}

func IpHashkey(ip net.IP) core.Hashkey {
	mask := net.CIDRMask(32, 128)
	if ip.To4() != nil {
		mask = net.CIDRMask(16, 32)
	}
	return core.NewHashkey(ip.Mask(mask).String())
}

// Copyright (c) 2021-2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"log"
	"sync"
	"time"

	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/core"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/delivery"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/delivery/mechanisms"
)

const (
	// FarInTheFuture determines a time span that's far enough in the future to
	// practically count as infinity.
	FarInTheFuture = time.Hour * 24 * 365 * 100
	// MaxResources determines the maximum number of resources that we're
	// willing to buffer before sending a request to bridgestrap.
	MaxResources = 25
)

// BridgeTestRequest represents requests for bridgestrap and onbasca.  Here's what its
// API look like: https://gitlab.torproject.org/phw/bridgestrap#input
type BridgeTestRequest struct {
	BridgeLines []string `json:"bridge_lines"`
}

// BridgeTest represents the status of a single bridge in bridgestrap's
// response.
type BridgeTest struct {
	Functional bool       `json:"functional"`
	LastTested *time.Time `json:"last_tested"`
	Ratio      *float64   `json:"ratio"`
	Error      string     `json:"error,omitempty"`
}

// BridgesTestResponse represents bridgestrap and onbasca's responses.
type BridgeTestResponse struct {
	Bridges map[string]*BridgeTest `json:"bridge_results"`
	Time    float64                `json:"time"`
	Error   string                 `json:"error,omitempty"`
}

// ResourceTestPool implements a pool to which we add resources until it's time
// to send them to bridgestrap for testing.
type ResourceTestPool struct {
	sync.Mutex
	flushTimeout            time.Duration
	shutdown                chan bool
	pending                 chan core.Resource
	bridgestrap             delivery.Mechanism
	onbasca                 delivery.Mechanism
	bandwidthRatioThreshold float64
	inProgress              map[string]bool
}

// NewResourceTestPool returns a new resource test pool.
func NewResourceTestPool(bridgestrapEndpoint string, bridgestrapToken string, onbascaEndpoint string, onbascaToken string, bandwidthRatioThreshold float64) *ResourceTestPool {
	p := &ResourceTestPool{}
	p.flushTimeout = time.Minute
	p.shutdown = make(chan bool)
	p.pending = make(chan core.Resource)
	p.bridgestrap = mechanisms.NewHttpsIpc(bridgestrapEndpoint, "GET", bridgestrapToken)
	p.onbasca = mechanisms.NewHttpsIpc(onbascaEndpoint, "GET", onbascaToken)
	p.bandwidthRatioThreshold = bandwidthRatioThreshold
	p.inProgress = make(map[string]bool)
	go p.dispatch()

	return p
}

// GetTestFunc returns a function that's executed when a new resource is added
// to rdsys's backend.  The function takes as input a resource and submits it
// to our testing pool.
func (p *ResourceTestPool) GetTestFunc() func(r core.Resource) {
	return func(r core.Resource) {
		p.pending <- r
	}
}

// Stop stops the test pool by signalling to the dispatcher that it's time to
// shut down.
func (p *ResourceTestPool) Stop() {
	close(p.shutdown)
}

// alreadyInProgress returns 'true' if the given bridge line is being tested
// right now.
func (p *ResourceTestPool) alreadyInProgress(bridgeLine string) bool {
	p.Lock()
	defer p.Unlock()

	if _, exists := p.inProgress[bridgeLine]; exists {
		return true
	}
	p.inProgress[bridgeLine] = true
	return false
}

// dispatch handles the following requests:
// 1) Incoming resources to be tested
// 2) A timer whose expiry signals that it's time to test bridges
// 3) A shutdown signal, indicating that the function should return
func (p *ResourceTestPool) dispatch() {
	defer log.Printf("Shutting down resource pool ticker.")
	log.Printf("Starting resource pool ticker.")

	ticker := time.NewTicker(FarInTheFuture)
	rMap := make(map[string]core.Resource)
	for {
		select {
		case <-ticker.C:
			log.Println("Test pool timer expired.  Testing resources.")
			go p.testResources(rMap)
			rMap = make(map[string]core.Resource)
		case r := <-p.pending:
			if p.alreadyInProgress(r.String()) {
				break
			}

			// We got a new resource to test.  Start timer if our pool was
			// empty.
			if len(rMap) == 0 {
				log.Println("Starting test pool timer.")
				ticker.Reset(p.flushTimeout)
			}
			rMap[r.String()] = r

			// Test resources if our pool is full.
			if len(rMap) == MaxResources {
				log.Println("Test pool reached capacity.  Resetting timer and testing resources.")
				ticker.Reset(FarInTheFuture)
				go p.testResources(rMap)
				rMap = make(map[string]core.Resource)
			}
		case <-p.shutdown:
			return
		}
	}
}

// testResources puts all resources that are currently in our pool into a
// bridgestrap request and an onbasca request and sends them to our
// bridgestrap and onbasca instances for testing.
// The testing results are then added to each resource's state.
func (p *ResourceTestPool) testResources(rMap map[string]core.Resource) {
	defer func() {
		p.Lock()
		for bridgeLine := range rMap {
			delete(p.inProgress, bridgeLine)
		}
		p.Unlock()
	}()

	if len(rMap) == 0 {
		return
	}

	p.testBridgestrap(rMap)
	p.testOnbasca(rMap)
}

func (p *ResourceTestPool) testBridgestrap(rMap map[string]core.Resource) {
	req := BridgeTestRequest{}
	resp := BridgeTestResponse{}
	for bridgeLine := range rMap {
		req.BridgeLines = append(req.BridgeLines, bridgeLine)
	}

	if err := p.bridgestrap.MakeJsonRequest(req, &resp); err != nil {
		log.Printf("Bridgestrap request failed: %s", err)
		return
	}
	if resp.Error != "" {
		log.Printf("Bridgestrap test failed: %s", resp.Error)
		return
	}

	numFunctional, numDysfunctional := 0, 0
	for bridgeLine, bridgeTest := range resp.Bridges {
		r, exists := rMap[bridgeLine]
		if !exists {
			log.Printf("Bug: %q not in our resource test pool.", bridgeLine)
			continue
		}

		rTest := r.TestResult()
		if bridgeTest.LastTested != nil {
			rTest.LastTested = *bridgeTest.LastTested
		}
		rTest.Error = bridgeTest.Error
		if bridgeTest.Functional {
			numFunctional++
			rTest.State = core.StateFunctional
		} else {
			numDysfunctional++
			rTest.State = core.StateDysfunctional
		}
	}
	log.Printf("Tested %d resources: %d functional and %d dysfunctional.",
		len(resp.Bridges), numFunctional, numDysfunctional)
}

func (p *ResourceTestPool) testOnbasca(rMap map[string]core.Resource) {
	req := BridgeTestRequest{}
	resp := BridgeTestResponse{}
	for bridgeLine := range rMap {
		req.BridgeLines = append(req.BridgeLines, bridgeLine)
	}

	numSpeedAccepted, numSpeedRejected := 0, 0
	if err := p.onbasca.MakeJsonRequest(req, &resp); err != nil {
		log.Printf("Onbasca request failed: %s", err)
		return
	}
	if resp.Error != "" {
		log.Printf("Onbasca test failed: %s", resp.Error)
		return
	}

	for bridgeLine, bridgeTest := range resp.Bridges {
		r, exists := rMap[bridgeLine]
		if !exists {
			log.Printf("Bug: %q not in our resource test pool.", bridgeLine)
			continue
		}

		rTest := r.TestResult()
		if bridgeTest.Error != "" {
			//Onbasca sends an error message for bridges that are not available at the moment they are tested
			// or else have timed out. We count these are having SpeedRejected
			log.Println("Onbasca gave an error testing the bridge:", bridgeTest.Error)
			rTest.Ratio = nil
			rTest.Speed = core.SpeedUntested
			numSpeedRejected++
		} else if bridgeTest.Ratio != nil && *bridgeTest.Ratio == 0 && bridgeTest.Functional {
			// Since onbasca doesn't test bridges when a request is sent, but rather adds them to a queue to be tested later,
			// a Functional bridge with Ratio set to 0 indicates an untested bridge that should not be rejected.
			rTest.Ratio = nil
			rTest.Speed = core.SpeedUntested
		} else {
			if *bridgeTest.Ratio < p.bandwidthRatioThreshold {
				rTest.Speed = core.SpeedRejected
				numSpeedRejected++
			} else {
				rTest.Speed = core.SpeedAccepted
				numSpeedAccepted++
			}
			rTest.Ratio = bridgeTest.Ratio
		}
	}
	log.Printf("Tested %d resources: %d have acceptable bandwidth and %d have unacceptable bandwidth.",
		len(resp.Bridges), numSpeedAccepted, numSpeedRejected)
}

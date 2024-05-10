// Copyright (c) 2021-2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"testing"
	"time"

	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/core"
)

// DummyDelivery is a drop-in replacement for our HTTPS interface and
// facilitates testing.
type DummyBridgeTestDelivery struct{}

func (d *DummyBridgeTestDelivery) StartStream(*core.ResourceRequest) {}
func (d *DummyBridgeTestDelivery) StopStream()                       {}

func (d *DummyBridgeTestDelivery) MakeJsonRequest(req interface{}, resp interface{}) error {
	var x float64 = 5.0
	resp.(*BridgeTestResponse).Bridges = make(map[string]*BridgeTest)
	for _, bridgeLine := range req.(BridgeTestRequest).BridgeLines {
		resp.(*BridgeTestResponse).Bridges[bridgeLine] = &BridgeTest{Functional: true, Ratio: &x}
	}
	return nil
}

func TestInProgress(t *testing.T) {

	bridgeLine := "dummy"
	p := NewResourceTestPool("", "", "", "", 1)

	if p.alreadyInProgress(bridgeLine) == true {
		t.Fatal("bridge line isn't currently being tested")
	}

	p.inProgress[bridgeLine] = true

	if p.alreadyInProgress(bridgeLine) != true {
		t.Fatal("bridge line is currently being tested")
	}
}

func TestDispatch(t *testing.T) {

	d := core.NewDummy(0, 0)
	p := NewResourceTestPool("", "", "", "", 1)
	p.bridgestrap = &DummyBridgeTestDelivery{}
	p.onbasca = &DummyBridgeTestDelivery{}
	// Set flush timeout to a nanosecond, so it triggers practically instantly.
	p.flushTimeout = time.Nanosecond
	defer p.Stop()

	p.pending <- d
	d.TestResult().State = core.StateUntested
	d.TestResult().Speed = core.SpeedUntested
	p.pending <- d
	time.Sleep(10 * time.Millisecond)

	if d.TestResult().State == core.StateUntested || d.TestResult().Speed == core.SpeedUntested {
		t.Fatal("resource should not be untested")
	}
}

func TestTestFunc(t *testing.T) {

	p := NewResourceTestPool("", "", "", "", 1)
	p.bridgestrap = &DummyBridgeTestDelivery{}
	p.onbasca = &DummyBridgeTestDelivery{}
	defer p.Stop()

	f := p.GetTestFunc()
	dummies := [25]*core.Dummy{}
	for i := 0; i < len(dummies); i++ {
		k := core.Hashkey(i)
		dummies[i] = core.NewDummy(k, k)
		f(dummies[i])
	}

	// Were all states set correctly?
	for i := 0; i < len(dummies); i++ {
		if dummies[i].TestResult().State != core.StateFunctional {
			t.Fatal("resource state was set incorrectly", dummies[i].TestResult().State)
		}
	}
	// Were all ratios set correctly?
	for i := 0; i < len(dummies); i++ {
		if dummies[i].TestResult().Speed != core.SpeedAccepted {
			t.Fatal("resource speed was set incorrectly", dummies[i].TestResult().Speed)
		}
	}
}

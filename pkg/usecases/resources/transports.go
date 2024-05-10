// Copyright (c) 2021-2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package resources

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/core"
)

const (
	bridgelinePrefix = "Bridge"
)

// TestFunc takes as input a resource and tests it.
type TestFunc func(r core.Resource)

// Transport represents a Tor bridge's pluggable transport.
type Transport struct {
	BridgeBase
	Parameters map[string]string `json:"params,omitempty"`
	testFunc   TestFunc
}

// NewTransport returns a new Transport object.
func NewTransport() *Transport {
	t := &Transport{BridgeBase: BridgeBase{ResourceBase: *core.NewResourceBase()}}
	// As of 2020-05-19, all of our pluggable transports are based on TCP, so
	// we might as well make it the default for now.
	t.Protocol = ProtoTypeTCP
	t.Parameters = make(map[string]string)
	return t
}

// FromBridgeline parses the bridgeline to create a Transport struct
func FromBridgeline(bridgeline string) (*Transport, error) {
	bridgeline = strings.TrimPrefix(bridgeline, bridgelinePrefix)
	bridgeline = strings.TrimSpace(bridgeline)
	bridgeParts := strings.Split(bridgeline, " ")

	var bridge Transport
	bridge.RType = bridgeParts[0]
	bridge.Fingerprint = bridgeParts[2]

	addrParts := strings.Split(bridgeParts[1], ":")
	if len(addrParts) != 2 {
		return nil, fmt.Errorf("Malformed address %s", bridgeParts[1])
	}
	addr, err := net.ResolveIPAddr("", addrParts[0])
	if err != nil {
		return nil, err
	}
	bridge.Address = IPAddr{IPAddr: *addr}
	port, err := strconv.Atoi(addrParts[1])
	if err != nil {
		return nil, fmt.Errorf("Can't convert port to integer: %s", err)
	}
	bridge.Port = uint16(port)

	bridge.Parameters = make(map[string]string)
	for _, param := range bridgeParts[3:] {
		paramParts := strings.Split(param, "=")
		if len(paramParts) != 2 {
			return nil, fmt.Errorf("Malformed param %s", param)
		}
		bridge.Parameters[paramParts[0]] = paramParts[1]
	}
	return &bridge, nil
}

// SetTest sets the resource's test result to the given ResourceTest.
func (t *Transport) SetTestFunc(f TestFunc) {
	t.testFunc = f
}

// Test runs a resource's test if a TestFunc was set for it.
func (t *Transport) Test() {
	if t.testFunc != nil {
		t.testFunc(t)
	}
}

func (t *Transport) String() string {

	var args []string
	for key, value := range t.Parameters {
		args = append(args, fmt.Sprintf("%s=%s", key, value))
	}
	// Guarantee deterministic ordering of our resource's string
	// representation.  The exact order doesn't matter because Tor doesn't
	// care.
	sort.Strings(args)

	strRep := fmt.Sprintf("%s %s:%d %s %s",
		t.Type(), PrintTorAddr(&t.Address), t.Port, t.Fingerprint, strings.Join(args, " "))
	return strings.TrimSpace(strRep)
}

func (t *Transport) IsValid() bool {
	return t.Type() != "" && t.Address.String() != "" && t.Port != 0
}

func (t *Transport) Expiry() time.Duration {
	return time.Duration(time.Hour * 3)
}

func (t *Transport) Oid() core.Hashkey {
	return core.NewHashkey(t.String() + "|" + t.BridgeBase.oidString())
}

// Uid simply returns the bridge line as a Hashkey. For PTs, we don't
// distinguish between unique and object IDs because some Tor bridges run more
// than one PT of the same type, e.g.:
//
//	obfs3 1.1.1.1:111 0123456789ABCDEF0123456789ABCDEF01234567
//	obfs3 2.2.2.2:222 0123456789ABCDEF0123456789ABCDEF01234567
//
// If a PT's Uid is TYPE || FINGERPRINT, then rdsys would get confused because
// the above two PTs would keep changing its Oid.
func (t *Transport) Uid() core.Hashkey {
	return core.NewHashkey(t.String())
}

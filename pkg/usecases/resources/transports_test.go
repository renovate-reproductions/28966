// Copyright (c) 2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package resources

import (
	"fmt"
	"testing"
)

const (
	tpe          = "obfs4"
	ip           = "100.77.53.79"
	port         = uint16(38248)
	fingerprint  = "7DFCB47E84DA8F6D1030F370F2E308D574281E77"
	fingerprint2 = "AAAAB47E84DA8F6D1030F370F2E308D574281E77"
)

var (
	params = map[string]string{
		"cert":     "61126de1b795b976f3ac878f48e88fa77a87d7308ba57c7642b9e1068403a496",
		"iat-mode": "0",
	}
)

func TestFromBridgeline(t *testing.T) {
	bridgeline := fmt.Sprintf("%s %s:%d %s cert=%s iat-mode=%s", tpe, ip, port, fingerprint, params["cert"], params["iat-mode"])
	bridgelinePrefix := fmt.Sprintf("Bridge %s %s:%d %s cert=%s iat-mode=%s", tpe, ip, port, fingerprint, params["cert"], params["iat-mode"])
	for _, bl := range []string{bridgeline, bridgelinePrefix} {
		bridge, err := FromBridgeline(bl)
		if err != nil {
			t.Fatalf("Error loading bridge %s: %v", bl, err)
		}

		if bridge.Type() != tpe {
			t.Errorf("Wrong type: %s", bridge.Type())
		}
		if bridge.Address.String() != ip {
			t.Errorf("Wrong ip: %s", bridge.Address.String())
		}
		if bridge.Port != port {
			t.Errorf("Wrong port: %d", bridge.Port)
		}
		if bridge.Fingerprint != fingerprint {
			t.Errorf("Wrong fingerprint: %s", bridge.Fingerprint)
		}
		if len(bridge.Parameters) != 2 {
			t.Errorf("Wrong parameters: %v", bridge.Parameters)
		}
		for k, v := range params {
			if bridge.Parameters[k] != v {
				t.Errorf("Wrong parameter %s: %s", k, bridge.Parameters[k])
			}
		}
	}
}

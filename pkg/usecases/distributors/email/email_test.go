// Copyright (c) 2024, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package email

import (
	"strings"
	"testing"

	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
)

var (
	dist = EmailDistributor{
		cfg: &internal.EmailDistConfig{
			Resources:      []string{"obfs4", "vanilla"},
			AllowedDomains: []string{"example.com"},
		},
	}
)

func TestParseAddress(t *testing.T) {
	result := "alice@example.com"
	cases := []string{"alice@example.com", "Alice <alice@example.com>", "alice+tag@example.com", "Alice <alice+tag@example.com>", "alice+tag+second@example.com"}
	for _, testAddress := range cases {
		address, err := dist.ParseAddress(testAddress)
		if err != nil {
			t.Fatal("Got an error cleaning address", testAddress, err)
		}
		if address != result {
			t.Error("Address", testAddress, "didn't get properly cleaned:", address)
		}
	}
}

func TestParseAddressErrors(t *testing.T) {
	cases := []string{"alice@example.net", "alice@noexample.com", "alice@example.com@otherdomain.net", "alice@example.com <alice@otherdomain.net>"}
	for _, testAddress := range cases {
		address, err := dist.ParseAddress(testAddress)
		if err == nil {
			t.Error("Parsing", testAddress, "didn't get an expected error:", address)
		}
	}
}

func TestParseCommand(t *testing.T) {
	cases := map[string]Command{
		"":                              {Type: "obfs4", IPv6: false},
		"not a valid command":           {Type: "obfs4", IPv6: false},
		" get ipv6":                     {Type: "obfs4", IPv6: true},
		"some text\nget vanilla\nipv6":  {Type: "vanilla", IPv6: false},
		"get transport vanilla":         {Type: "vanilla", IPv6: false},
		"   get ipv6 transport vanilla": {Type: "vanilla", IPv6: true},
		"get obfs4":                     {Type: "obfs4", IPv6: false},
	}
	for body, command := range cases {
		c := dist.ParseCommand(strings.NewReader(body))
		if c.Type != command.Type {
			t.Error("Parsing", body, "didn't get exptected type:", command.Type, "=>", c.Type)
		}
		if c.IPv6 != command.IPv6 {
			t.Error("Parsing", body, "didn't get exptected ipv6:", command.IPv6, "=>", c.IPv6)
		}
	}
}

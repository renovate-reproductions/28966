// Copyright (c) 2021-2023, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"

	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
)

func main() {
	cfg, close, err := internal.ParseFlags()
	if err != nil {
		log.Fatal(err)
	}
	defer close()

	b := internal.BackendContext{}
	b.InitBackend(cfg)
}

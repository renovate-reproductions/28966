// Copyright (c) 2021-2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"log"

	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
	gettorUpdater "gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/presentation/updaters/gettor"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/updaters/gettor"
)

func main() {
	var updName string
	flag.StringVar(&updName, "name", "", "Updater name.")
	cfg, close, err := internal.ParseFlags()
	if err != nil {
		log.Fatal(err)
	}
	defer close()

	if updName == "" {
		log.Fatal("No updater name provided.  The argument -name is mandatory.")
	}

	var constructors = map[string]func(*internal.Config){
		gettor.UpdName: gettorUpdater.InitUpdater,
	}
	runFunc, exists := constructors[updName]
	if !exists {
		log.Fatalf("Updater %q not found.", updName)
	}

	log.Printf("Starting updater %q.", updName)
	runFunc(cfg)
}

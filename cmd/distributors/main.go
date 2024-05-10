// Copyright (c) 2021-2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"log"

	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
	emailMail "gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/presentation/distributors/email"
	gettorMail "gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/presentation/distributors/gettor"
	httpsUI "gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/presentation/distributors/https"
	moatWeb "gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/presentation/distributors/moat"
	stubWeb "gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/presentation/distributors/stub"
	telegramBot "gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/presentation/distributors/telegram"
	whatsapp "gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/presentation/distributors/whatsapp"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/distributors/email"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/distributors/gettor"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/distributors/https"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/distributors/moat"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/distributors/stub"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/distributors/telegram"
)

func main() {
	var distName string
	flag.StringVar(&distName, "name", "", "Distributor name.")
	cfg, close, err := internal.ParseFlags()
	if err != nil {
		log.Fatal(err)
	}
	defer close()

	if distName == "" {
		log.Fatal("No distributor name provided.  The argument -name is mandatory.")
	}

	var constructors = map[string]func(*internal.Config){
		https.DistName:    httpsUI.InitFrontend,
		stub.DistName:     stubWeb.InitFrontend,
		gettor.DistName:   gettorMail.InitFrontend,
		email.DistName:    emailMail.InitFrontend,
		moat.DistName:     moatWeb.InitFrontend,
		telegram.DistName: telegramBot.InitFrontend,
		whatsapp.DistName: whatsapp.InitFrontend,
	}
	runFunc, exists := constructors[distName]
	if !exists {
		log.Fatalf("Distributor %q not found.", distName)
	}

	log.Printf("Starting distributor %q.", distName)
	runFunc(cfg)
}

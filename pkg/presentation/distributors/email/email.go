// Copyright (c) 2024, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package email

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/presentation/distributors/common"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/distributors/email"
)

// InitFrontend is the entry point to email frontend. It will connect
// to it's IMAP account and process any incoming email until it receives a
// SIGINT.
func InitFrontend(cfg *internal.Config) {
	dist := &email.EmailDistributor{}
	dist.Init(cfg)

	handler := func(msg *mail.Message, send common.SendFunction) error {
		address, err := dist.ParseAddress(msg.Header.Get("From"))
		if err != nil {
			if !errors.Is(err, email.NotAllowedDomain) {
				log.Println(err)
			}
			return nil
		}

		subject := msg.Header.Get("Subject")
		msgBody := io.MultiReader(strings.NewReader(subject+"\n"), msg.Body)
		command := dist.ParseCommand(msgBody)

		resources := dist.GetResources(address, command)
		bridgeLines := []string{}
		for _, r := range resources {
			bridgeLines = append(bridgeLines, r.String())
		}
		if len(bridgeLines) == 0 {
			bridgeLines = append(bridgeLines, noBridges)
		}

		replyBody := fmt.Sprintf(body, strings.Join(bridgeLines, joinLines))
		return send("Re: "+subject, replyBody)
	}

	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(cfg.Distributors.Gettor.MetricsAddress, nil)

	common.StartEmail(
		&cfg.Distributors.Gettor.Email,
		cfg,
		dist,
		handler,
	)
}

const (
	body = `[This is an automated email.]

Here is your bridge:

%s

If you are using Tor Browser:

1. Choose "☰ ▸ Settings ▸ Tor" to open your Tor settings.

2. In the "Bridges" section, enter your bridge in the "Provide a bridge" field.

If you are using Tails, enter your bridge in the Tor Connection assistant.

If these bridges are not what you need, reply to this email with one of
the following commands in the message body:

  get bridges            (Request default Tor bridges.)
  get ipv6               (Request IPv6 bridges.)
  get transport obfs4    (Request obfs4 obfuscated bridges.)
  get vanilla            (Request unobfuscated Tor bridges.)
`
	joinLines = `

If it doesn't work, you can try this other bridge:

`
	noBridges = "There are not bridges available of the requested type"
)

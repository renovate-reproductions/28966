// Copyright (c) 2024, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package email

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/core"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/delivery"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/delivery/mechanisms"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/resources"
)

const (
	DistName = "email"
)

var (
	NotAllowedDomain = errors.New("This email provider is not allowed to request bridges.")

	requestsCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "email_request_total",
		Help: "The total number of email requests",
	},
		[]string{"type", "ipv6", "provider"},
	)
	rejectedCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "email_rejected_total",
		Help: "The total number of email rejected requests",
	},
		[]string{"reason"},
	)
)

type EmailDistributor struct {
	collection core.Collection
	cfg        *internal.EmailDistConfig
	ipc        delivery.Mechanism
	wg         sync.WaitGroup
	shutdown   chan bool
}

type Command struct {
	Type string
	IPv6 bool
}

func (d *EmailDistributor) Init(cfg *internal.Config) {
	log.Printf("Initialising %s distributor.", DistName)
	d.cfg = &cfg.Distributors.Email
	d.shutdown = make(chan bool)

	collectionConfig := core.CollectionConfig{}
	for _, rType := range d.cfg.Resources {
		collectionConfig.Types = append(collectionConfig.Types, core.TypeConfig{
			Type:          rType,
			NewResource:   resources.ResourceMap[rType].New,
			Unpartitioned: true,
		})
	}
	d.collection = core.NewCollection(&collectionConfig)

	log.Printf("Initialising resource stream.")
	d.ipc = mechanisms.NewHttpsIpc(
		cfg.Backend.ResourceStreamURL(),
		"GET",
		cfg.Backend.ApiTokens[DistName])
	rStream := make(chan *core.ResourceDiff)
	req := core.ResourceRequest{
		RequestOrigin: DistName,
		ResourceTypes: d.cfg.Resources,
		Receiver:      rStream,
	}
	d.ipc.StartStream(&req)

	d.wg.Add(1)
	go d.housekeeping(rStream)
}

// housekeeping listens to updates from the backend resources
func (d *EmailDistributor) housekeeping(rStream chan *core.ResourceDiff) {
	defer d.wg.Done()
	defer close(rStream)
	defer d.ipc.StopStream()

	for {
		select {
		case diff := <-rStream:
			d.collection.ApplyDiff(diff)
		case <-d.shutdown:
			log.Printf("Shutting down housekeeping.")
			return
		}
	}
}

func (d *EmailDistributor) Shutdown() {
	log.Printf("Shutting down %s distributor.", DistName)

	close(d.shutdown)
	d.wg.Wait()
}

func (d *EmailDistributor) GetResources(address string, command *Command) []core.Resource {
	requestsCount.WithLabelValues(command.Type, strconv.FormatBool(command.IPv6), strings.Split(address, "@")[1]).Inc()

	now := time.Now().Unix() / (60 * 60)
	period := now / int64(d.cfg.RotationPeriodHours)
	hashKey := core.NewHashkey(fmt.Sprintf("%s-%d", address, period))

	filterFunc := func(r core.Resource) bool {
		switch rTyped := r.(type) {
		case *resources.Transport:
			if !resources.ResourceMap[command.Type].IsAddressDummy && command.IPv6 != (rTyped.Address.IP.To4() == nil) {
				return false
			}
		}
		return true

	}

	hashring := d.collection.GetHashring("", command.Type)
	res, err := hashring.GetManyFiltered(hashKey, filterFunc, d.cfg.NumBridgesPerRequest)
	if err != nil {
		log.Println("Error getting resources from the hashring:", err)
	}
	return res
}

// ParseEmailAddress gets an email header (like "Name <me+tag@example.com>") and returns a cleaned up address (like "me@example.com").
// It will return an error if the email domain is not part of the allowed domains or the email header is malformed.
// This method should be called to clean the address before using it as parameter for GetResources
func (d *EmailDistributor) ParseAddress(emailAddress string) (string, error) {
	a, err := mail.ParseAddress(emailAddress)
	if err != nil {
		rejectedCount.WithLabelValues("invalid").Inc()
		return "", err
	}
	address := a.Address

	// Check that the domain is on the list of allowed ones
	parts := strings.Split(address, "@")
	if len(parts) != 2 {
		rejectedCount.WithLabelValues("invalid").Inc()
		return "", fmt.Errorf("Not valid email address: %s", address)
	}
	domain := parts[1]
	found := false
	for _, d := range d.cfg.AllowedDomains {
		if d == domain {
			found = true
			break
		}
	}
	if !found {
		rejectedCount.WithLabelValues("domain").Inc()
		return "", NotAllowedDomain
	}

	// Remove "+" as some providers allow to use them as tags on the same account
	parts = strings.Split(address, "+")
	if len(parts) > 1 {
		address = parts[0] + "@" + domain
	}
	return address, nil
}

func (d *EmailDistributor) ParseCommand(body io.Reader) *Command {
	command := Command{
		Type: d.cfg.Resources[0],
	}

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if len(line) == 0 || line[0] == '>' || (len(line) >= 3 && line[0:3] == "re:") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "get" {
			continue
		}

		for _, word := range fields {
			if word == "ipv6" {
				command.IPv6 = true
				continue
			}

			for _, r := range d.cfg.Resources {
				if word == r {
					command.Type = word
					break
				}
			}
		}
	}

	return &command
}

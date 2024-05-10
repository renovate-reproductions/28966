// Copyright (c) 2021-2024, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/NullHypothesis/zoossh"
	"github.com/prometheus/client_golang/prometheus"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/core"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/resources"
)

const (
	KrakenTickerInterval  = 30 * time.Minute
	MinTransportWords     = 3
	MinFunctionalFraction = 0.5
	MinRatioFraction      = 0.5
	TransportPrefix       = "transport"
	ExtraInfoPrefix       = "extra-info"
	RecordEndPrefix       = "-----END SIGNATURE-----"
)

type flicker struct {
	speed     int
	flickered bool
}

func InitKraken(cfg *Config, shutdown chan bool, ready chan bool, bCtx *BackendContext) {
	log.Println("Initialising resource kraken.")
	ticker := time.NewTicker(KrakenTickerInterval)
	defer ticker.Stop()

	rcol := &bCtx.Resources
	testFunc := bCtx.rTestPool.GetTestFunc()
	// Immediately parse bridge descriptor when we're called, and let caller
	// know when we're done.
	reloadBridgeDescriptors(cfg, rcol, testFunc)
	currentRatios := calcTestedResources(bCtx.metrics, nil, rcol)
	ready <- true
	bCtx.metrics.updateDistributors(cfg, rcol)
	for {
		select {
		case <-shutdown:
			log.Printf("Kraken shut down.")
			return
		case <-ticker.C:
			log.Println("Kraken's ticker is ticking.")
			reloadBridgeDescriptors(cfg, rcol, testFunc)
			pruneExpiredResources(rcol)
			currentRatios = calcTestedResources(bCtx.metrics, currentRatios, rcol)
			bCtx.metrics.updateDistributors(cfg, rcol)
			log.Printf("Backend resources: %s", rcol)
		}
	}
}

// calcTestedResources determines the fraction of each resource state per
// resource type and exposes them via Prometheus.  The function can tell us
// that e.g. among all obfs4 bridges, 0.2 are untested, 0.7 are functional, and
// 0.1 are dysfunctional.
func calcTestedResources(metrics *Metrics, currentRatios map[core.Hashkey]flicker, rcol *core.BackendResources) map[core.Hashkey]flicker {
	metrics.Resources.Reset()

	newRatios := make(map[core.Hashkey]flicker)
	functionalCount := 0.
	acceptedCount := 0.
	numResources := 0.
	for rName, hashring := range rcol.Collection {
		stateCount := map[int]int{
			core.StateUntested:      0,
			core.StateFunctional:    0,
			core.StateDysfunctional: 0,
		}
		ratioCount := map[int]int{
			core.SpeedUntested: 0,
			core.SpeedAccepted: 0,
			core.SpeedRejected: 0,
		}

		for _, r := range hashring.GetAll() {
			rTest := r.TestResult()
			stateCount[rTest.State] += 1
			ratioCount[rTest.Speed] += 1
			newRatios[r.Uid()] = flicker{
				speed:     rTest.Speed,
				flickered: false,
			}

			histRatio := 0.0
			const maxRatioVal = 3.0
			const minRatioVal = 0.0
			if rTest.Ratio != nil {
				if *rTest.Ratio <= minRatioVal {
					histRatio = minRatioVal
				} else if *rTest.Ratio >= maxRatioVal {
					histRatio = maxRatioVal
				} else {
					histRatio = *rTest.Ratio
				}
			}
			metrics.RatiosSeen.Observe(histRatio)

			running := false
			if b, ok := getBridgeBase(r); ok {
				running = b.Flags.Running
			}
			metrics.Resources.With(
				prometheus.Labels{
					"type":       rName,
					"functional": core.StateToString(rTest.State),
					"ratio":      core.SpeedToString(rTest.Speed),
					"running":    strconv.FormatBool(running),
				}).Inc()
		}

		functionalCount += float64(stateCount[core.StateFunctional])
		acceptedCount += float64(ratioCount[core.SpeedAccepted])
		numResources += float64(hashring.Len())
		checkFlickered(metrics, currentRatios, newRatios)

	}

	// Distribute only functional resources if the fraction is high enough.
	// The fraction might be low after a restart as many resources will be
	// untested or if there is an issue with bridgestrap.
	functionalFraction := functionalCount / numResources
	rcol.OnlyFunctional = functionalFraction >= MinFunctionalFraction
	if rcol.OnlyFunctional {
		metrics.DistributingNonFunctional.Set(0)
	} else {
		metrics.DistributingNonFunctional.Set(1)
	}

	// Distribute only resources with ratio above the threshold if the
	// fraction is high enough
	acceptedFraction := acceptedCount / numResources
	rcol.UseBandwidthRatio = acceptedFraction >= MinRatioFraction
	if rcol.UseBandwidthRatio {
		metrics.IgnoringBandwidthRatio.Set(0)
	} else {
		metrics.IgnoringBandwidthRatio.Set(1)
	}

	return newRatios
}

func checkFlickered(metrics *Metrics, currentRatios map[core.Hashkey]flicker, newRatios map[core.Hashkey]flicker) {
	// Check for resources that have changed between accepted/rejected
	if currentRatios != nil {
		if reflect.DeepEqual(currentRatios, newRatios) {
			metrics.FlickeringBandwidth.With(prometheus.Labels{"flickered": "None"}).Inc()
		} else {
			for fingerprint, newFlicker := range newRatios {
				currentFlicker, found := currentRatios[fingerprint]
				if found {
					if currentFlicker.speed != newFlicker.speed {
						newRatios[fingerprint] = flicker{
							speed:     newFlicker.speed,
							flickered: true,
						}
						if currentFlicker.flickered && newFlicker.speed == core.SpeedAccepted {
							metrics.FlickeringBandwidth.With(prometheus.Labels{"flickered": "ON"}).Inc()
						} else if currentFlicker.flickered && newFlicker.speed != core.SpeedAccepted {
							metrics.FlickeringBandwidth.With(prometheus.Labels{"flickered": "OFF"}).Inc()
						}
					}
				} else {
					metrics.FlickeringBandwidth.With(prometheus.Labels{"flickered": "NEW"}).Inc()
				}
			}
			for fingerprint := range currentRatios {
				_, found := newRatios[fingerprint]
				if !found {
					metrics.FlickeringBandwidth.With(prometheus.Labels{"flickered": "GONE"}).Inc()
				}
			}
		}
	}
}

func pruneExpiredResources(rcol *core.BackendResources) {

	for rName, hashring := range rcol.Collection {
		origLen := hashring.Len()
		prunedResources := rcol.Prune(rName)
		if len(prunedResources) > 0 {
			log.Printf("Pruned %d out of %d resources from %s hashring.", len(prunedResources), origLen, rName)
		}
	}
}

// reloadBridgeDescriptors reloads bridge descriptors from the given
// cached-extrainfo file and its corresponding cached-extrainfo.new.
func reloadBridgeDescriptors(cfg *Config, rcol *core.BackendResources, testFunc resources.TestFunc) {

	//First load bridge descriptors from network status file
	bridges, err := loadBridgesFromNetworkstatus(cfg.Backend.NetworkstatusFile)
	if err != nil {
		log.Printf("Error loading network statuses: %s", err.Error())
	}

	distributorNames := make([]string, 0, len(cfg.Backend.DistProportions)+1)
	distributorNames = append(distributorNames, "none")
	for dist := range cfg.Backend.DistProportions {
		distributorNames = append(distributorNames, dist)
	}

	err = getBridgeDistributionRequest(cfg.Backend.DescriptorsFile, distributorNames, bridges)
	if err != nil {
		log.Printf("Error loading bridge descriptors file: %s", err.Error())
	}

	//Update bridges from extrainfo files
	for _, filename := range []string{cfg.Backend.ExtrainfoFile, cfg.Backend.ExtrainfoFile + ".new"} {
		descriptors, err := loadBridgesFromExtrainfo(filename)
		if err != nil {
			log.Printf("Failed to reload bridge descriptors: %s", err)
			continue
		}

		for fingerprint, desc := range descriptors {
			bridge, ok := bridges[fingerprint]
			if !ok {
				log.Printf("Received extrainfo descriptor for bridge %s but could not find bridge with that fingerprint", fingerprint)
				continue
			}
			bridge.Transports = desc.Transports
		}
	}

	bl, err := newBlockList(cfg.Backend.BlocklistFile, cfg.Backend.AllowlistFile)
	if err != nil {
		log.Println("Problem loading block list:", err)
	}

	log.Printf("Adding %d bridges.", len(bridges))
	for _, bridge := range bridges {
		blockedIn := bl.blockedIn(bridge.Fingerprint)

		for _, t := range bridge.Transports {
			if !resources.ResourceMap[t.Type()].IsAddressDummy && t.Address.Invalid() {
				log.Printf("Reject bridge %s transport %s as its IP is not valid: %s", t.Fingerprint, t.Type(), t.Address.String())
				t.SetTestFunc(setTestResourceInvalidAddress)
			} else {
				t.SetTestFunc(testFunc)
			}
			t.Flags = bridge.Flags
			t.Distribution = bridge.Distribution
			t.SetBlockedIn(blockedIn)
			rcol.Add(t)
		}

		// only hand out vanilla flavour if there are no transports
		if len(bridge.Transports) == 0 {
			if bridge.Address.Invalid() {
				log.Printf("Reject vanilla bridge %s s as its IP is not valid: %s", bridge.Fingerprint, bridge.Address.String())
				continue
			}
			bridge.SetBlockedIn(blockedIn)
			bridge.SetTestFunc(testFunc)
			rcol.Add(bridge)
		}
	}
	rcol.Save()
}

// learn about available bridges by parsing a network status file
func loadBridgesFromNetworkstatus(networkstatusFile string) (map[string]*resources.Bridge, error) {
	bridges := make(map[string]*resources.Bridge)
	consensus, err := zoossh.ParseUnsafeConsensusFile(networkstatusFile)
	if err != nil {
		return nil, err
	}

	numBridges := 0
	for obj := range consensus.Iterate(nil) {
		status, ok := consensus.Get(obj.GetFingerprint())
		if !ok {
			log.Printf("Could not retrieve network status for bridge %s",
				string(obj.GetFingerprint()))
			continue
		}
		// create a new bridge for this status
		b := resources.NewBridge()
		b.Fingerprint = string(status.GetFingerprint())

		if addr, err := net.ResolveIPAddr("", status.Address.IPv6Address.String()); err == nil {
			b.Address = resources.IPAddr{IPAddr: *addr}
			b.Port = status.Address.IPv6ORPort
			oraddress := resources.ORAddress{
				IPVersion: 6,
				Port:      b.Port,
				Address:   b.Address,
			}
			b.ORAddresses = append(b.ORAddresses, oraddress)
		}
		if addr, err := net.ResolveIPAddr("", status.Address.IPv4Address.String()); err == nil {
			b.Address = resources.IPAddr{IPAddr: *addr}
			b.Port = status.Address.IPv4ORPort
			oraddress := resources.ORAddress{
				IPVersion: 4,
				Port:      b.Port,
				Address:   b.Address,
			}
			b.ORAddresses = append(b.ORAddresses, oraddress)
		}

		b.Flags.Fast = status.Flags.Fast
		b.Flags.Stable = status.Flags.Stable
		b.Flags.Running = status.Flags.Running
		b.Flags.Valid = status.Flags.Valid

		bridges[b.Fingerprint] = b
		numBridges++
	}
	return bridges, nil
}

// getBridgeDistributionRequest from the bridge-descriptors file
func getBridgeDistributionRequest(descriptorsFile string, distributorNames []string, bridges map[string]*resources.Bridge) error {
	descriptors, err := zoossh.ParseUnsafeDescriptorFile(descriptorsFile)
	if err != nil {
		return err
	}

	for fingerprint, bridge := range bridges {
		descriptor, ok := descriptors.Get(zoossh.Fingerprint(fingerprint))
		if !ok {
			log.Printf("Bridge %s from networkstatus not pressent in the descriptors file %s", fingerprint, descriptorsFile)
			continue
		}

		if descriptor.BridgeDistributionRequest != "any" {
			for _, dist := range distributorNames {
				if dist == descriptor.BridgeDistributionRequest {
					bridge.Distribution = dist
					break
				}
			}
			if bridge.Distribution == "" {
				log.Printf("Bridge %s has an unsupported distribution request: %s. Setting it to none.", fingerprint, descriptor.BridgeDistributionRequest)
				bridge.Distribution = "none"
			}
		}
	}
	return nil
}

// loadBridgesFromExtrainfo loads and returns bridges from Serge's extrainfo
// files.
func loadBridgesFromExtrainfo(extrainfoFile string) (map[string]*resources.Bridge, error) {

	file, err := os.Open(extrainfoFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	extra, err := parseExtrainfoDoc(file)
	if err != nil {
		return nil, err
	}

	return extra, nil
}

// parseExtrainfoDoc parses the given extra-info document and returns the
// content as a Bridges object.  Note that the extra-info document format is as
// it's produced by the bridge authority.
func parseExtrainfoDoc(r io.Reader) (map[string]*resources.Bridge, error) {

	bridges := make(map[string]*resources.Bridge)

	scanner := bufio.NewScanner(r)
	b := resources.NewBridge()
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// We're dealing with a new extra-info block, i.e., a new bridge.
		if strings.HasPrefix(line, ExtraInfoPrefix) {
			words := strings.Split(line, " ")
			if len(words) != 3 {
				return nil, errors.New("incorrect number of words in 'extra-info' line")
			}
			b.Fingerprint = words[2]
		}

		// We're dealing with a bridge's transport protocols.  There may be
		// several.
		if strings.HasPrefix(line, TransportPrefix) {
			t := resources.NewTransport()
			t.Fingerprint = b.Fingerprint
			err := populateTransportInfo(line, t)
			if err != nil {
				return nil, err
			}
			b.AddTransport(t)
		}

		// Let's store the bridge when the record ends
		if strings.HasPrefix(line, RecordEndPrefix) {
			bridges[b.Fingerprint] = b
			b = resources.NewBridge()
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return bridges, nil
}

// populateTransportInfo parses the given transport line of the format:
//
//	"transport" transportname address:port [arglist] NL
//
// ...and writes it to the given transport object.  See the specification for
// more details on what transport lines look like:
// <https://gitweb.torproject.org/torspec.git/tree/dir-spec.txt?id=2b31c63891a63cc2cad0f0710a45989071b84114#n1234>
func populateTransportInfo(transport string, t *resources.Transport) error {

	if !strings.HasPrefix(transport, TransportPrefix) {
		return errors.New("no 'transport' prefix")
	}

	words := strings.Split(transport, " ")
	if len(words) < MinTransportWords {
		return errors.New("not enough arguments in 'transport' line")
	}
	t.SetType(words[1])

	host, port, err := net.SplitHostPort(words[2])
	if err != nil {
		return err
	}
	addr, err := net.ResolveIPAddr("", host)
	if err != nil {
		return err
	}
	t.Address = resources.IPAddr{IPAddr: net.IPAddr{IP: addr.IP, Zone: addr.Zone}}
	p, err := strconv.Atoi(port)
	if err != nil {
		return err
	}
	t.Port = uint16(p)

	// We may be dealing with one or more key=value pairs.
	if len(words) > MinTransportWords {
		args := strings.Split(words[3], ",")
		for _, arg := range args {
			kv := strings.Split(arg, "=")
			if len(kv) != 2 {
				return fmt.Errorf("key:value pair in %q not separated by a '='", words[3])
			}
			t.Parameters[kv[0]] = kv[1]
		}
	}

	return nil
}

func setTestResourceInvalidAddress(r core.Resource) {
	rTest := r.TestResult()
	rTest.State = core.StateDysfunctional
	rTest.Speed = core.SpeedUntested
	rTest.LastTested = time.Now()
	rTest.Error = "Bridge address is not valid"
}

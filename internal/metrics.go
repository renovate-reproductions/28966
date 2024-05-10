// Copyright (c) 2021-2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/core"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/resources"
)

const (
	PrometheusNamespace = "rdsys_backend"
)

type Metrics struct {
	DistributingNonFunctional prometheus.Gauge
	IgnoringBandwidthRatio    prometheus.Gauge
	FlickeringBandwidth       *prometheus.CounterVec
	RatiosSeen                prometheus.Histogram
	Resources                 *prometheus.GaugeVec
	DistributorResources      *prometheus.GaugeVec
	Requests                  *prometheus.CounterVec
}

// InitMetrics initialises our Prometheus metrics.
func InitMetrics() *Metrics {

	metrics := &Metrics{}

	metrics.DistributingNonFunctional = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: PrometheusNamespace,
			Name:      "distributing_non_functional_resources",
			Help:      "If rdsys is distributing non functional bridges",
		},
	)

	metrics.IgnoringBandwidthRatio = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: PrometheusNamespace,
			Name:      "ignoring_resource_bandwidth_ratio",
			Help:      "If rdsys is ignoring the resource bandwidth ratio",
		},
	)

	metrics.FlickeringBandwidth = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: PrometheusNamespace,
			Name:      "flickering_bandwidth",
			Help:      "The number of resources that have changed from acceptable to rejected bandwidths",
		},
		[]string{"flickered"},
	)

	metrics.RatiosSeen = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: PrometheusNamespace,
			Name:      "ratio_seen",
			Buckets:   prometheus.LinearBuckets(0.0, 0.1, 30),
			Help:      "The different bandwidth ratios that were observed",
		},
	)

	metrics.Resources = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: PrometheusNamespace,
			Name:      "resources",
			Help:      "The number of resources we have by their type, functionality, ratio and running state",
		},
		[]string{"type", "functional", "ratio", "running"},
	)

	metrics.DistributorResources = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: PrometheusNamespace,
			Name:      "distributor_resources",
			Help:      "The number of resources we have per distributor",
		},
		[]string{"distributor", "type"},
	)

	metrics.Requests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: PrometheusNamespace,
			Name:      "requests_total",
			Help:      "The number of API requests",
		},
		[]string{"target"},
	)

	return metrics
}

func (m *Metrics) updateDistributors(cfg *Config, rcol *core.BackendResources) {
	file, err := os.OpenFile(cfg.Backend.AssignmentsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println("Can't open assignments file", cfg.Backend.AssignmentsFile, err)
		return
	}
	defer file.Close()

	fmt.Fprintln(file, "bridge-pool-assignment", time.Now().UTC().Format("2006-01-02 15:04:05"))
	distributors := []string{}
	for distributor := range cfg.Backend.DistProportions {
		distributors = append(distributors, distributor)
		for transport := range cfg.Backend.Resources {
			rs := rcol.Get(distributor, transport)
			for _, resource := range rs.Working {
				appendAssingment(file, resource, distributor, true)
			}
			for _, resource := range rs.Notworking {
				appendAssingment(file, resource, distributor, false)
			}

			m.DistributorResources.
				With(prometheus.Labels{"distributor": distributor, "type": transport}).
				Set(float64(len(rs.Working)))
		}
	}

	filterNone := func(r core.Resource) bool {
		distributor := r.Distributor()
		if distributor == "" {
			return false
		}

		for _, distName := range distributors {
			if distName == distributor {
				return false
			}
		}
		return true
	}
	for transport := range cfg.Backend.Resources {
		rs := rcol.Collection[transport].Filter(filterNone)
		for _, resource := range rs {
			appendAssingment(file, resource, "none", false)
		}

		m.DistributorResources.
			With(prometheus.Labels{"distributor": "none", "type": transport}).
			Set(float64(len(rs)))
	}
}

func appendAssingment(file *os.File, resource core.Resource, distributor string, distributed bool) {
	bridgeBase, ok := getBridgeBase(resource)
	if ok {
		info := bridgeInfo(bridgeBase)
		testResult := bridgeTestResult(resource)
		fmt.Fprintln(file, bridgeBase.Fingerprint, distributor, "transport="+resource.Type(), info, "distributed="+strconv.FormatBool(distributed), testResult)
	}
}

func getBridgeBase(resource core.Resource) (bridgeBase *resources.BridgeBase, ok bool) {
	transport, ok := resource.(*resources.Transport)
	if ok {
		return &transport.BridgeBase, ok
	}

	bridge, ok := resource.(*resources.Bridge)
	if ok {
		return &bridge.BridgeBase, ok
	}
	return nil, false
}

func bridgeInfo(bridge *resources.BridgeBase) string {
	ip := map[uint16]struct{}{}

	if bridge.Address.IP.To4() != nil {
		ip[4] = struct{}{}
	} else {
		ip[6] = struct{}{}
	}

	for _, address := range bridge.ORAddresses {
		ip[address.IPVersion] = struct{}{}
	}

	versions := make([]string, 0, len(ip))
	for version := range ip {
		versions = append(versions, strconv.Itoa(int(version)))
	}

	info := []string{"ip=" + strings.Join(versions, ",")}
	if bridge.Port == 443 {
		info = append(info, "port=443")
	}

	blockedIn := bridge.BlockedIn()
	if len(blockedIn) != 0 {
		countries := make([]string, 0, len(blockedIn))
		for k := range blockedIn {
			countries = append(countries, k)
		}

		info = append(info, "blocklist="+strings.Join(countries, ","))
	}

	return strings.Join(info, " ")
}

func bridgeTestResult(resource core.Resource) string {
	testResult := resource.TestResult()
	info := "state=" + core.StateToString(testResult.State)
	info += " bandwidth=" + core.SpeedToString(testResult.Speed)
	if testResult.Ratio != nil {
		info += fmt.Sprintf(" ratio=%.3f", *testResult.Ratio)
	}
	return info
}

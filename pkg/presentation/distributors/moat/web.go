// Copyright (c) 2021-2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package moat

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gitlab.torproject.org/tpo/anti-censorship/geoip"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/presentation/distributors/common"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/distributors/moat"
)

type moatHandler struct {
	dist    *moat.MoatDistributor
	geoipdb *geoip.Geoip
	cfg     *internal.MoatDistConfig
}

type jsonError struct {
	Errors []jsonErrorEntry `json:"errors"`
}

type jsonErrorEntry struct {
	Code   int    `json:"code"`
	Detail string `json:"detail"`
}

var (
	invalidRequest = jsonError{[]jsonErrorEntry{{
		Code:   400,
		Detail: "Not valid request",
	}}}
	countryNotFound = jsonError{[]jsonErrorEntry{{
		Code:   406,
		Detail: "Could not find country code for circumvention settings",
	}}}
	transportNotFound = jsonError{[]jsonErrorEntry{{
		Code:   404,
		Detail: "No provided transport is available for this country",
	}}}
)

// InitFrontend is the entry point to HTTPS's Web frontend.  It spins up the
// Web server and then waits until it receives a SIGINT.
func InitFrontend(cfg *internal.Config) {
	var mh moatHandler

	mh.cfg = &cfg.Distributors.Moat
	mh.dist = &moat.MoatDistributor{
		FetchBridges: fetchBridges,
	}
	err := loadFile(mh.cfg.CircumventionMap, mh.dist.LoadCircumventionMap)
	if err != nil {
		log.Fatalf("Can't load circumvention map %s: %v", mh.cfg.CircumventionMap, err)
	}
	err = loadFile(mh.cfg.CircumventionDefaults, mh.dist.LoadCircumventionDefaults)
	if err != nil {
		log.Fatalf("Can't load circumvention defaults %s: %v", mh.cfg.CircumventionDefaults, err)
	}
	if mh.cfg.DummyBridgesFile != "" {
		err = loadFile(mh.cfg.DummyBridgesFile, mh.dist.LoadDummyBridges)
		if err != nil {
			log.Fatalf("Can't load dummy bridges %s: %v", mh.cfg.DummyBridgesFile, err)
		}
	}

	mh.geoipdb, err = geoip.New(mh.cfg.GeoipDB, mh.cfg.Geoip6DB)
	if err != nil {
		log.Fatal("Can't load geoip databases", mh.cfg.GeoipDB, mh.cfg.Geoip6DB, ":", err)
	}

	handlers := map[string]http.HandlerFunc{
		"/moat/circumvention/map":            http.HandlerFunc(mh.circumventionMapHandler),
		"/moat/circumvention/countries":      http.HandlerFunc(mh.countriesHandler),
		"/moat/circumvention/settings":       http.HandlerFunc(mh.circumventionSettingsHandler),
		"/moat/circumvention/builtin":        http.HandlerFunc(mh.builtinHandler),
		"/moat/circumvention/defaults":       http.HandlerFunc(mh.circumventionDefaultsHandler),
		"/meek/moat/circumvention/map":       http.HandlerFunc(mh.circumventionMapHandler),
		"/meek/moat/circumvention/countries": http.HandlerFunc(mh.countriesHandler),
		"/meek/moat/circumvention/settings":  http.HandlerFunc(mh.circumventionSettingsHandler),
		"/meek/moat/circumvention/builtin":   http.HandlerFunc(mh.builtinHandler),
		"/meek/moat/circumvention/defaults":  http.HandlerFunc(mh.circumventionDefaultsHandler),

		"/moat/fetch":      http.HandlerFunc(mh.captchaFetchHandler),
		"/moat/check":      http.HandlerFunc(mh.captchaCheckHandler),
		"/meek/moat/fetch": http.HandlerFunc(mh.captchaFetchHandler),
		"/meek/moat/check": http.HandlerFunc(mh.captchaCheckHandler),

		"/metrics": promhttp.Handler().ServeHTTP,
	}

	common.StartWebServer(
		&cfg.Distributors.Moat.WebApi,
		cfg,
		mh.dist,
		handlers,
	)
}

func loadFile(path string, loadFn func(r io.Reader) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return loadFn(f)
}

func (mh moatHandler) circumventionMapHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	m := mh.dist.GetCircumventionMap()
	enc := json.NewEncoder(w)
	err := enc.Encode(m)
	if err != nil {
		log.Println("Error encoding circumvention map:", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
func (mh moatHandler) countriesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	m := mh.dist.GetCircumventionMap()
	countries := make([]string, 0, len(m))
	for k := range m {
		countries = append(countries, k)
	}

	enc := json.NewEncoder(w)
	err := enc.Encode(countries)
	if err != nil {
		log.Println("Error encoding countries list:", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

type circumventionSettingsRequest struct {
	Country    string   `json:"country"`
	Transports []string `json:"transports"`
}

func (mh moatHandler) circumventionSettingsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)

	var request circumventionSettingsRequest
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&request)
	if err != nil && !errors.Is(err, io.EOF) {
		log.Println("Error decoding circumvention settings request:", err)
		err = enc.Encode(invalidRequest)
		if err != nil {
			log.Println("Error encoding jsonError:", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	ip := common.IpFromRequest(r, mh.cfg.TrustProxy)
	if request.Country == "" {
		request.Country = mh.countryFromIP(ip)
		if request.Country == "" {
			log.Println("Could not find country code for cicrumvention settings")
			err = enc.Encode(countryNotFound)
			if err != nil {
				log.Println("Error encoding jsonError:", err)
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}
	}

	shimToken := r.Header.Get("shim-token")
	s, err := mh.dist.GetCircumventionSettings(request.Country, request.Transports, ip, shimToken)
	if err != nil {
		if errors.Is(err, moat.NoTransportError) {
			err = enc.Encode(transportNotFound)
			if err != nil {
				log.Println("Error encoding jsonError:", err)
				w.WriteHeader(http.StatusInternalServerError)
			}
		} else {
			log.Println("Error getting circumvention settings:", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	err = enc.Encode(s)
	if err != nil {
		log.Println("Error encoding circumvention settings:", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (mh moatHandler) circumventionDefaultsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)

	var request transportsRequest
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&request)
	if err != nil && !errors.Is(err, io.EOF) {
		log.Println("Error decoding circumvention defaults request:", err)
		err = enc.Encode(invalidRequest)
		if err != nil {
			log.Println("Error encoding jsonError:", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	ip := common.IpFromRequest(r, mh.cfg.TrustProxy)
	shimToken := r.Header.Get("shim-token")
	s, err := mh.dist.GetCircumventionDefaults(request.Transports, ip, shimToken)
	if err != nil {
		if errors.Is(err, moat.NoTransportError) {
			err = enc.Encode(transportNotFound)
			if err != nil {
				log.Println("Error encoding jsonError:", err)
				w.WriteHeader(http.StatusInternalServerError)
			}
		} else {
			log.Println("Error getting circumvention defaults:", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	if s == nil {
		w.Write([]byte("{}"))
		return
	}

	err = enc.Encode(s)
	if err != nil {
		log.Println("Error encoding circumvention defaults:", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (mh moatHandler) countryFromIP(ip net.IP) string {
	country, ok := mh.geoipdb.GetCountryByAddr(ip)
	if !ok {
		return ""
	}
	return strings.ToLower(country)
}

type transportsRequest struct {
	Transports []string `json:"transports"`
}

func (mh moatHandler) builtinHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)

	var request transportsRequest
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&request)
	if err != nil && !errors.Is(err, io.EOF) {
		log.Println("Error decoding builtin request:", err)
		err = enc.Encode(invalidRequest)
		if err != nil {
			log.Println("Error encoding jsonError:", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	bb := mh.dist.GetBuiltInBridges(request.Transports)
	err = enc.Encode(bb)
	if err != nil {
		log.Println("Error encoding builtin bridges:", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

type builtinBridgesJSON struct {
	Bridges map[string][]string `json:"bridges"`
}

func fetchBridges(url string) (map[string][]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Status code: %d", resp.StatusCode)
	}

	var builtinBridges builtinBridgesJSON
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&builtinBridges)
	return builtinBridges.Bridges, err
}

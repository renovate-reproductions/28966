// Copyright (c) 2021-2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package telegram

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/core"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/delivery"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/delivery/mechanisms"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/persistence"
)

const (
	DistName = "telegram"
)

var (
	bridgeRequestsCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "telegram_bridges_request_total",
		Help: "The total number of bridge requests",
	},
		[]string{"pool", "status"},
	)

	newBridgesGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "telegram_new_bridges",
		Help: "The total number of new bridges in the telegram distributor",
	},
		[]string{"updater"},
	)
)

type metricsData struct {
	hashKey core.Hashkey
	pool    string
	err     error
}

type TelegramDistributor struct {
	oldHashring    *core.Hashring
	newHashring    *core.Hashring
	cfg            *internal.TelegramDistConfig
	ipc            delivery.Mechanism
	wg             sync.WaitGroup
	shutdown       chan bool
	metricsChan    chan<- metricsData
	dynamicBridges map[string][]core.Resource
	seenIDs        map[int64]time.Time

	// newHashrightLock is used to block read access when an update is happening in the newHashring
	newHashrightLock sync.RWMutex

	// NewBridgesStore maps each updater to it's persistence mechanism
	NewBridgesStore map[string]persistence.Mechanism
	// IdStore creates a persistence mechanism for seen IDs
	IdStore persistence.Mechanism
}

func (d *TelegramDistributor) GetResources(id int64) []core.Resource {
	now := time.Now().Unix() / (60 * 60)
	period := now / int64(d.cfg.RotationPeriodHours)
	hashKey := core.NewHashkey(fmt.Sprintf("%d-%d", id, period))

	md := metricsData{hashKey: hashKey}

	d.newHashrightLock.RLock()
	resources, err := d.newHashring.GetMany(hashKey, d.cfg.NumBridgesPerRequest)
	d.newHashrightLock.RUnlock()
	if err != nil {
		log.Println("Error getting resources from the hashring:", err)
		md.err = err
	}

	md.pool = "new"
	if id < d.cfg.MinUserID {
		md.pool = "old"
		oldResources, err := d.oldHashring.GetMany(hashKey, d.cfg.NumBridgesPerRequest)
		if err != nil {
			log.Println("Error getting resources from the old hashring:", err)
			md.err = err
		}
		resources = append(oldResources, resources...)
	}

	d.metricsChan <- md
	return resources
}

type IdFreshnessError struct {
}

func (e *IdFreshnessError) Error() string {
	return fmt.Sprintf("Error: ID is too fresh, invitation can not be issued.")
}

type InvitationLimitError struct {
	ClaimTime time.Time
}

func (e *InvitationLimitError) Error() string {
	return fmt.Sprintf("%s ", e.ClaimTime.String())
}

type LoxRequestError struct {
	Err string
}

func (e *LoxRequestError) Error() string {
	return fmt.Sprintf(e.Err)
}
func (d *TelegramDistributor) GetInvitation(id int64) ([]byte, error) {

	if id > d.cfg.MinUserID {
		return nil, &IdFreshnessError{}
	}
	if added, ok := d.seenIDs[id]; ok {
		if (added.AddDate(0, 0, InvitationRequestDayLimit)).After(time.Now()) {
			claim_time := added.AddDate(0, 0, InvitationRequestDayLimit)
			return nil, &InvitationLimitError{ClaimTime: claim_time}
		}
	}
	response, err := GetLoxInvitation(d.cfg.LoxServerAddress)
	if err == nil {
		d.seenIDs[id] = time.Now()
		d.IdStore.Save(d.seenIDs)
		return response, nil
	}
	return nil, &LoxRequestError{Err: err.Error()}
}

func GetLoxInvitation(loxserver string) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest("POST", loxserver+"/invite", nil)
	if err != nil {
		log.Println("error making http request: ", err)
		return nil, err
	}

	req.Header = http.Header{
		"Content-Type": {"application/json"},
	}

	res, err := client.Do(req)
	if err != nil {
		log.Println("error getting http response: ", err)
		return nil, err
	}

	if res.StatusCode != 200 {
		log.Println("client: bad request:", err)
		return nil, err
	}
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Println("client: could not read response body:", err)
		return nil, err
	}
	return resBody, nil
}

// housekeeping listens to updates from the backend resources
func (d *TelegramDistributor) housekeeping(rStream chan *core.ResourceDiff) {
	defer d.wg.Done()
	defer close(rStream)
	defer d.ipc.StopStream()

	for {
		select {
		case diff := <-rStream:
			d.oldHashring.ApplyDiff(diff)
		case <-d.shutdown:
			log.Printf("Shutting down housekeeping.")
			return
		}
	}
}

func (d *TelegramDistributor) Init(cfg *internal.Config) {
	d.cfg = &cfg.Distributors.Telegram
	d.shutdown = make(chan bool)
	d.oldHashring = core.NewHashring()
	d.newHashring = core.NewHashring()
	d.seenIDs = make(map[int64]time.Time)
	d.loadNewBridgesFromStore()
	d.loadIdsFromStore()
	d.dynamicBridges = make(map[string][]core.Resource)

	metricsChan := make(chan metricsData)
	d.metricsChan = metricsChan
	go metricsUpdater(metricsChan, cfg.Distributors.Telegram.RotationPeriodHours)

	log.Printf("Initialising resource stream.")
	d.ipc = mechanisms.NewHttpsIpc(
		cfg.Backend.ResourceStreamURL(),
		"GET",
		cfg.Backend.ApiTokens[DistName])
	rStream := make(chan *core.ResourceDiff)
	req := core.ResourceRequest{
		RequestOrigin: DistName,
		ResourceTypes: []string{d.cfg.Resource},
		Receiver:      rStream,
	}
	d.ipc.StartStream(&req)

	d.wg.Add(1)
	go d.housekeeping(rStream)
}

func (d *TelegramDistributor) Shutdown() {
	log.Printf("Shutting down %s distributor.", DistName)

	close(d.metricsChan)
	close(d.shutdown)
	d.wg.Wait()
}

func metricsUpdater(ch <-chan metricsData, rotationPeriodHours int) {
	requestHashKeys := make(map[core.Hashkey]time.Time)
	lastCleanup := time.Now()

	for md := range ch {
		status := "fresh"
		keepDate := time.Now().Add(-time.Hour * time.Duration(rotationPeriodHours))
		if date, ok := requestHashKeys[md.hashKey]; ok && date.After(keepDate) {
			status = "cached"
		} else {
			requestHashKeys[md.hashKey] = time.Now()
		}
		if md.err != nil {
			status = "error"
		}
		bridgeRequestsCount.WithLabelValues(md.pool, status).Inc()

		if lastCleanup.Before(keepDate) {
			for hk, t := range requestHashKeys {
				if t.Before(keepDate) {
					delete(requestHashKeys, hk)
				}
			}
		}
	}
}

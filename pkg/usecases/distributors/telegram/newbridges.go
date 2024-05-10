// Copyright (c) 2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/core"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/resources"
)

const InvitationRequestDayLimit int = 7

type bridgesJSON struct {
	Bridgelines []string `json:"bridgelines"`
}

func (d *TelegramDistributor) loadNewBridgesFromStore() {
	d.newHashrightLock.Lock()
	defer d.newHashrightLock.Unlock()

	for updater, store := range d.NewBridgesStore {
		var rs []resources.Transport
		err := store.Load(&rs)
		if err != nil {
			log.Println("Error loading updater", updater, ":", err)
			continue
		}
		for _, r := range rs {
			d.newHashring.Add(&r)
		}
	}
}

func (d *TelegramDistributor) loadIdsFromStore() {
	var seenIDs map[int64]time.Time
	err := d.IdStore.Load(&seenIDs)
	if err != nil {
		log.Println("Error loading IdStore :", err)
	}
	for id, seen := range seenIDs {
		if seen.AddDate(0, 0, InvitationRequestDayLimit).Before(time.Now()) {
			continue
		}
		d.seenIDs[id] = seen
	}
}

// LoadNewBridges loads bridges in bridgesJSON format from the reader into the new bridges newHashring
//
// This function locks a mutex when accessing the newHashring, we should be careful to don't make
// a deadlock with the internal mutex in the hashring. Never call this function while holding the
// newHashring mutex.
func (d *TelegramDistributor) LoadNewBridges(name string, r io.Reader) error {
	var updatedBridges bridgesJSON
	dec := json.NewDecoder(r)
	err := dec.Decode(&updatedBridges)
	if err != nil {
		return err
	}

	resourceList := make([]core.Resource, len(updatedBridges.Bridgelines))
	for i, bridgeline := range updatedBridges.Bridgelines {
		resource, err := resources.FromBridgeline(bridgeline)
		if err != nil {
			return err
		}
		if resource.Type() != d.cfg.Resource {
			return fmt.Errorf("Not valid bridge type %s", resource.Type())
		}

		resourceList[i] = resource
	}

	d.newHashrightLock.Lock()
	for _, resource := range d.dynamicBridges[name] {
		d.newHashring.Remove(resource)
	}
	d.dynamicBridges[name] = resourceList

	for _, resource := range resourceList {
		d.newHashring.Add(resource)
	}
	d.newHashrightLock.Unlock()

	numBridges := len(resourceList)
	log.Println("Got", numBridges, "new bridges from", name)
	newBridgesGauge.WithLabelValues(name).Set(float64(numBridges))

	persistence := d.NewBridgesStore[name]
	if persistence != nil {
		return d.NewBridgesStore[name].Save(resourceList)
	}

	return nil
}

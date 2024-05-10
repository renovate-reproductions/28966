// Copyright (c) 2021-2024, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package core

import (
	"log"
	"sync"
)

const (
	// These constants represent resource event types.  The backend informs
	// distributors if a resource is new, has changed, or has disappeared.
	ResourceUnchanged = iota
	ResourceIsNew
	ResourceChanged
	ResourceIsGone
)

// BackendResources implements a collection of resources for our backend.  The
// backend uses this data structure to keep track of all of its resource types.
type BackendResources struct {
	Collection

	// OnlyFunctional resources will be provided to distributors
	OnlyFunctional bool

	// UseBandwidthRatio to decide wich bridges to distribute
	UseBandwidthRatio bool

	// The mutex us used to protect the access to EventRecipients.
	// The hashrings in the Collection have their own mutex and the entries
	// of the Collection map are only set during intialization.
	sync.RWMutex
	// EventRecipients maps a distributor name (e.g., "salmon") to an event
	// recipient struct that helps us keep track of notifying distributors when
	// their resources change.
	EventRecipients map[string]*EventRecipient
}

// EventRecipient represents the recipient of a resource event, i.e. a
// distributor; or rather, what we need to send updates to said distributor.
type EventRecipient struct {
	EventChans []chan *ResourceDiff
	Request    *ResourceRequest
}

// NewBackendResources creates and returns a new resource collection.
// rTypes is a map of resource type names to a boolean indicating if the resource is unpartitioend
func NewBackendResources(cfg *CollectionConfig) *BackendResources {
	r := &BackendResources{}
	r.Collection = NewCollection(cfg)
	for _, rc := range cfg.Types {
		if !rc.Unpartitioned {
			r.Collection[rc.Type] = newPartitionedWithDistributors(r.Collection[rc.Type])
		}
	}
	r.EventRecipients = make(map[string]*EventRecipient)
	return r
}

// Add adds the given resource to the resource collection.  If the resource
// already exists but has changed (i.e. its unique ID remains the same but its
// object ID changed), we update the existing resource.
func (ctx *BackendResources) Add(r1 Resource) {
	hashring, exists := ctx.Collection[r1.Type()]
	if !exists {
		return
	}

	event := hashring.AddOrUpdate(r1)
	if event != ResourceUnchanged {
		ctx.propagateUpdate(r1, event)
	}
}

// Prune removes expired resources.
func (ctx *BackendResources) Prune(rName string) []Resource {

	hashring := ctx.Collection[rName]
	prunedResources := hashring.Prune()
	for _, resource := range prunedResources {
		ctx.propagateUpdate(resource, ResourceIsGone)
	}
	return prunedResources
}

// propagateUpdate sends updates about new, changed, and gone resources to
// channels, allowing the backend to immediately inform a distributor of the
// update.
func (ctx *BackendResources) propagateUpdate(r Resource, event int) {
	ctx.RLock()
	defer ctx.RUnlock()

	if _, exists := ctx.Collection[r.Type()]; !exists {
		return
	}

	// Prepare the hashring difference that we're about to send.
	diff := &ResourceDiff{}
	rm := ResourceMap{r.Type(): []Resource{r}}
	switch event {
	case ResourceIsNew:
		diff.New = rm
	case ResourceChanged:
		diff.Changed = rm
	case ResourceIsGone:
		diff.Gone = rm
	default:
		return
	}

	distName := ctx.Collection[r.Type()].getPartitionName(r)
	eventRecipient, ok := ctx.EventRecipients[distName]
	if !ok {
		// no recipients for that resource
		return
	}
	if !ctx.EventRecipients[distName].Request.HasResourceType(r.Type()) {
		return
	}

	for _, c := range eventRecipient.EventChans {
		c <- diff
	}
}

// RegisterChan registers a channel to be informed about resource updates.
func (ctx *BackendResources) RegisterChan(req *ResourceRequest, recipient chan *ResourceDiff) {
	ctx.Lock()
	defer ctx.Unlock()

	distName := req.RequestOrigin
	log.Printf("Registered new channel for distributor %q to receive updates.", distName)
	_, exists := ctx.EventRecipients[distName]
	if !exists {
		er := &EventRecipient{Request: req, EventChans: []chan *ResourceDiff{recipient}}
		ctx.EventRecipients[distName] = er
	} else {
		ctx.EventRecipients[distName].EventChans = append(ctx.EventRecipients[distName].EventChans, recipient)
	}
}

// UnregisterChan unregisters a channel to be informed about resource updates.
func (ctx *BackendResources) UnregisterChan(distName string, recipient chan *ResourceDiff) {
	ctx.Lock()
	defer ctx.Unlock()

	chanSlice := ctx.EventRecipients[distName].EventChans
	newSlice := []chan *ResourceDiff{}

	for i, c := range chanSlice {
		if c == recipient {
			log.Printf("Unregistering channel from recipients.")
			// Are we dealing with the last element in the slice?
			if i == len(chanSlice)-1 {
				newSlice = chanSlice[:i]
			} else {
				newSlice = append(chanSlice[:i], chanSlice[i+1:]...)
			}
			break
		}
	}
	ctx.EventRecipients[distName].EventChans = newSlice
}

// Get returns a struct that contains the state of resources
// distributor.
func (ctx *BackendResources) Get(distName string, rType string) ResourceState {
	hashring := ctx.GetHashring(distName, rType)
	if hashring == nil {
		log.Printf("Failed to get resources for distributor %q", distName)
		return ResourceState{}
	}

	var resourceState = ResourceState{}
	for _, resource := range hashring.GetAll() {
		rTest := resource.TestResult()
		if (!ctx.OnlyFunctional || rTest.State == StateFunctional) && (!ctx.UseBandwidthRatio || rTest.Speed != SpeedRejected) {
			resourceState.Working = append(resourceState.Working, resource)
		} else {
			resourceState.Notworking = append(resourceState.Notworking, resource)
		}
	}
	return resourceState
}

type partitionedWithDistributors struct {
	*partitionedHashring
}

func newPartitionedWithDistributors(rg ResourceGroup) *partitionedWithDistributors {
	p := rg.(*partitionedHashring)
	p.partitions["none"] = NewHashring()
	return &partitionedWithDistributors{p}
}

func (p partitionedWithDistributors) Add(resource Resource) error {
	name := p.getPartitionName(resource)
	p.addRelationIdentifiers(resource, name)
	hashring := p.partitions[name]
	return hashring.Add(resource)
}

func (p partitionedWithDistributors) AddOrUpdate(resource Resource) int {
	name := p.getPartitionName(resource)
	p.addRelationIdentifiers(resource, name)
	hashring := p.partitions[name]
	return hashring.AddOrUpdate(resource)
}

func (p partitionedWithDistributors) Remove(resource Resource) error {
	hashring := p.partitions[p.getPartitionName(resource)]
	return hashring.Remove(resource)
}

func (p partitionedWithDistributors) getPartitionName(resource Resource) string {
	distName := resource.Distributor()
	if distName != "" {
		if _, ok := p.partitions[distName]; !ok {
			distName = "none"
		}
		return distName
	}
	return p.partitionedHashring.getPartitionName(resource)
}

func (p partitionedWithDistributors) addRelationIdentifiers(resource Resource, partitionName string) {
	if partitionName == "none" {
		return
	}
	p.partitionedHashring.addRelationIdentifiers(resource, partitionName)
}

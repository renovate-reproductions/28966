// Copyright (c) 2021-2024, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package core

import (
	"fmt"
	"log"
	"sort"
	"strings"
)

// Collection maps a resource type (e.g. "obfs4") to its corresponding
// Hashring
type Collection map[string]ResourceGroup

// ResourceGroup is a type that holds a list of resources like a Hashring
type ResourceGroup interface {
	Add(resource Resource) error
	AddOrUpdate(resource Resource) int
	Remove(resource Resource) error
	Len() int
	Clear()
	Filter(FilterFunc) []Resource
	GetAll() []Resource
	Prune() []Resource

	getHashring(partitionName string) *Hashring
	getPartitionName(resource Resource) string
	save() error
}

// CollectionConfig holds the configuration to create a Collection
type CollectionConfig struct {
	// StorageDir is the path to the persistant folder where data will be stored
	StorageDir string

	// Types is the list of Resource types that will be stored in the collection
	Types []TypeConfig
}

// TypeConfig holds the configuration of one Resource type
type TypeConfig struct {
	// Type name of the Resources
	Type string

	// NewResource is a function that retourns a new resource of the Type
	NewResource func() Resource

	// Unpartitioned if the resource hosted in a single Hashring or in a partitioned one
	Unpartitioned bool

	// Proportions is used for partitioned hashrings and indicates the names
	// of each partition and it's proportion of resources that should be asigned to it
	Proportions map[string]int

	// Stored indicates if the resources of this type should be persistant stored in StoreDir
	Stored bool
}

// NewCollection creates and returns a new resource collection
func NewCollection(cfg *CollectionConfig) Collection {
	c := make(Collection)

	for _, rc := range cfg.Types {
		if rc.Unpartitioned {
			h := NewHashring()
			if rc.Stored && cfg.StorageDir != "" {
				h.initStore(rc.Type, cfg.StorageDir, rc.NewResource)
			}
			c[rc.Type] = h
		} else {
			h := newPartitionedHashring(rc.Proportions)
			if cfg.StorageDir != "" {
				h.initStore(rc.Type, cfg.StorageDir, rc.Stored, rc.NewResource)
			}
			c[rc.Type] = h
		}
	}
	return c
}

// Save to the persitant store
func (c Collection) Save() {
	for rType, h := range c {
		err := h.save()
		if err != nil {
			log.Println("Error saving", rType, "to store:", err)
		}
	}
	return
}

// Add resource to the collection
func (c Collection) Add(resource Resource) error {
	rt, ok := c[resource.Type()]
	if !ok {
		return fmt.Errorf("No resource type %s in collection", resource.Type())
	}
	return rt.Add(resource)
}

// String returns a summary of the backend resources.
func (c Collection) String() string {
	keys := []string{}
	for rType := range c {
		keys = append(keys, rType)
	}
	sort.Strings(keys)

	s := []string{}
	for _, key := range keys {
		h := c[key]
		s = append(s, fmt.Sprintf("%d %s", h.Len(), key))
	}
	return strings.Join(s, ", ")
}

// GetHashring returns the hashring of the requested type for the given
// distributor.
func (c Collection) GetHashring(partitionName string, rType string) *Hashring {
	rt, exists := c[rType]
	if !exists {
		log.Printf("Requested resource type %q not present in our resource collection.", rType)
		return NewHashring()
	}
	return rt.getHashring(partitionName)
}

// ApplyDiff updates the collection with the resources changed in ResrouceDiff
func (c Collection) ApplyDiff(diff *ResourceDiff) {
	if diff.FullUpdate {
		for rType := range c {
			c[rType].Clear()
		}
	}

	for rType, resources := range diff.New {
		log.Printf("Adding %d resources of type %s.", len(resources), rType)
		for _, r := range resources {
			c[rType].Add(r)
		}
	}
	for rType, resources := range diff.Changed {
		log.Printf("Changing %d resources of type %s.", len(resources), rType)
		for _, r := range resources {
			c[rType].AddOrUpdate(r)
		}
	}
	for rType, resources := range diff.Gone {
		log.Printf("Removing %d resources of type %s.", len(resources), rType)
		for _, r := range resources {
			c[rType].Remove(r)
		}
	}
}

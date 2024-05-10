// Copyright (c) 2021-2024, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package core

import (
	"testing"
	"time"
)

var (
	partitionName    = "partition"
	proportions      = map[string]int{partitionName: 1}
	collectionConfig = CollectionConfig{
		Types: []TypeConfig{
			{Type: "dummy", Proportions: proportions},
		},
	}
)

func TestAddCollection(t *testing.T) {
	d1 := NewDummy(1, 1)
	d2 := NewDummy(2, 2)
	d3 := NewDummy(3, 2)
	c := NewBackendResources(&collectionConfig)

	c.Add(d1)
	if c.Collection[d1.Type()].Len() != 1 {
		t.Errorf("expected length 1 but got %d", len(c.Collection))
	}
	c.Add(d2)
	if c.Collection[d1.Type()].Len() != 2 {
		t.Errorf("expected length 2 but got %d", len(c.Collection))
	}
	// d3 has the same unique ID as d2 but a different object ID.  Our
	// collection should update d2 but not create a new element.
	c.Add(d3)
	if c.Collection[d1.Type()].Len() != 2 {
		t.Errorf("expected length 2 but got %d", len(c.Collection))
	}

	hashring := c.GetHashring(partitionName, d3.Type())
	elems, err := hashring.GetMany(Hashkey(0), 2)
	if err != nil {
		t.Errorf(err.Error())
	}
	if elems[0] != d1 {
		t.Errorf("got unexpected element")
	}
	if elems[1] != d3 {
		t.Errorf("got unexpected element: %d", elems[1].Oid())
	}
}

func TestStringCollection(t *testing.T) {
	c := NewBackendResources(&collectionConfig)
	s := c.String()
	expected := "0 dummy"
	if s != expected {
		t.Errorf("expected %q but got %q", expected, s)
	}
}

func TestPruneCollection(t *testing.T) {
	d := NewDummy(1, 1)
	d.ExpiryTime = time.Minute * 10
	c := NewBackendResources(&collectionConfig)
	c.Add(d)
	hLength := func() int { return c.Collection[d.Type()].Len() }

	// We should now have one element in the hashring.
	if hLength() != 1 {
		t.Fatalf("expectec hashring of length 1 but got %d", hLength())
	}

	// Expire the hashring node.
	hashring := c.GetHashring(partitionName, d.Type())
	i, err := hashring.getIndex(d.Uid())
	if err != nil {
		t.Errorf("failed to retrieve existing resource: %s", err)
	}
	node := hashring.hashnodes[i]
	node.lastUpdate = time.Now().UTC().Add(-d.ExpiryTime - time.Minute)

	for rName := range c.Collection {
		c.Prune(rName)
	}
	// Pruning should have left our hashring empty.
	if hLength() != 0 {
		t.Fatalf("expectec hashring of length 0 but got %d", hLength())
	}
}

func TestCollectionProportions(t *testing.T) {
	distName := "distributor"
	d := NewDummy(1, 1)

	c1 := NewBackendResources(&collectionConfig)
	c1.Add(d)
	resources := c1.Get(distName, d.Type())
	if len(resources.Working) != 0 {
		t.Errorf("Unexpected resource len %d: %v", len(resources.Working), resources)
	}

	c2 := NewBackendResources(&CollectionConfig{
		Types: []TypeConfig{
			{Type: d.Type(), Proportions: map[string]int{distName: 1}},
		},
	})
	c2.Add(d)
	resources = c2.Get(distName, d.Type())
	if len(resources.Working) != 1 {
		t.Fatalf("Unexpected resource len %d: %v", len(resources.Working), resources)
	}
	if resources.Working[0].Oid() != d.Oid() {
		t.Errorf("Unexpected dummy resource: %v", resources.Working[0])
	}
}

func TestUnpartitioned(t *testing.T) {
	d1 := NewDummy(1, 1)
	d2 := NewDummy(2, 2)
	d3 := NewDummy(3, 2)
	c := NewBackendResources(&CollectionConfig{
		Types: []TypeConfig{
			{Type: d1.Type(), Unpartitioned: true},
		},
	})

	c.Add(d1)
	if c.Collection[d1.Type()].Len() != 1 {
		t.Errorf("expected length 1 but got %d", len(c.Collection))
	}
	c.Add(d2)
	if c.Collection[d1.Type()].Len() != 2 {
		t.Errorf("expected length 2 but got %d", len(c.Collection))
	}
	// d3 has the same unique ID as d2 but a different object ID.  Our
	// collection should update d2 but not create a new element.
	c.Add(d3)
	if c.Collection[d1.Type()].Len() != 2 {
		t.Errorf("expected length 2 but got %d", len(c.Collection))
	}

	hashring := c.GetHashring("", d3.Type())
	elems, err := hashring.GetMany(Hashkey(0), 2)
	if err != nil {
		t.Errorf(err.Error())
	}
	if elems[0] != d1 {
		t.Errorf("got unexpected element")
	}
	if elems[1] != d3 {
		t.Errorf("got unexpected element: %d", elems[1].Oid())
	}

	hashring = c.GetHashring("the name should not matter", d3.Type())
	if hashring.Len() != 2 {
		t.Errorf("Didn't get the right number of unpartitioned resources: %d", hashring.Len())
	}
}

func TestNoneDistributor(t *testing.T) {
	dnone := NewDummy(1, 1)
	dnone.Distribution = "none"
	dnone.RelationIds = []string{"fingerprint"}
	dany := NewDummy(2, 2)
	dany.RelationIds = []string{"fingerprint"}
	dany2 := NewDummy(3, 3)
	dany2.RelationIds = []string{"fingerprint"}

	c := NewBackendResources(&collectionConfig)
	c.Add(dnone)
	c.Add(dany)
	c.Add(dany2)

	hashring := c.GetHashring(partitionName, dany.Type())
	if hashring.Len() != 2 {
		t.Errorf("Didn't get the right number of resources: %d", hashring.Len())
	}
	elems, err := hashring.GetMany(Hashkey(0), 2)
	if err != nil {
		t.Errorf(err.Error())
	}
	if elems[0] != dany {
		t.Errorf("got unexpected element")
	}
	if elems[1] != dany2 {
		t.Errorf("got unexpected element: %d", elems[1].Oid())
	}

	hashring = c.GetHashring("none", dany.Type())
	if hashring.Len() != 1 {
		t.Errorf("Didn't get the right number of resources: %d", hashring.Len())
	}
	elems, err = hashring.GetMany(Hashkey(0), 1)
	if err != nil {
		t.Errorf(err.Error())
	}
	if elems[0] != dnone {
		t.Errorf("got unexpected element")
	}
}

func TestUnknownDistributor(t *testing.T) {
	dnone := NewDummy(1, 1)
	dnone.Distribution = "unrecognized"
	dnone.RelationIds = []string{"fingerprint"}
	dany := NewDummy(2, 2)
	dany.RelationIds = []string{"fingerprint"}
	dany2 := NewDummy(3, 3)
	dany2.RelationIds = []string{"fingerprint"}

	c := NewBackendResources(&collectionConfig)
	c.Add(dnone)
	c.Add(dany)
	c.Add(dany2)

	hashring := c.GetHashring(partitionName, dany.Type())
	if hashring.Len() != 2 {
		t.Errorf("Didn't get the right number of resources: %d", hashring.Len())
	}
	elems, err := hashring.GetMany(Hashkey(0), 2)
	if err != nil {
		t.Errorf(err.Error())
	}
	if elems[0] != dany {
		t.Errorf("got unexpected element")
	}
	if elems[1] != dany2 {
		t.Errorf("got unexpected element: %d", elems[1].Oid())
	}

	hashring = c.GetHashring("none", dany.Type())
	if hashring.Len() != 1 {
		t.Errorf("Didn't get the right number of resources: %d", hashring.Len())
	}
	elems, err = hashring.GetMany(Hashkey(0), 1)
	if err != nil {
		t.Errorf(err.Error())
	}
	if elems[0] != dnone {
		t.Errorf("got unexpected element")
	}
}

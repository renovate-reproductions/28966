// Copyright (c) 2024, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package core

import (
	"math/rand"
	"os"
	"testing"
)

var (
	multipleProportions = map[string]int{"partition1": 1, "partition2": 1}
	newDummy            = func() Resource { return NewDummy(0, 0) }
)

func TestRelations(t *testing.T) {
	d1 := NewDummy(1, 1)
	d1.RelationIds = []string{"fingerprint1"}
	d2 := NewDummy(2, 2)
	d2.RelationIds = []string{"fingerprint2"}
	d1related := NewDummy(3, 3)
	d1related.RelationIds = []string{"fingerprint1"}

	// Use Monte Carlo simulation to test if the proportions behave as they
	// should.
	hits := 0
	runs := 10000
	for i := 0; i < runs; i++ {
		d1.UniqueId = Hashkey(rand.Uint64())
		d2.UniqueId = Hashkey(rand.Uint64())
		d1related.UniqueId = Hashkey(rand.Uint64())

		c := NewCollection(&CollectionConfig{
			Types: []TypeConfig{
				{Type: d1.Type(), Proportions: multipleProportions},
			},
		})
		c.Add(d1)
		c.Add(d2)
		c.Add(d1related)

		d1partition := c[d1.Type()].getPartitionName(d1)
		d2partition := c[d2.Type()].getPartitionName(d2)
		d1relatedpartition := c[d1related.Type()].getPartitionName(d1related)

		if d1partition != d1relatedpartition {
			t.Fatal("related resources have different partitions")
		}
		if d1partition != d2partition {
			hits++
		}
	}

	// Did we get more or less the right number of hits?
	// Are d1 and d2 50% of the time in different partitions?
	tolerance := 500
	lowerLimit := hits + hits - tolerance
	upperLimit := hits + hits + tolerance

	if runs <= lowerLimit {
		t.Errorf("got unexpectedly small number of hits")
	}
	if runs >= upperLimit {
		t.Errorf("got unexpectedly large number of hits")
	}
}

func TestStoreRelations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "core-test-")
	if err != nil {
		t.Fatal("Can't create a temp dir:", err)
	}
	defer os.RemoveAll(tmpDir)

	d := NewDummy(1, 1)
	d.RelationIds = []string{"fingerprint"}

	c := NewCollection(&CollectionConfig{
		StorageDir: tmpDir,
		Types: []TypeConfig{
			{Type: d.Type(), NewResource: newDummy, Proportions: multipleProportions},
		},
	})
	d.UniqueId = Hashkey(rand.Uint64())
	c.Add(d)
	c.Save()
	partitionName := c[d.Type()].getPartitionName(d)

	// Let's run it few times as there is some 50% chance to get the resource in the same partition by it's UID
	runs := 10
	for i := 0; i < runs; i++ {
		c = NewCollection(&CollectionConfig{
			StorageDir: tmpDir,
			Types: []TypeConfig{
				{Type: d.Type(), NewResource: newDummy, Proportions: multipleProportions},
			},
		})
		d.UniqueId = Hashkey(rand.Uint64())
		c.Add(d)
		if partitionName != c[d.Type()].getPartitionName(d) {
			t.Fatal("Loading collection from storage got the resource in the wrong partition")
		}
	}
}

func TestStorePartitionedResources(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "core-test-")
	if err != nil {
		t.Fatal("Can't create a temp dir:", err)
	}
	defer os.RemoveAll(tmpDir)

	d := NewDummy(1, 1)
	d.RelationIds = []string{"fingerprint"}

	c := NewCollection(&CollectionConfig{
		StorageDir: tmpDir,
		Types: []TypeConfig{
			{Type: d.Type(), NewResource: newDummy, Proportions: multipleProportions, Stored: true},
		},
	})
	c.Add(d)
	c.Save()

	c = NewCollection(&CollectionConfig{
		StorageDir: tmpDir,
		Types: []TypeConfig{
			{Type: d.Type(), NewResource: newDummy, Proportions: multipleProportions, Stored: true},
		},
	})
	resources := c[d.Type()].GetAll()
	if len(resources) != 1 {
		t.Fatal("Unexpected number of resources:", len(resources))
	}
	if d.Uid() != resources[0].Uid() {
		t.Error("Not the same resource:", d.Uid(), resources[0].Uid())
	}
}

func TestStoreUnpartitionedResources(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "core-test-")
	if err != nil {
		t.Fatal("Can't create a temp dir:", err)
	}
	defer os.RemoveAll(tmpDir)

	d := NewDummy(1, 1)
	d.RelationIds = []string{"fingerprint"}

	c := NewCollection(&CollectionConfig{
		StorageDir: tmpDir,
		Types: []TypeConfig{
			{Type: d.Type(), NewResource: newDummy, Unpartitioned: true, Stored: true},
		},
	})
	c.Add(d)
	c.Save()

	c = NewCollection(&CollectionConfig{
		StorageDir: tmpDir,
		Types: []TypeConfig{
			{Type: d.Type(), NewResource: newDummy, Unpartitioned: true, Stored: true},
		},
	})
	resources := c[d.Type()].GetAll()
	if len(resources) != 1 {
		t.Fatal("Unexpected number of resources:", len(resources))
	}
	if d.Uid() != resources[0].Uid() {
		t.Error("Not the same resource:", d.Uid(), resources[0].Uid())
	}
}

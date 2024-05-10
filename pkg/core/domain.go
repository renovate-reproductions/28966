// Copyright (c) 2021-2024, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package core

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// The following constants represent the states that a resource can be in.
	// Before rdsys had a chance to ask bridgestrap about a resource's state,
	// it's untested.  Afterwards, it's either functional or not functional.
	StateUntested = iota
	StateFunctional
	StateDysfunctional
)

const (
	// The following constants act as a crude representation of the bandwidth speed
	// that a resource can have. This is meant as a way to check whether or not the ratio
	// meets the bandwidth threshold for the given resource without requiring the context.
	// Before an onbasca test, a resource's speed is untested.  Afterwards, it's either
	// sufficient or insufficient.
	SpeedUntested = iota
	SpeedAccepted
	SpeedRejected
)

// Resource specifies the resources that rdsys hands out to users.  This could
// be a vanilla Tor bridge, and obfs4 bridge, a Snowflake proxy, and even Tor
// Browser links.  Your imagination is the limit.
type Resource interface {
	Type() string
	String() string
	IsValid() bool
	BlockedIn() LocationSet
	SetBlockedIn(LocationSet)
	SetLastPassed(time.Time)
	// Uid returns the resource's unique identifier.  Bridges with different
	// fingerprints have different unique identifiers.
	Uid() Hashkey
	// Oid returns the resource's object identifier.  Bridges with the *same*
	// fingerprint but different, say, IP addresses have different object
	// identifiers.  If two resources have the same Oid, they must have the
	// same Uid but not vice versa.
	Oid() Hashkey

	// RelationIdentifiers retrunrs a list of identifiers that represent a
	// relation between resources. For example the fingerprint, two resources
	// with the same fingerprint are related to eachother.
	RelationIdentifiers() []string

	Test()
	TestResult() *ResourceTest
	// Expiry returns the duration after which the resource should be deleted
	// from the backend (if the backend hasn't received an update).
	Expiry() time.Duration

	// Distributor set for this resource
	Distributor() string
}

// ResourceTest represents the result of a test of a resource.  We use the tool
// bridgestrap for testing if the bridge is functional:
// https://gitlab.torproject.org/tpo/anti-censorship/bridgestrap
// And onbasca to test it's speed ratio:
// https://gitlab.torproject.org/tpo/network-health/onbasca/
type ResourceTest struct {
	State      int       `json:"-"`
	Speed      int       `json:"-"`
	Ratio      *float64  `json:"-"`
	LastTested time.Time `json:"-"`
	LastPassed time.Time `json:"last_passed"`
	Error      string    `json:"-"`
}

// ResourceMap maps a resource type to a slice of respective resources.
type ResourceMap map[string]ResourceQueue

// ResourceQueue implements a queue of resources.
type ResourceQueue []Resource

// Enqueue adds a resource to the queue.  The function returns an error if the
// resource already exists in the queue.
func (q *ResourceQueue) Enqueue(r1 Resource) error {
	for _, r2 := range *q {
		if r1.Uid() == r2.Uid() {
			return errors.New("resource already exists")
		}
	}
	*q = append(*q, r1)
	return nil
}

// Dequeue return and removes the oldest resource in the queue.  If the queue
// is empty, the function returns an error.
func (q *ResourceQueue) Dequeue() (Resource, error) {
	if len(*q) == 0 {
		return nil, errors.New("queue is empty")
	}

	r := (*q)[0]
	if len(*q) > 1 {
		*q = (*q)[1:]
	} else {
		*q = []Resource{}
	}

	return r, nil
}

// Delete removes the resource from the queue.  If the queue is empty, the
// function returns an error.
func (q *ResourceQueue) Delete(r1 Resource) error {
	if len(*q) == 0 {
		return errors.New("queue is empty")
	}

	// See the following article on why this works:
	// https://github.com/golang/go/wiki/SliceTricks#filtering-without-allocating
	new := (*q)[:0]
	for _, r2 := range *q {
		if r1.Uid() != r2.Uid() {
			new = append(new, r2)
		}
	}

	*q = new
	return nil
}

// Update updates an existing resource if its unique ID matches the unique ID
// of the given resource.  If the queue is empty, the function returns an
// error.
func (q *ResourceQueue) Update(r1 Resource) error {
	if len(*q) == 0 {
		return errors.New("queue is empty")
	}

	for i, r2 := range *q {
		if r1.Uid() == r2.Uid() {
			(*q)[i] = r1
		}
	}

	return nil
}

// Search searches the resource queue for the given unique ID and either
// returns the resource it found, or an error if the resource could not be
// found.
func (q *ResourceQueue) Search(key Hashkey) (Resource, error) {
	if len(*q) == 0 {
		return nil, errors.New("queue is empty")
	}

	for _, r := range *q {
		if r.Uid() == key {
			return r, nil
		}
	}
	return nil, errors.New("resource not found")
}

// String returns a string representation of the resource map that's easy on
// the eyes.
func (m ResourceMap) String() string {
	if len(m) == 0 {
		return "empty"
	}

	s := []string{}
	for rType, queue := range m {
		s = append(s, fmt.Sprintf("%s: %d", rType, len(queue)))
	}
	return strings.Join(s, ", ")
}

// ApplyDiff applies the given ResourceDiff to the ResourceMap.  New resources
// are added, changed resources are updated, and gone resources are removed.
func (m ResourceMap) ApplyDiff(d *ResourceDiff) {
	resmap := m
	if d.FullUpdate {
		resmap = make(ResourceMap)
		for t := range m {
			resmap[t] = make(ResourceQueue, 0, len(d.New[t]))
		}
	}

	for rType, resources := range d.New {
		for _, r := range resources {
			q := resmap[rType]
			q.Enqueue(r)
			resmap[rType] = q
		}
	}

	for rType, resources := range d.Changed {
		for _, r := range resources {
			q := resmap[rType]
			q.Update(r)
			resmap[rType] = q
		}
	}

	for rType, resources := range d.Gone {
		for _, r := range resources {
			q := resmap[rType]
			q.Delete(r)
			resmap[rType] = q
		}
	}

	if d.FullUpdate {
		for t := range m {
			m[t] = resmap[t]
		}
	}
}

// Location represents the physical and topological location of a resource or
// requester.
type Location struct {
	CountryCode string // ISO 3166-1 alpha-2 country code, e.g. "AR".
	ASN         uint32 // Autonomous system number, e.g. 1234.
}

// String returns the string representation of the given location, e.g. "RU
// 1234".
func (l *Location) String() string {
	if l.ASN == 0 {
		return fmt.Sprintf("%s", l.CountryCode)
	} else {
		return fmt.Sprintf("%s (%d)", l.CountryCode, l.ASN)
	}
}

// LocationSet maps the string representation of locations (because we cannot
// use structs as map keys) to 'true'.
type LocationSet map[string]bool

// String returns a string representation of the given location set.
func (l LocationSet) String() string {

	ls := []string{}
	for key := range l {
		ls = append(ls, key)
	}
	return strings.Join(ls, ", ")
}

// HasLocationsNotIn returns true if s1 contains at least one location that is
// not in s2.
func (s1 LocationSet) HasLocationsNotIn(s2 LocationSet) bool {
	for key := range s1 {
		if _, exists := s2[key]; !exists {
			return true
		}
	}
	return false
}

// ResourceBase provides a data structure plus associated methods that are
// shared across all of our resources.
type ResourceBase struct {
	RType      string      `json:"type"`
	RBlockedIn LocationSet `json:"blocked_in"`
	Location   *Location
	Test       *ResourceTest `json:"test_result"`
}

// NewResourceBase returns a new ResourceBase.
func NewResourceBase() *ResourceBase {
	test := &ResourceTest{State: StateUntested}
	return &ResourceBase{RBlockedIn: make(LocationSet), Test: test}
}

// Type returns the resource's type.
func (r *ResourceBase) Type() string {
	return r.RType
}

// SetType sets the resource's type to the given type.
func (r *ResourceBase) SetType(Type string) {
	r.RType = Type
}

// TestResult returns the resource's test result.
func (r *ResourceBase) TestResult() *ResourceTest {
	return r.Test
}

// BlockedIn returns the set of locations that block the resource.
func (r *ResourceBase) BlockedIn() LocationSet {
	return r.RBlockedIn
}

// SetBlockedIn adds the given location set to the set of locations that block
// the resource.
func (r *ResourceBase) SetBlockedIn(l LocationSet) {
	for key := range l {
		r.RBlockedIn[key] = true
	}
}

// SetLastPassed sets the resource's last passed time to the time the test last passed
func (r *ResourceBase) SetLastPassed(lptime time.Time) {
	r.Test.LastPassed = lptime
}

// ResourceRequest represents a request for resources.  Distributors use
// ResourceRequest to request resources from the backend.
type ResourceRequest struct {
	// Name of requesting distributor.
	RequestOrigin string             `json:"request_origin"`
	ResourceTypes []string           `json:"resource_types"`
	Receiver      chan *ResourceDiff `json:"-"`
}

// HasResourceType returns true if the resource request contains the given
// resource type.
func (r *ResourceRequest) HasResourceType(rType1 string) bool {

	for _, rType2 := range r.ResourceTypes {
		if rType1 == rType2 {
			return true
		}
	}
	return false
}

func StateToString(state int) string {
	var str string
	switch state {
	case StateUntested:
		str = "untested"
	case StateFunctional:
		str = "functional"
	case StateDysfunctional:
		str = "dysfunctional"
	default:
		str = "unknown"
	}
	return str
}

func SpeedToString(speed int) string {
	var str string
	switch speed {
	case StateUntested:
		str = "untested"
	case StateFunctional:
		str = "accepted"
	case StateDysfunctional:
		str = "rejected"
	default:
		str = "unknown"
	}
	return str
}

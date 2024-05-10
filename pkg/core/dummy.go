// Copyright (c) 2021-2024, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package core

import (
	"fmt"
	"time"
)

// Dummy implements a simple Resource, which we use in unit tests.
type Dummy struct {
	ObjectId     Hashkey
	UniqueId     Hashkey
	ExpiryTime   time.Duration
	test         *ResourceTest
	testFunc     func(Resource)
	Distribution string
	RelationIds  []string
}

func NewDummy(oid Hashkey, uid Hashkey) *Dummy {
	return &Dummy{
		ObjectId:    oid,
		UniqueId:    uid,
		test:        &ResourceTest{State: StateFunctional, Speed: SpeedAccepted},
		ExpiryTime:  time.Hour,
		RelationIds: []string{},
	}
}
func (d *Dummy) Oid() Hashkey {
	return d.ObjectId
}
func (d *Dummy) Uid() Hashkey {
	return d.UniqueId
}
func (d *Dummy) RelationIdentifiers() []string {
	return d.RelationIds
}
func (d *Dummy) String() string {
	return fmt.Sprintf("dummy-%d-%d", d.UniqueId, d.ObjectId)
}
func (d *Dummy) Type() string {
	return "dummy"
}
func (d *Dummy) TestResult() *ResourceTest {
	return d.test
}
func (d *Dummy) Test() {
	if d.testFunc != nil {
		d.testFunc(d)
	}
}

func (d *Dummy) LastPassed() time.Time {
	return time.Now().UTC()
}

func (d *Dummy) SetLastPassed(time.Time) {
}

func (d *Dummy) SetTestFunc(f func(Resource)) {
	d.testFunc = f
}
func (d *Dummy) SetTest(t *ResourceTest) {
	d.test = t
}
func (d *Dummy) Expiry() time.Duration {
	return d.ExpiryTime
}
func (d *Dummy) Distributor() string {
	return d.Distribution
}
func (d *Dummy) IsValid() bool {
	return true
}
func (d *Dummy) BlockedIn() LocationSet {
	return make(LocationSet)
}
func (d *Dummy) SetBlockedIn(LocationSet) {
}

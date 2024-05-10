// Copyright (c) 2021-2024, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package core

import (
	"errors"
	"log"
	"math/rand"
	"sort"
)

// stencil is a list of intervals to partition hashrings.
// Distributor-specific stencils make it easy to
// deterministically select non-overlapping subsets of a hashring that should
// be given to a distributor.
type stencil struct {
	intervals []*interval
}

// interval represents a numerical interval.
type interval struct {
	Begin int
	End   int
	Name  string
}

// buildStencil turns the partition proportions into an interval chain,
// which helps us determine what partition a given resource should map to.
func buildStencil(proportions map[string]int) *stencil {

	var keys []string
	for key := range proportions {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	stencil := &stencil{}
	i := 0
	for _, k := range keys {
		stencil.AddInterval(&interval{i, i + proportions[k] - 1, k})
		i += proportions[k]
	}
	return stencil
}

// Contains returns 'true' if the given number n falls into the interval [a, b]
// so that a <= n <= b.
func (i *interval) Contains(n int) bool {
	return i.Begin <= n && n <= i.End
}

// FindByValue attempts to return the interval that the given number falls into
// and an error otherwise.
func (s *stencil) FindByValue(n int) (*interval, error) {
	for _, interval := range s.intervals {
		if interval.Contains(n) {
			return interval, nil
		}
	}
	return nil, errors.New("no interval that contains given value")
}

// AddInterval adds the given interval to the stencil.
func (s *stencil) AddInterval(i *interval) {
	s.intervals = append(s.intervals, i)
}

// GetUpperEnd returns the the maximum of all intervals of the stencil.
func (s *stencil) GetUpperEnd() (int, error) {

	if len(s.intervals) == 0 {
		return 0, errors.New("cannot determine upper end of empty stencil")
	}

	max := 0
	for _, interval := range s.intervals {
		if interval.End > max {
			max = interval.End
		}
	}
	return max, nil
}

func (s *stencil) GetPartitionName(r Resource) string {
	// FIXME: once we have bridge storage in place we should change the assigning function
	//        rand.Seed is deprecated and doesn't support concurrency
	upperEnd, err := s.GetUpperEnd()
	if err != nil {
		log.Printf("Can't get stenctip upper end: %v", err)
		return ""
	}

	seed := r.Uid()
	rand.Seed(int64(seed))
	n := rand.Intn(upperEnd + 1)

	i, err := s.FindByValue(n)
	if err != nil {
		log.Printf("Bug: resource %q does not fall in any interval.", r.String())
		return ""
	}
	return i.Name
}

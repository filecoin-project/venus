package blockstoreutil

import (
	"sync"

	"github.com/ipfs/go-cid"
)

// Set is a implementation of a set of Cids, that is, a structure
// to which holds a single copy of every Cids that is added to it.
type Set struct {
	set map[cid.Cid]struct{}
	lk  sync.Mutex
}

// NewSet initializes and returns a new Set.
func NewSet() *Set {
	return &Set{set: make(map[cid.Cid]struct{}), lk: sync.Mutex{}}
}

// Add puts a Cid in the Set.
func (s *Set) Add(c cid.Cid) {
	s.lk.Lock()
	defer s.lk.Unlock()
	s.set[c] = struct{}{}
}

// Add puts a Cid in the Set.
func (s *Set) add(c cid.Cid) {
	s.set[c] = struct{}{}
}

// Has returns if the Set contains a given Cid.
func (s *Set) Has(c cid.Cid) bool {
	s.lk.Lock()
	defer s.lk.Unlock()
	_, ok := s.set[c]
	return ok
}

// Has returns if the Set contains a given Cid.
func (s *Set) has(c cid.Cid) bool {
	_, ok := s.set[c]
	return ok
}

// Remove deletes a Cid from the Set.
func (s *Set) Remove(c cid.Cid) {
	s.lk.Lock()
	defer s.lk.Unlock()
	delete(s.set, c)
}

// Len returns how many elements the Set has.
func (s *Set) Len() int {
	s.lk.Lock()
	defer s.lk.Unlock()
	return len(s.set)
}

// Keys returns the Cids in the set.
func (s *Set) Keys() []cid.Cid {
	s.lk.Lock()
	defer s.lk.Unlock()
	out := make([]cid.Cid, 0, len(s.set))
	for k := range s.set {
		out = append(out, k)
	}
	return out
}

// Visit adds a Cid to the set only if it is
// not in it already.
func (s *Set) Visit(c cid.Cid) bool {
	s.lk.Lock()
	defer s.lk.Unlock()
	if !s.has(c) {
		s.add(c)
		return true
	}

	return false
}

// ForEach allows to run a custom function on each
// Cid in the set.
func (s *Set) ForEach(f func(c cid.Cid) error) error {
	s.lk.Lock()
	defer s.lk.Unlock()
	for c := range s.set {
		err := f(c)
		if err != nil {
			return err
		}
	}
	return nil
}

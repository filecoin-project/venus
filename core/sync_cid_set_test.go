package core

import (
	"math/rand"
	"sync"
	"testing"

	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

var testRand *rand.Rand
var randlk sync.Mutex

func init() {
	testRand = rand.New(rand.NewSource(42))
}

func randCid() *cid.Cid {
	// The underlying rand.NewSource() is not safe for concurrent access, so we lock calling into testRand.
	randlk.Lock()
	defer randlk.Unlock()

	pref := cid.NewPrefixV0(mh.BLAKE2B_MIN + 31)
	data := make([]byte, 16)
	testRand.Read(data)

	c, err := pref.Sum(data)
	if err != nil {
		panic(err)
	}
	return c
}

func TestCidSetAsync(t *testing.T) {
	s := &SyncCidSet{set: cid.NewSet()}
	for i := 0; i < 4; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				c := randCid()
				s.Add(c)
				if !s.Has(c) {
					t.Error("expected to have cid: ", c)
				}
			}
		}()
	}
}

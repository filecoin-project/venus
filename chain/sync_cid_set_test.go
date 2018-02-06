package chain

import (
	"math/rand"
	"testing"

	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

func randCid() *cid.Cid {
	pref := cid.NewPrefixV0(mh.BLAKE2B_MIN + 31)
	data := make([]byte, 16)
	rand.Read(data)

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

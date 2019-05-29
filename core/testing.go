package core

import (
	"context"
	//	"math/big"
	//	"math/rand"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

// MustGetNonce returns the next nonce for an actor at an address or panics.
func MustGetNonce(st state.Tree, a address.Address) uint64 {
	ctx := context.Background()
	actr, err := st.GetActor(ctx, a)
	if err != nil {
		panic(err)
	}

	nonce, err := actor.NextNonce(actr)
	if err != nil {
		panic(err)
	}
	return nonce
}

// MustAdd adds the given messages to the messagepool or panics if it cannot.
func MustAdd(p *MessagePool, height uint64, msgs ...*types.SignedMessage) {
	ctx := context.Background()
	for _, m := range msgs {
		if _, err := p.Add(ctx, m, height); err != nil {
			panic(err)
		}
	}
}

// MustConvertParams abi encodes the given parameters into a byte array (or panics)
func MustConvertParams(params ...interface{}) []byte {
	vals, err := abi.ToValues(params)
	if err != nil {
		panic(err)
	}

	out, err := abi.EncodeValues(vals)
	if err != nil {
		panic(err)
	}
	return out
}

// NewChainWithMessages creates a chain of tipsets containing the given messages
// and stores them in the given store.  Note the msg arguments are slices of
// slices of messages -- each slice of slices goes into a successive tipset,
// and each slice within this slice goes into a block of that tipset
func NewChainWithMessages(store *hamt.CborIpldStore, root types.TipSet, msgSets ...[][]*types.SignedMessage) []types.TipSet {
	tipSets := []types.TipSet{}
	parents := root
	height := uint64(0)

	// only add root to the chain if it is not the zero-valued-tipset
	if len(parents) != 0 {
		for _, blk := range parents {
			MustPut(store, blk)
		}
		tipSets = append(tipSets, parents)
		height, _ = parents.Height()
		height++
	}

	for _, tsMsgs := range msgSets {
		ts := types.TipSet{}
		// If a message set does not contain a slice of messages then
		// add a tipset with no messages and a single block to the chain
		if len(tsMsgs) == 0 {
			child := &types.Block{
				Height:  types.Uint64(height),
				Parents: parents.ToSortedCidSet(),
			}
			MustPut(store, child)
			ts[child.Cid()] = child
		}
		for _, msgs := range tsMsgs {
			child := &types.Block{
				Messages: msgs,
				Parents:  parents.ToSortedCidSet(),
				Height:   types.Uint64(height),
			}
			MustPut(store, child)
			ts[child.Cid()] = child
		}
		tipSets = append(tipSets, ts)
		parents = ts
		height++
	}

	return tipSets
}

// MustPut stores the thingy in the store or panics if it cannot.
func MustPut(store *hamt.CborIpldStore, thingy interface{}) cid.Cid {
	cid, err := store.Put(context.Background(), thingy)
	if err != nil {
		panic(err)
	}
	return cid
}

// MustDecodeCid decodes a string to a Cid pointer, panicking on error
func MustDecodeCid(cidStr string) cid.Cid {
	decode, err := cid.Decode(cidStr)
	if err != nil {
		panic(err)
	}

	return decode
}

// CreateStorages creates an empty state tree and storage map.
func CreateStorages(ctx context.Context, t *testing.T) (state.Tree, vm.StorageMap) {
	cst := hamt.NewCborStore()
	d := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(d)
	blk, err := consensus.DefaultGenesis(cst, bs)
	require.NoError(t, err)

	st, err := state.LoadStateTree(ctx, cst, blk.StateRoot, builtin.Actors)
	require.NoError(t, err)

	vms := vm.NewStorageMap(bs)

	return st, vms
}

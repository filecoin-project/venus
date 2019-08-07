package core

import (
	"context"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/types"
)

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

// msgBuild takes in the msgSet dictating which messages go on which block of
// a test tipset and returns a build function that adds these messages to the
// correct block using the chain.Builder.
func msgBuild(t *testing.T, msgSet [][]*types.SignedMessage) func(*chain.BlockBuilder, int) {
	return func(bb *chain.BlockBuilder, i int) {
		require.True(t, i <= len(msgSet))
		bb.AddMessages(msgSet[i], types.EmptyReceipts(len(msgSet[i])))
	}
}

// RequireChainWithMessages creates a chain of tipsets containing the given messages
// using the provided chain builder.  The builder stores the chain.  Note that
// each msgSet argument is a slice of message slices.  Each slice of slices
// goes into a successive tipset and each subslice goes into one tipset block.
// Precondition: the root tipset must be defined.  The chain of tipsets is
// returned.
func RequireChainWithMessages(t *testing.T, builder *chain.Builder, root types.TipSet, msgSets ...[][]*types.SignedMessage) []types.TipSet {
	var tipSets []types.TipSet
	parent := root
	require.True(t, parent.Defined())

	for _, tsMsgSet := range msgSets {
		if len(tsMsgSet) == 0 {
			parent = builder.BuildOn(parent, nil)
		} else {
			parent = builder.Build(parent, len(tsMsgSet), msgBuild(t, tsMsgSet))
		}
		tipSets = append(tipSets, parent)
	}
	return tipSets
}

// NewChainWithMessages creates a chain of tipsets containing the given messages
// and stores them in the given store.  Note the msg arguments are slices of
// slices of messages -- each slice of slices goes into a successive tipset,
// and each slice within this slice goes into a block of that tipset
func NewChainWithMessages(store *hamt.CborIpldStore, msgStore *chain.MessageStore, root types.TipSet, msgSets ...[][]*types.SignedMessage) []types.TipSet {
	var tipSets []types.TipSet
	parents := root
	height := uint64(0)
	stateRootCidGetter := types.NewCidForTestGetter()

	// only add root to the chain if it is not the zero-valued-tipset
	if parents.Defined() {
		for i := 0; i < parents.Len(); i++ {
			MustPut(store, parents.At(i))
		}
		tipSets = append(tipSets, parents)
		height, _ = parents.Height()
		height++
	}
	emptyMessagesCid, err := msgStore.StoreMessages(context.Background(), []*types.SignedMessage{})
	if err != nil {
		panic(err)
	}
	emptyReceiptsCid, err := msgStore.StoreReceipts(context.Background(), []*types.MessageReceipt{})
	if err != nil {
		panic(err)
	}

	for _, tsMsgs := range msgSets {
		var blocks []*types.Block
		// If a message set does not contain a slice of messages then
		// add a tipset with no messages and a single block to the chain
		if len(tsMsgs) == 0 {
			child := &types.Block{
				Height:          types.Uint64(height),
				Parents:         parents.Key(),
				Messages:        emptyMessagesCid,
				MessageReceipts: emptyReceiptsCid,
			}
			MustPut(store, child)
			blocks = append(blocks, child)
		}
		for _, msgs := range tsMsgs {
			msgsCid, err := msgStore.StoreMessages(context.Background(), msgs)
			if err != nil {
				panic(err)
			}

			child := &types.Block{
				Messages:        msgsCid,
				Parents:         parents.Key(),
				Height:          types.Uint64(height),
				StateRoot:       stateRootCidGetter(), // Differentiate all blocks
				MessageReceipts: emptyReceiptsCid,
			}
			MustPut(store, child)
			blocks = append(blocks, child)
		}
		ts, err := types.NewTipSet(blocks...)
		if err != nil {
			panic(err)
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

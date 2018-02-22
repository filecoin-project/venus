package core

import (
	"context"
	"math/big"
	"testing"

	"gx/ipfs/QmZhoiN2zi5SBBBKb181dQm4QdvWAvEwbppZvKpp4gRyNY/go-hamt-ipld"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/types"
)

func makeStateTree(cst *hamt.CborIpldStore, balances map[types.Address]*big.Int) (*cid.Cid, types.StateTree) {
	ctx := context.Background()
	t := types.NewEmptyTree(cst)
	for k, v := range balances {
		act := &types.Actor{Balance: v}
		if err := t.SetActor(ctx, k, act); err != nil {
			panic(err)
		}
	}
	c, err := t.Flush(ctx)
	if err != nil {
		panic(err)
	}

	return c, t
}

func TestProcessBlock(t *testing.T) {
	ctx := context.Background()
	cst := hamt.NewCborStore()

	addr1 := types.Address("one")
	addr2 := types.Address("two")
	stc, st := makeStateTree(cst, map[types.Address]*big.Int{
		addr1: big.NewInt(10000),
	})
	stc2, _ := makeStateTree(cst, map[types.Address]*big.Int{
		addr1: big.NewInt(10000 - 550),
		addr2: big.NewInt(550),
	})

	msg := types.NewMessage(addr1, addr2, big.NewInt(550), "", nil)

	blk := &types.Block{
		Height:    20,
		StateRoot: stc,
		Messages:  []*types.Message{msg},
	}

	assert.NoError(t, ProcessBlock(ctx, blk, st))

	stc2out, err := st.Flush(ctx)
	assert.NoError(t, err)

	assert.Equal(t, stc2, stc2out)
}

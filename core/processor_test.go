package core

import (
	"context"
	"math/big"
	"testing"

	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/QmdBXcN47jVwKLwSyN9e9xYVZ7WcAWgQ5N4cmNw7nzWq2q/go-hamt-ipld"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/types"
)

func makeStateTree(cst *hamt.CborIpldStore, balances map[types.Address]*big.Int) (*cid.Cid, *types.StateTree, error) {
	ctx := context.Background()
	t := types.NewEmptyStateTree(cst)

	for k, v := range balances {
		act, err := NewAccountActor(v)
		if err != nil {
			return nil, nil, err
		}

		if err := t.SetActor(ctx, k, act); err != nil {
			return nil, nil, err
		}
	}

	c, err := t.Flush(ctx)
	if err != nil {
		return nil, nil, err
	}

	return c, t, nil
}

func TestProcessBlock(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()
	cst := hamt.NewCborStore()

	addr1 := types.Address("one")
	addr2 := types.Address("two")
	stc, st, err := makeStateTree(cst, map[types.Address]*big.Int{
		addr1: big.NewInt(10000),
	})
	assert.NoError(err)
	stc2, _, err := makeStateTree(cst, map[types.Address]*big.Int{
		addr1: big.NewInt(10000 - 550),
		addr2: big.NewInt(550),
	})
	assert.NoError(err)

	msg := types.NewMessage(addr1, addr2, big.NewInt(550), "", nil)

	blk := &types.Block{
		Height:    20,
		StateRoot: stc,
		Messages:  []*types.Message{msg},
	}

	receipts, err := ProcessBlock(ctx, blk, st)
	assert.NoError(err)

	assert.Len(receipts, 1)

	stc2out, err := st.Flush(ctx)
	assert.NoError(err)

	assert.Equal(stc2, stc2out)
}

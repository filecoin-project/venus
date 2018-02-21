package types

import (
	"context"
	"math/big"
	"testing"

	"gx/ipfs/QmZhoiN2zi5SBBBKb181dQm4QdvWAvEwbppZvKpp4gRyNY/go-hamt-ipld"

	"github.com/stretchr/testify/assert"
)

func TestStatePutGet(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	cst := hamt.NewCborStore()
	tree := NewEmptyTree(cst)

	act1 := &Actor{Balance: big.NewInt(155), Nonce: 17}
	act2 := &Actor{Balance: big.NewInt(1799), Nonce: 1}

	addr1 := Address("foo")
	addr2 := Address("bar")

	assert.NoError(tree.SetActor(ctx, addr1, act1))
	assert.NoError(tree.SetActor(ctx, addr2, act2))

	act1out, err := tree.GetActor(ctx, addr1)
	assert.NoError(err)
	assert.Equal(act1, act1out)
	act2out, err := tree.GetActor(ctx, addr2)
	assert.NoError(err)
	assert.Equal(act2, act2out)

	// now test it persists across recreation of tree
	tcid, err := tree.Flush(ctx)
	assert.NoError(err)

	tree2, err := LoadStateTree(ctx, cst, tcid)
	assert.NoError(err)

	act1out2, err := tree2.GetActor(ctx, addr1)
	assert.NoError(err)
	assert.Equal(act1, act1out2)
	act2out2, err := tree2.GetActor(ctx, addr2)
	assert.NoError(err)
	assert.Equal(act2, act2out2)
}

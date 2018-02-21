package types

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"gx/ipfs/QmZhoiN2zi5SBBBKb181dQm4QdvWAvEwbppZvKpp4gRyNY/go-hamt-ipld"

	"github.com/stretchr/testify/assert"
)

func TestStatePutGet(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	cst := hamt.NewCborStore()
	tree := NewEmptyStateTree(cst)

	act1 := NewActor(AccountActorCodeCid, nil)
	act1.WriteStorage([]byte("hello"))
	act1.IncNonce()
	act2 := NewActor(AccountActorCodeCid, nil)
	act2.WriteStorage([]byte("world"))
	act2.IncNonce()
	act2.IncNonce()

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

func TestStateGetOrCreate(t *testing.T) {
	ctx := context.Background()
	cst := hamt.NewCborStore()
	tree := NewEmptyStateTree(cst)

	addr := Address("coolio")

	// no actor - error
	t.Run("no actor - error", func(t *testing.T) {
		assert := assert.New(t)

		actor, err := tree.GetOrCreateActor(ctx, addr, func() (*Actor, error) {
			return nil, fmt.Errorf("fail")
		})
		assert.EqualError(err, "fail")
		assert.Nil(actor)
	})

	t.Run("no actor - success", func(t *testing.T) {
		assert := assert.New(t)

		actor, err := tree.GetOrCreateActor(ctx, addr, func() (*Actor, error) {
			return &Actor{}, nil
		})
		assert.NoError(err)
		assert.Equal(actor, &Actor{})
	})

	t.Run("actor exists", func(t *testing.T) {
		assert := assert.New(t)

		actor := NewActor(nil, big.NewInt(10))
		assert.NoError(tree.SetActor(ctx, addr, actor))

		actorBack, err := tree.GetOrCreateActor(ctx, addr, func() (*Actor, error) {
			return &Actor{}, nil
		})
		assert.NoError(err)
		assert.Equal(actorBack, actor)
	})
}

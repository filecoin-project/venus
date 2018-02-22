package core

import (
	"context"
	"testing"

	"gx/ipfs/QmZhoiN2zi5SBBBKb181dQm4QdvWAvEwbppZvKpp4gRyNY/go-hamt-ipld"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/types"
)

func TestVMContextStorage(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	cst := hamt.NewCborStore()
	state := types.NewEmptyStateTree(cst)

	toActor, err := NewAccountActor(nil)
	assert.NoError(err)
	toAddr := types.Address("to")

	assert.NoError(state.SetActor(ctx, toAddr, toActor))

	msg := types.NewMessage(types.Address(""), toAddr, nil, "hello", nil)

	vmCtx := NewVMContext(nil, toActor, msg, state)

	assert.NoError(vmCtx.WriteStorage([]byte("hello")))

	// make sure we can read it back
	toActorBack, err := state.GetActor(ctx, toAddr)
	assert.NoError(err)

	storage := NewVMContext(nil, toActorBack, msg, state).ReadStorage()
	assert.Equal(storage, []byte("hello"))
}

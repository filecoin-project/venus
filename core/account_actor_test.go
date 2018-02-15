package core

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"gx/ipfs/QmdBXcN47jVwKLwSyN9e9xYVZ7WcAWgQ5N4cmNw7nzWq2q/go-hamt-ipld"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/types"
)

func TestAccountBalanceAndTransfer(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	from, err := NewAccountActor(big.NewInt(500))
	assert.NoError(err)
	fromAddr := types.Address("from")
	to := types.NewActor(types.AccountActorCid)
	toAddr := types.Address("to")
	me := types.NewActor(types.AccountActorCid)
	meAddr := types.Address("me")

	cst := hamt.NewCborStore()
	state := types.NewEmptyStateTree(cst)

	assert.NoError(state.SetActor(ctx, fromAddr, from))
	assert.NoError(state.SetActor(ctx, toAddr, to))
	assert.NoError(state.SetActor(ctx, meAddr, me))

	sendFrom := func(fAddr types.Address, tAddr types.Address, method string, value *big.Int, params []interface{}) []byte {
		t.Helper()

		// load actors to ensure they are current

		// only load from actor if one is passed in
		var fActor *types.Actor
		if fAddr != types.Address("") {
			a, err := state.GetActor(ctx, fAddr)
			assert.NoError(err)
			fActor = a
		}

		tActor, err := state.GetActor(ctx, tAddr)
		assert.NoError(err)

		execActor, err := LoadCode(tActor.Code())
		assert.NoError(err)
		t.Logf("to: %v", tActor)

		msg := types.NewMessage(fAddr, tAddr, value, method, params)
		ctx := NewVMContext(fActor, tActor, msg, state)

		ret, exitCode, err := MakeTypedExport(execActor, method)(ctx)
		assert.NoError(err)
		assert.Equal(exitCode, uint8(0))

		return ret
	}

	send := func(toAddr types.Address, method string, value *big.Int, params []interface{}) []byte {
		t.Helper()
		return sendFrom(meAddr, toAddr, method, value, params)
	}

	assertInt := func(actual []byte, expected int64) {
		t.Helper()
		actualInt := big.NewInt(0)
		actualInt.SetBytes(actual)
		assert.Equal(actualInt, big.NewInt(expected))
	}

	t.Log(`check that "from" has the set balance of 500, with no from address`)
	b0 := sendFrom(types.Address(""), fromAddr, "balance", nil, nil)
	assertInt(b0, 500)

	t.Log(`check that "from" has the set balance of 500`)
	b0 = send(fromAddr, "balance", nil, nil)
	assertInt(b0, 500)

	t.Log(`check that "to" has the set balance of 0`)
	b1 := send(toAddr, "balance", nil, nil)
	assertInt(b1, 0)

	t.Log(`transfer 100 from "from" to "to"`)
	_ = sendFrom(fromAddr, toAddr, "transfer", big.NewInt(100), nil)

	t.Log(`check the right amount was subtracted from "from"`)
	b2 := send(fromAddr, "balance", nil, nil)
	assertInt(b2, 400)

	t.Log(`check the right amount was added to "to"`)
	b3 := send(toAddr, "balance", nil, nil)
	assertInt(b3, 100)
}

func TestAccountActorMarshalStorage(t *testing.T) {
	assert := assert.New(t)

	t.Log("empty storage")
	emptyStorage := &AccountStorage{}
	outEmpty, err := MarshalStorage(emptyStorage)
	assert.NoError(err)

	emptyStorageBack, err := unmarshalAccountStorage(outEmpty)
	assert.NoError(err)
	assert.Equal(emptyStorage, emptyStorageBack)

	t.Log("storage with balance")
	storage := &AccountStorage{Balance: big.NewInt(10)}
	out, err := MarshalStorage(storage)
	assert.NoError(err)

	storageBack, err := unmarshalAccountStorage(out)
	assert.NoError(err)
	assert.Equal(storage, storageBack)
}

func TestAccountActorWithStorage(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	from, err := NewAccountActor(big.NewInt(0))
	assert.NoError(err)
	fromAddr := types.Address("from")

	to, err := NewAccountActor(big.NewInt(10))
	assert.NoError(err)
	toAddr := types.Address("to")

	cst := hamt.NewCborStore()
	state := types.NewEmptyStateTree(cst)

	assert.NoError(state.SetActor(ctx, fromAddr, from))
	assert.NoError(state.SetActor(ctx, toAddr, to))

	msg := types.NewMessage(fromAddr, toAddr, nil, "balance", nil)
	vmCtx := NewVMContext(from, to, msg, state)

	t.Log("when no error is returned the changes are applied")
	result, err := withStorage(vmCtx, func(storage *AccountStorage) (interface{}, error) {
		assert.Equal(storage.Balance, big.NewInt(10))
		storage.Balance = big.NewInt(20)

		return "hello", nil
	})
	assert.NoError(err)
	assert.Equal(result.(string), "hello")

	_, err = withStorage(vmCtx, func(storage *AccountStorage) (interface{}, error) {
		assert.Equal(storage.Balance, big.NewInt(20))
		return nil, nil
	})
	assert.NoError(err)

	t.Log("when an error is returned the changes are not applied")
	_, err = withStorage(vmCtx, func(storage *AccountStorage) (interface{}, error) {
		storage.Balance = big.NewInt(1000)
		return nil, fmt.Errorf("fail")
	})
	assert.Error(err, "fail")

	_, err = withStorage(vmCtx, func(storage *AccountStorage) (interface{}, error) {
		assert.Equal(storage.Balance, big.NewInt(20))
		return nil, nil
	})
	assert.NoError(err)
}

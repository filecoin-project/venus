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

func TestTokenActor(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	tokenAddr := types.Address("token")
	fromAddr := types.Address("from")
	toAddr := types.Address("to")

	balances := map[types.Address]*Balance{}
	balances[fromAddr] = &Balance{
		Total: big.NewInt(500),
	}
	token, err := NewTokenActor(balances)
	assert.NoError(err)

	from, err := NewAccountActor()
	assert.NoError(err)

	cst := hamt.NewCborStore()
	state := types.NewEmptyStateTree(cst)

	assert.NoError(state.SetActor(ctx, tokenAddr, token))
	assert.NoError(state.SetActor(ctx, fromAddr, from))

	sendFrom := func(fAddr types.Address, tAddr types.Address, method string, params []interface{}) ([]byte, uint8, error) {
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

		msg := types.NewMessage(fAddr, tAddr, method, params)
		ctx := NewVMContext(fActor, tActor, msg, state)

		return MakeTypedExport(execActor, method)(ctx)
	}

	send := func(toAddr types.Address, method string, params []interface{}) ([]byte, uint8, error) {
		return sendFrom(fromAddr, toAddr, method, params)
	}

	sendSuccess := func(toAddr types.Address, method string, params []interface{}) []byte {
		ret, exitCode, err := send(toAddr, method, params)
		assert.NoError(err)
		assert.Equal(exitCode, uint8(0))
		return ret
	}

	sendFromSuccess := func(fromAddr, toAddr types.Address, method string, params []interface{}) []byte {
		ret, exitCode, err := sendFrom(fromAddr, toAddr, method, params)
		assert.NoError(err)
		assert.Equal(exitCode, uint8(0))
		return ret
	}

	assertInt := func(actual []byte, expected int64) {
		actualInt := big.NewInt(0)
		actualInt.SetBytes(actual)
		assert.Equal(big.NewInt(expected), actualInt)
	}

	t.Run("balance and transfer", func(t *testing.T) {
		t.Log(`check that "from" has the set balance of 500, with no from address`)
		b0 := sendFromSuccess(types.Address(""), tokenAddr, "balance", []interface{}{fromAddr})
		assertInt(b0, 500)

		t.Log(`check that "from" has the set balance of 500`)
		b0 = sendSuccess(tokenAddr, "balance", []interface{}{fromAddr})
		assertInt(b0, 500)

		t.Log(`check that "to" has a balance of 0`)
		b1 := sendSuccess(tokenAddr, "balance", []interface{}{toAddr})
		assertInt(b1, 0)

		t.Log(`transfer 100 from "from" to "to"`)
		_ = sendFromSuccess(fromAddr, tokenAddr, "transfer", []interface{}{toAddr, big.NewInt(100)})

		t.Log(`check the right amount was subtracted from "from"`)
		b2 := sendSuccess(tokenAddr, "balance", []interface{}{fromAddr})
		assertInt(b2, 400)

		t.Log(`check the right amount was added to "to"`)
		b3 := sendSuccess(tokenAddr, "balance", []interface{}{toAddr})
		assertInt(b3, 100)
	})

	t.Run("transfer fail", func(t *testing.T) {
		ret, exitCode, err := sendFrom(fromAddr, tokenAddr, "transfer", []interface{}{toAddr, big.NewInt(100000)})
		assert.Equal(err.Error(), "not enough balance")
		assert.Equal(exitCode, uint8(1))
		assert.Nil(ret)
	})
}

func TestTokenActorMarshalStorage(t *testing.T) {
	assert := assert.New(t)

	t.Log("empty storage")
	emptyStorage := &TokenStorage{}
	outEmpty, err := MarshalStorage(emptyStorage)
	assert.NoError(err)

	emptyStorageBack, err := unmarshalTokenStorage(outEmpty)
	assert.NoError(err)
	assert.Equal(emptyStorage, emptyStorageBack)

	t.Log("storage with balance")
	balances := map[types.Address]*Balance{}
	balances[types.Address("hello")] = &Balance{Total: big.NewInt(10)}
	balances[types.Address("world")] = &Balance{Total: big.NewInt(100)}

	storage := &TokenStorage{Balances: balances}
	out, err := MarshalStorage(storage)
	assert.NoError(err)

	storageBack, err := unmarshalTokenStorage(out)
	assert.NoError(err)
	assert.Equal(storage, storageBack)
}

func TestTokenActorWithStorage(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	fromAddr := types.Address("from")
	toAddr := types.Address("to")
	tokenAddr := types.Address("token")

	from, err := NewAccountActor()
	assert.NoError(err)

	to, err := NewAccountActor()
	assert.NoError(err)

	balances := map[types.Address]*Balance{}
	balances[fromAddr] = &Balance{Total: big.NewInt(10)}

	token, err := NewTokenActor(balances)
	assert.NoError(err)

	cst := hamt.NewCborStore()
	state := types.NewEmptyStateTree(cst)

	assert.NoError(state.SetActor(ctx, tokenAddr, token))
	assert.NoError(state.SetActor(ctx, fromAddr, from))
	assert.NoError(state.SetActor(ctx, toAddr, to))

	msg := types.NewMessage(fromAddr, toAddr, "balance", nil)
	vmCtx := NewVMContext(from, token, msg, state)

	t.Log("when no error is returned the changes are applied")
	result, err := withTokenStorage(vmCtx, func(storage *TokenStorage) (interface{}, error) {
		assert.Equal(storage.Balances[fromAddr].Total, big.NewInt(10))
		storage.Balances[fromAddr].Total = big.NewInt(20)

		return "hello", nil
	})
	assert.NoError(err)
	assert.Equal(result.(string), "hello")

	_, err = withTokenStorage(vmCtx, func(storage *TokenStorage) (interface{}, error) {
		assert.Equal(storage.Balances[fromAddr].Total, big.NewInt(20))
		return nil, nil
	})
	assert.NoError(err)

	t.Log("when an error is returned the changes are not applied")
	_, err = withTokenStorage(vmCtx, func(storage *TokenStorage) (interface{}, error) {
		storage.Balances[fromAddr].Total = big.NewInt(1000)
		return nil, fmt.Errorf("fail")
	})
	assert.Error(err, "fail")

	_, err = withTokenStorage(vmCtx, func(storage *TokenStorage) (interface{}, error) {
		assert.Equal(storage.Balances[fromAddr].Total, big.NewInt(20))
		return nil, nil
	})
	assert.NoError(err)
}

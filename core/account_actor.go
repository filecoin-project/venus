package core

import (
	"fmt"
	"math/big"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cbor "gx/ipfs/QmZpue627xQuNGXn7xHieSjSZ8N4jot6oBHwe9XTn3e4NU/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	cbor.RegisterCborType(AccountStorage{})
}

type AccountActor struct{}

type AccountStorage struct {
	Balance *big.Int
}

func (mem *AccountStorage) isStorage() bool {
	return true
}

var _ ExecutableActor = (*AccountActor)(nil)
var _ ActorStorage = (*AccountStorage)(nil)

// NewAccountActor creates a new Actor with a predefined balance.
func NewAccountActor(balance *big.Int) (*types.Actor, error) {
	storageBytes, err := MarshalStorage(&AccountStorage{
		Balance: balance,
	})
	if err != nil {
		return nil, err
	}
	return types.NewActorWithMemory(types.AccountActorCid, storageBytes), nil
}

var exports = Exports{
	"balance": &FunctionSignature{
		Params: []interface{}{},
		Return: &big.Int{},
	},
	"transfer": &FunctionSignature{
		Params: []interface{}{},
		Return: nil,
	},
	"subtract": &FunctionSignature{
		Params: []interface{}{&types.Message{}, &big.Int{}},
		Return: nil,
	},
	"main": &FunctionSignature{
		Params: []interface{}{},
		Return: nil,
	},
}

func withStorage(ctx *VMContext, f func(*AccountStorage) (interface{}, error)) (interface{}, error) {
	storage, err := unmarshalStorage(ctx.ReadStorage())
	if err != nil {
		return nil, err
	}

	ret, err := f(storage)

	newStorage, err := marshalStorage(storage)
	if err != nil {
		return nil, err
	}

	if err := ctx.WriteStorage(newStorage); err != nil {
		return nil, err
	}

	return ret, nil
}

// Exports makes the available methods for this contract available.
func (account *AccountActor) Exports() Exports {
	return exports
}

func (account *AccountActor) Main(ctx *VMContext) (uint8, error) {
	return 0, nil
}

func (account *AccountActor) Balance(ctx *VMContext) (*big.Int, uint8, error) {
	balance, err := withStorage(ctx, func(storage *AccountStorage) (interface{}, error) {
		return storage.Balance, nil
	})

	if err != nil {
		return nil, 1, err
	}

	return balance.(*big.Int), 0, nil
}

func (account *AccountActor) Transfer(ctx *VMContext) (uint8, error) {
	value := ctx.Message().Value()
	_, _, err := ctx.Send(ctx.Message().From(), "subtract", []interface{}{ctx.Message(), value})
	if err != nil {
		return 1, errors.Wrap(err, "failed to send message: subtract")
	}

	_, err = withStorage(ctx, func(storage *AccountStorage) (interface{}, error) {
		storage.Balance.Add(storage.Balance, value)
		return nil, nil
	})
	if err != nil {
		return 1, err
	}

	return 0, nil
}

func (account *AccountActor) Subtract(ctx *VMContext, msg *types.Message, value *big.Int) (uint8, error) {
	// validate we agreed to this value being sent
	// TODO: instead of passing the message, only send the cid, and fetch it from the state, to make sure it
	// is valid and included in the state tree
	// how can we prevent reuse?
	// 1. validate signature
	// TODO
	// 2. check we are the sender
	if msg.From() != ctx.Message().To() {
		return 1, fmt.Errorf("invalid sender")
	}
	// 3. check the value matches
	if value.Cmp(msg.Value()) != 0 {
		return 1, fmt.Errorf("invalid value")
	}

	_, err := withStorage(ctx, func(storage *AccountStorage) (interface{}, error) {
		// make sure enough is available
		if storage.Balance.Cmp(value) == -1 {
			return 1, fmt.Errorf("not enough balance")
		}

		storage.Balance.Sub(storage.Balance, value)
		return nil, nil
	})
	if err != nil {
		return 1, err
	}

	return 0, nil
}

func unmarshalStorage(raw []byte) (*AccountStorage, error) {
	storage := &AccountStorage{Balance: big.NewInt(0)}

	// no storage to initialize
	if len(raw) == 0 {
		return storage, nil
	}

	if err := UnmarshalStorage(raw, storage); err != nil {
		return nil, err
	}

	return storage, nil
}

func marshalStorage(storage *AccountStorage) ([]byte, error) {
	return MarshalStorage(storage)
}

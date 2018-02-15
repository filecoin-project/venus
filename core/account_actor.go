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

// AccountActor is the builtin actor for handling individual accounts.
//
// AccountActor __is__ shared shared between multiple accounts, as it is the
// underlying code.
type AccountActor struct{}

// AccountStorage is what the AccountActor uses to store data permanently
// onchain. It is unmarshalled & marshalled when needed, as only raw bytes
// can be stored onchain.
//
// AccountStorage __is not__ shared between multiple accounts, as it represents
// the individual instances of an account.
type AccountStorage struct {
	// Balance is the current balance in Filecoin of this account.
	Balance *big.Int
}

// ensure AccountActor is an ExecutableActor at compile time
var _ ExecutableActor = (*AccountActor)(nil)

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

// exports are the publicly (externally callable) methods of the AccountActor.
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
}

// Exports makes the available methods for this contract available.
func (account *AccountActor) Exports() Exports {
	return exports
}

// Balance retrieves the current balance of this account in Filecoin.
func (account *AccountActor) Balance(ctx *VMContext) (*big.Int, uint8, error) {
	balance, err := withStorage(ctx, func(storage *AccountStorage) (interface{}, error) {
		return storage.Balance, nil
	})

	if err != nil {
		return nil, 1, err
	}

	return balance.(*big.Int), 0, nil
}

// Transfer sends a specified amount of Filecoin from the sender of the message to this
// account.
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

// Subtract reduces the balance by the value sent.
//
// DANGER ZONE: This has critical security implications and as such needs to ensure that the Message
// passed in is valid and only used a single time.
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

// withStorage is a helper to initialize the accounts storage, operate on it, and then
// commit it again.
func withStorage(ctx *VMContext, f func(*AccountStorage) (interface{}, error)) (interface{}, error) {
	storage, err := unmarshalStorage(ctx.ReadStorage())
	if err != nil {
		return nil, err
	}

	ret, err := f(storage)
	if err != nil {
		return nil, err
	}

	newStorage, err := MarshalStorage(storage)
	if err != nil {
		return nil, err
	}

	if err := ctx.WriteStorage(newStorage); err != nil {
		return nil, err
	}

	return ret, nil
}

// unmarshalStorage initializes and unmarshales the account storage.
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

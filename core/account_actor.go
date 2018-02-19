package core

import (
	"bytes"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cbor "gx/ipfs/QmZpue627xQuNGXn7xHieSjSZ8N4jot6oBHwe9XTn3e4NU/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	cbor.RegisterCborType(AccountStorage{})
}

var (
	// ErrRequiredFrom is returned when a method requires a from address, but none was passed.
	ErrRequiredFrom = errors.New("message is missing required from")
)

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
}

// ensure AccountActor is an ExecutableActor at compile time
var _ ExecutableActor = (*AccountActor)(nil)

// NewAccountActor creates a new actor.
func NewAccountActor() (*types.Actor, error) {
	storageBytes, err := MarshalStorage(&AccountStorage{})
	if err != nil {
		return nil, err
	}
	return types.NewActorWithMemory(types.AccountActorCid, storageBytes), nil
}

// accountExports are the publicly (externally callable) methods of the AccountActor.
var accountExports = Exports{
	"verify": &FunctionSignature{
		Params: []interface{}{[]byte{}, &types.Message{}},
		Return: false,
	},
	"sign": &FunctionSignature{
		Params: []interface{}{&types.Message{}},
		Return: []byte{},
	},
}

// Exports makes the available methods for this contract available.
func (state *AccountActor) Exports() Exports {
	return accountExports
}

func (state *AccountActor) Verify(ctx *VMContext, signature []byte, msg *types.Message) (bool, uint8, error) {
	// TODO: actually verify
	return bytes.Equal(signature, []byte{1}), 0, nil
}

func (state *AccountActor) Sign(ctx *VMContext, msg *types.Message) ([]byte, uint8, error) {
	// TODO: actually sign
	return []byte{1}, 0, nil
}

// withAccountStorage is a helper to initialize the accounts storage, operate on it, and then
// commit it again.
func withAccountStorage(ctx *VMContext, f func(*AccountStorage) (interface{}, error)) (interface{}, error) {
	storage, err := loadAccountStorage(ctx)
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

// loadAccountStorage fetches the storage from the actor.
func loadAccountStorage(ctx *VMContext) (*AccountStorage, error) {
	return unmarshalAccountStorage(ctx.ReadStorage())
}

// unmarshalAccountStorage initializes and unmarshales the account storage.
func unmarshalAccountStorage(raw []byte) (*AccountStorage, error) {
	storage := &AccountStorage{}

	// no storage to initialize
	if len(raw) == 0 {
		return storage, nil
	}

	if err := UnmarshalStorage(raw, storage); err != nil {
		return nil, err
	}

	return storage, nil
}

package core

import (
	cbor "gx/ipfs/QmZpue627xQuNGXn7xHieSjSZ8N4jot6oBHwe9XTn3e4NU/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	cbor.RegisterCborType(types.CborEntryFromStruct(AccountStorage{}))
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
type AccountStorage struct{}

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
var accountExports = Exports{}

// Exports makes the available methods for this contract available.
func (state *AccountActor) Exports() Exports {
	return accountExports
}

package account

import (
	cbor "gx/ipfs/QmRiRJhn427YVuufBEHofLreKWNw7P7BWNq86Sb9kzqdbd/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	cbor.RegisterCborType(Storage{})
}

// Actor is the builtin actor responsible for individual accounts.
// The current iteration is not doing any useful work yet, and a simple placeholder.
// More details on future responsibilities can be found at https://github.com/filecoin-project/specs/blob/master/spec.md#account-actor.
//
// Actor __is__ shared shared between multiple accounts, as it is the
// underlying code.
// TODO make singleton vs not more clear
type Actor struct{}

// Storage is what the AccountActor uses to store data permanently
// onchain. It is unmarshalled & marshalled when needed, as only raw bytes
// can be stored onchain.
//
// Storage __is not__ shared between multiple accounts, as it represents
// the individual instances of an account.
type Storage struct{}

// NewStorage returns an empty AccountStorage struct
func (state *Actor) NewStorage() interface{} {
	return &Storage{}
}

// ensure AccountActor is an ExecutableActor at compile time
var _ exec.ExecutableActor = (*Actor)(nil)

// NewActor creates a new account actor.
func NewActor(balance *types.AttoFIL) (*types.Actor, error) {
	storageBytes, err := actor.MarshalStorage(&Storage{})
	if err != nil {
		return nil, err
	}
	return types.NewActorWithMemory(types.AccountActorCodeCid, balance, storageBytes), nil
}

// UpgradeActor converts the given actor to an account actor, leaving its balance and nonce in place
func UpgradeActor(act *types.Actor) error {
	act.Code = types.AccountActorCodeCid
	storageBytes, err := actor.MarshalStorage(&Storage{})
	if err != nil {
		return err
	}
	act.WriteStorage(storageBytes)
	return nil
}

// accountExports are the publicly (externally callable) methods of the AccountActor.
var accountExports = exec.Exports{}

// Exports makes the available methods for this contract available.
func (state *Actor) Exports() exec.Exports {
	return accountExports
}

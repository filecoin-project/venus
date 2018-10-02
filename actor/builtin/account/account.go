package account

import (
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
)

// Actor is the builtin actor responsible for individual accounts.
// More details on future responsibilities can be found at https://github.com/filecoin-project/specs/blob/master/spec.md#account-actor.
//
// Actor __is__ shared between multiple accounts, as it is the
// underlying code.
// TODO make singleton vs not more clear
type Actor struct{}

// Ensure AccountActor is an ExecutableActor at compile time.
var _ exec.ExecutableActor = (*Actor)(nil)

// NewActor creates a new account actor.
func NewActor(balance *types.AttoFIL) (*actor.Actor, error) {
	return actor.NewActor(types.AccountActorCodeCid, balance), nil
}

// UpgradeActor converts the given actor to an account actor, leaving its balance and nonce in place.
func UpgradeActor(act *actor.Actor) error {
	act.Code = types.AccountActorCodeCid
	return nil
}

// accountExports are the publicly (externally callable) methods of the AccountActor.
var accountExports = exec.Exports{}

// Exports makes the available methods for this contract available.
func (a *Actor) Exports() exec.Exports {
	return accountExports
}

// InitializeState for account actors does nothing.
func (a *Actor) InitializeState(_ exec.Storage, _ interface{}) error {
	return nil
}

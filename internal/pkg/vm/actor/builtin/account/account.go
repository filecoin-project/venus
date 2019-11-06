package account

import (
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
)

// Actor is the builtin actor responsible for individual accounts.
// More details on future responsibilities can be found at https://github.com/filecoin-project/specs/blob/master/spec.md#account-actor.
//
// Actor __is__ shared between multiple accounts, as it is the
// underlying code.
// TODO make singleton vs not more clear
type Actor struct{}

// NewActor creates a new account actor.
func NewActor(balance types.AttoFIL) (*actor.Actor, error) {
	return actor.NewActor(types.AccountActorCodeCid, balance), nil
}

// UpgradeActor converts the given actor to an account actor, leaving its balance and nonce in place.
func UpgradeActor(a *actor.Actor) error {
	if !a.Empty() {
		return errors.Errorf("Can't upgrade non-empty actor with code %s", a.Code)
	}
	a.Code = types.AccountActorCodeCid
	return nil
}

//
// ExecutableActor impl for Actor
//

// Ensure AccountActor is an ExecutableActor at compile time.
var _ dispatch.ExecutableActor = (*Actor)(nil)

// signatures are the publicly (externally callable) methods of the AccountActor.
var signatures = dispatch.Exports{}

// Method returns method definition for a given method id.
func (*Actor) Method(id types.MethodID) (dispatch.Method, *dispatch.FunctionSignature, bool) {
	return nil, nil, false
}

// InitializeState for account actors does nothing.
func (*Actor) InitializeState(_ runtime.Storage, _ interface{}) error {
	return nil
}

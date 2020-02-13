package account

import (
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/pattern"
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
func NewActor(balance abi.TokenAmount) (*actor.Actor, error) {
	return actor.NewActor(builtin.AccountActorCodeID, balance), nil
}

// State is the account actors storage.
type State struct {
	// Address is a public key based address that can be used to verify signatures
	Address address.Address
}

// NewState creates a new actor state.
func NewState(addr address.Address) *State {
	return &State{Address: addr}
}

// Actor methods
const (
	Constructor types.MethodID = types.ConstructorMethodID
)

//
// ExecutableActor impl for Actor
//

// Ensure AccountActor is an ExecutableActor at compile time.
var _ dispatch.Actor = (*Actor)(nil)

// Exports implements `dispatch.Actor`
func (a *Actor) Exports() []interface{} {
	return []interface{}{
		Constructor: (*Impl)(a).Constructor,
	}
}

// InitializeState for account actors does nothing.
func (*Actor) InitializeState(handle runtime.ActorStateHandle, initializerData interface{}) error {
	inputState, ok := initializerData.(*State)
	if !ok {
		return fmt.Errorf("Initial state to account actor is not a account.State struct")
	}

	if inputState.Address.Protocol() != address.SECP256K1 && inputState.Address.Protocol() != address.BLS {
		return fmt.Errorf("Attempt to create account actor with wrong type of address")
	}

	var state State
	_, err := handle.Transaction(&state, func() (interface{}, error) {
		// create id address
		state.Address = inputState.Address
		return nil, nil
	})
	if err != nil {
		return err
	}

	return nil
}

//
// vm methods for actor
//

// Impl is the VM implementation of the actor.
type Impl Actor

// Constructor initializes the actor's state
func (impl *Impl) Constructor(ctx runtime.InvocationContext, addr address.Address) {
	ctx.ValidateCaller(pattern.IsAInitActor{})

	err := (*Actor)(impl).InitializeState(ctx.State(), NewState(addr))
	if err != nil {
		panic(err)
	}
}

package vmcontext

import (
	"fmt"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/util/adt"

	"github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"

	validation "github.com/filecoin-project/chain-validation/chain/types"
	validationState "github.com/filecoin-project/chain-validation/state"
	notinit "github.com/filecoin-project/specs-actors/actors/builtin/init"
)

// ValidationVMWrapper abstracts the inspection and mutation of an implementation-specific state tree and storage.
// The interface wraps a single, mutable state.
type wrapper interface {
	// Returns the CID of the root node of the state tree.
	Root() cid.Cid
	// Returns the actor storage for the actor at `address` (which is empty if there is no such actor).
	Store() adt.Store
	// Returns the actor state at `address` (or an error if there is none).
	Actor(addr address.Address) (validationState.Actor, error)
	CreateActor(code cid.Cid, addr address.Address, balance abi.TokenAmount, state runtime.CBORMarshaler) (validationState.Actor, error)
	SetActorState(addr address.Address, balance abi.TokenAmount, state runtime.CBORMarshaler) (validationState.Actor, error)
}

type applier interface {
	ApplyMessage(context *validation.ExecutionContext, state wrapper, msg *validation.Message) (validation.MessageReceipt, error)
}

type actorWrapper struct {
	*actor.Actor
}

func (a *actorWrapper) Code() cid.Cid {
	return a.Actor.Code.Cid
}
func (a *actorWrapper) Head() cid.Cid {
	return a.Actor.Head.Cid
}
func (a *actorWrapper) CallSeqNum() int64 {
	return int64(a.Actor.CallSeqNum)
}
func (a *actorWrapper) Balance() abi.TokenAmount {
	return a.Actor.Balance
}

type validationVMWrapper struct {
	vm *VM
}

func (w *validationVMWrapper) PersistChanges() error {
	if err := w.vm.state.Commit(w.vm.context); err != nil {
		return err
	}
	if _, err := w.vm.state.Flush(w.vm.context); err != nil {
		return err
	}
	if err := w.vm.store.Flush(); err != nil {
		return err
	}
	return nil
}

var _ applier = (*validationVMWrapper)(nil)

func (w *validationVMWrapper) ApplyMessage(context *validation.ExecutionContext, state wrapper, msg *validation.Message) (validation.MessageReceipt, error) {
	// set epoch
	// Note: this would have normally happened during `ApplyTipset()`
	w.vm.currentEpoch = context.Epoch

	// map message
	// Dragons: fix after cleaning up our msg
	ourmsg := &types.UnsignedMessage{
		To:         msg.To,
		From:       msg.From,
		CallSeqNum: uint64(msg.CallSeqNum),
		Value:      msg.Value,
		Method:     msg.Method,
		Params:     msg.Params,
		GasPrice:   msg.GasPrice,
		GasLimit:   types.GasUnits(msg.GasLimit.Int64()),
	}

	// invoke vm
	ourreceipt, _, _ := w.vm.applyMessage(ourmsg, ourmsg.OnChainLen(), context.MinerOwner)

	// commit and persist changes
	// Note: this is not done on production for each msg
	if err := w.PersistChanges(); err != nil {
		return validation.MessageReceipt{}, err
	}

	// map receipt
	receipt := validation.MessageReceipt{
		ExitCode:    ourreceipt.ExitCode,
		ReturnValue: ourreceipt.ReturnValue,
		GasUsed:     big.Int(ourreceipt.GasUsed),
	}

	return receipt, nil
}

var _ wrapper = (*validationVMWrapper)(nil)

// Root implements ValidationVMWrapper.
func (w *validationVMWrapper) Root() cid.Cid {
	root, err := w.vm.state.Flush(w.vm.context)
	if err != nil {
		panic(err)
	}
	return root
}

// Store implements ValidationVMWrapper.
func (w *validationVMWrapper) Store() adt.Store {
	return w.vm.ContextStore()
}

// Actor implements ValidationVMWrapper.
func (w *validationVMWrapper) Actor(addr address.Address) (validationState.Actor, error) {
	// TODO: resolve address

	a, err := w.vm.state.GetActor(w.vm.context, addr)
	if err != nil {
		return nil, err
	}
	return &actorWrapper{a}, nil
}

// CreateActor implements ValidationVMWrapper.
func (w *validationVMWrapper) CreateActor(code cid.Cid, addr address.Address, balance abi.TokenAmount, state runtime.CBORMarshaler) (validationState.Actor, error) {
	idAddr := addr
	if addr.Protocol() != address.ID {
		// go through init to register
		initActorEntry, err := w.vm.state.GetActor(w.vm.context, builtin.InitActorAddr)

		// get a view into the actor state
		var state notinit.State
		if err := w.vm.store.Get(initActorEntry.Head.Cid, &state); err != nil {
			return nil, err
		}

		// add addr to inits map
		idAddr, err = state.MapAddressToNewID(w.vm.ContextStore(), addr)
		if err != nil {
			return nil, err
		}
	}

	// create actor on state stree
	a, _, err := w.vm.state.GetOrCreateActor(w.vm.context, idAddr, func() (*actor.Actor, address.Address, error) {
		return &actor.Actor{}, addr, nil
	})
	if err != nil {
		return nil, err
	}
	if !a.Empty() {
		return nil, fmt.Errorf("actor with address already exists")
	}

	// store state
	head, err := w.vm.store.Put(state)
	if err != nil {
		return nil, err
	}

	// update fields
	a.Code = enccid.NewCid(code)
	a.Head = enccid.NewCid(head)
	a.Balance = balance

	if err := w.PersistChanges(); err != nil {
		return nil, err
	}

	return &actorWrapper{a}, nil
}

// SetActorState implements ValidationVMWrapper.
func (w *validationVMWrapper) SetActorState(addr address.Address, balance abi.TokenAmount, headObj runtime.CBORMarshaler) (validationState.Actor, error) {
	idAddr, ok := w.vm.normalizeFrom(addr)
	if !ok {
		return nil, fmt.Errorf("actor not found")
	}

	a, err := w.vm.state.GetActor(w.vm.context, idAddr)
	if err != nil {
		return nil, err
	}
	// store state
	head, err := w.vm.store.Put(headObj)
	if err != nil {
		return nil, err
	}
	// update fields
	a.Head = enccid.NewCid(head)
	a.Balance = balance

	if err := w.PersistChanges(); err != nil {
		return nil, err
	}

	return &actorWrapper{a}, nil
}

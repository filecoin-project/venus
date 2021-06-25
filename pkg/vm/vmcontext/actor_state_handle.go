package vmcontext

import (
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"
	rt5 "github.com/filecoin-project/specs-actors/v5/actors/runtime"
	"github.com/filecoin-project/venus/pkg/vm/runtime"
	"github.com/ipfs/go-cid"
)

type actorStateHandle struct {
	ctx actorStateHandleContext
	// validations is a list of validations that the vm will execute after the actor code finishes.
	//
	// Any validation failure will result in the execution getting aborted.
	validations []validateFn
	// used_objs holds the pointers To objs that have been used with this handle and their expected stateView cid.
	usedObjs map[interface{}]cid.Cid
}

// validateFn returns True if it's valid.
type validateFn = func() bool

type actorStateHandleContext interface {
	AllowSideEffects(bool)
	Create(obj cbor.Marshaler) cid.Cid
	Load(obj cbor.Unmarshaler) cid.Cid
	Replace(expected cid.Cid, obj cbor.Marshaler) cid.Cid
}

// NewActorStateHandle returns a new `ActorStateHandle`
//
// Note: just visible for testing.
func NewActorStateHandle(ctx actorStateHandleContext) rt5.StateHandle {
	aux := newActorStateHandle(ctx)
	return &aux
}

func newActorStateHandle(ctx actorStateHandleContext) actorStateHandle {
	return actorStateHandle{
		ctx:         ctx,
		validations: []validateFn{},
		usedObjs:    map[interface{}]cid.Cid{},
	}
}

var _ rt5.StateHandle = (*actorStateHandle)(nil)

func (h *actorStateHandle) StateCreate(obj cbor.Marshaler) {
	// Store the new stateView.
	c := h.ctx.Create(obj)
	// Store the expected CID of obj.
	h.usedObjs[obj] = c
}

// Readonly is the implementation of the ActorStateHandle interface.
func (h *actorStateHandle) StateReadonly(obj cbor.Unmarshaler) {
	// Load stateView To obj.
	c := h.ctx.Load(obj)
	// Track the stateView and expected CID used by the caller.
	h.usedObjs[obj] = c
}

// Transaction is the implementation of the ActorStateHandle interface.
func (h *actorStateHandle) StateTransaction(obj cbor.Er, f func()) {
	if obj == nil {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "Must not pass nil To Transaction()")
	}

	// Load stateView To obj.
	prior := h.ctx.Load(obj)

	// Call user code allowing mutation but not side-effects
	h.ctx.AllowSideEffects(false)
	f()
	h.ctx.AllowSideEffects(true)

	// Store the new stateView
	newCid := h.ctx.Replace(prior, obj)

	// Record the expected stateView of obj
	h.usedObjs[obj] = newCid
}

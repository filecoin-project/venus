package vmcontext

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/ipfs/go-cid"
)

type actorStateHandle struct {
	ctx  actorStateHandleContext
	head cid.Cid
	// validations is a list of validations that the vm will execute after the actor code finishes.
	//
	// Any validation failure will result in the execution getting aborted.
	validations []validateFn
	// used_objs holds the pointers to objs that have been used with this handle and their expected state cid.
	usedObjs map[interface{}]cid.Cid
}

// validateFn returns True if it's valid.
type validateFn = func() bool

type actorStateHandleContext interface {
	Store() specsruntime.Store
	AllowSideEffects(bool)
}

type readonlyContextWrapper struct {
	store specsruntime.Store
}

func (w readonlyContextWrapper) Store() specsruntime.Store {
	return w.store
}

func (readonlyContextWrapper) AllowSideEffects(bool) {}

// NewActorStateHandle returns a new `ActorStateHandle`
//
// Note: just visible for testing.
func NewActorStateHandle(ctx actorStateHandleContext, head cid.Cid) specsruntime.StateHandle {
	aux := newActorStateHandle(ctx, head)
	return &aux
}

// NewReadonlyStateHandle returns a new reaadonly `ActorStateHandle`
func NewReadonlyStateHandle(store specsruntime.Store, head cid.Cid) specsruntime.StateHandle {
	aux := newActorStateHandle(readonlyContextWrapper{store: store}, head)
	return &aux
}

func newActorStateHandle(ctx actorStateHandleContext, head cid.Cid) actorStateHandle {
	return actorStateHandle{
		ctx:         ctx,
		head:        head,
		validations: []validateFn{},
		usedObjs:    map[interface{}]cid.Cid{},
	}
}

var _ specsruntime.StateHandle = (*actorStateHandle)(nil)

func (h *actorStateHandle) Create(obj specsruntime.CBORMarshaler) {
	if h.head != cid.Undef {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "Actor state already initialized")
	}

	// store state
	h.head = h.ctx.Store().Put(obj)

	// update internal ref to state
	h.usedObjs[obj] = h.head
}

// Readonly is the implementation of the ActorStateHandle interface.
func (h *actorStateHandle) Readonly(obj specsruntime.CBORUnmarshaler) {
	// load state from storage
	// Note: we copy the head over to `readonlyHead` in case it gets modified afterwards via `Transaction()`.
	readonlyHead := h.head

	if readonlyHead == cid.Undef {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "Nil state can not be accessed via Readonly(), use Transaction() instead")
	}

	// track the variable used by the caller
	h.usedObjs[obj] = h.head

	// load it to obj
	h.ctx.Store().Get(readonlyHead, obj)
}

// Transaction is the implementation of the ActorStateHandle interface.
func (h *actorStateHandle) Transaction(obj specsruntime.CBORer, f func() interface{}) interface{} {
	if obj == nil {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "Must not pass nil to Transaction()")
	}

	oldcid := h.head

	// load state only if it already exists
	if h.head != cid.Undef {
		h.ctx.Store().Get(oldcid, obj)
	}

	// call user code allowing mutation but not side-effects
	h.ctx.AllowSideEffects(false)
	out := f()
	h.ctx.AllowSideEffects(true)

	// store the new state
	newcid := h.ctx.Store().Put(obj)

	// update head
	h.head = newcid

	// track the variable used by the caller
	h.usedObjs[obj] = h.head

	return out
}

// Validate validates that the state was mutated properly.
//
// This method is not part of the public API,
// it is expected to be called by the runtime after each actor method.
func (h *actorStateHandle) Validate(cidFn func(interface{}) cid.Cid) {
	for obj, head := range h.usedObjs {
		// verify the obj has not changed
		usedCid := cidFn(obj)
		if usedCid != head {
			runtime.Abortf(exitcode.SysErrorIllegalActor, "State mutated outside of Transaction() scope")
		}
	}
}

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
	// used_obj holds the pointer to the obj that has been used with this handle.
	//
	// any subsequent calls needs to be using the same variable.
	usedObj interface{}
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
	h.usedObj = obj
}

// Readonly is the implementation of the ActorStateHandle interface.
func (h *actorStateHandle) Readonly(obj specsruntime.CBORUnmarshaler) {
	// track the variable used by the caller
	if h.usedObj == nil {
		h.usedObj = obj
	} else if h.usedObj != obj {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "Must use the same state variable on repeated calls")
	}

	// load state from storage
	// Note: we copy the head over to `readonlyHead` in case it gets modified afterwards via `Transaction()`.
	readonlyHead := h.head

	if readonlyHead == cid.Undef {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "Nil state can not be accessed via Readonly(), use Transaction() instead")
	}

	// load it to obj
	h.ctx.Store().Get(readonlyHead, obj)
}

// Transaction is the implementation of the ActorStateHandle interface.
func (h *actorStateHandle) Transaction(obj specsruntime.CBORer, f func() interface{}) interface{} {
	if obj == nil {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "Must not pass nil to Transaction()")
	}

	// track the variable used by the caller
	if h.usedObj == nil {
		h.usedObj = obj
	} else if h.usedObj != obj {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "Must use the same state variable on repeated calls")
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

	return out
}

// Validate validates that the state was mutated properly.
//
// This method is not part of the public API,
// it is expected to be called by the runtime after each actor method.
func (h *actorStateHandle) Validate(cidFn func(interface{}) cid.Cid) {
	if h.usedObj != nil {
		// verify the obj has not changed
		usedCid := cidFn(h.usedObj)
		if usedCid != h.head {
			runtime.Abortf(exitcode.SysErrorIllegalActor, "State mutated outside of Transaction() scope")
		}
	}
}

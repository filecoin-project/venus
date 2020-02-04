package vmcontext

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/exitcode"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
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
	Storage() runtime.Storage
	AllowSideEffects(bool)
}

type readonlyContextWrapper struct {
	store runtime.Storage
}

func (w readonlyContextWrapper) Storage() runtime.Storage {
	return w.store
}

func (readonlyContextWrapper) AllowSideEffects(bool) {}

// NewActorStateHandle returns a new `ActorStateHandle`
//
// Note: just visible for testing.
func NewActorStateHandle(ctx actorStateHandleContext, head cid.Cid) runtime.ActorStateHandle {
	aux := newActorStateHandle(ctx, head)
	return &aux
}

// NewReadonlyStateHandle returns a new `ReadonlyActorStateHandle`
func NewReadonlyStateHandle(store runtime.Storage, head cid.Cid) runtime.ReadonlyActorStateHandle {
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

var _ runtime.ActorStateHandle = (*actorStateHandle)(nil)

func (h *actorStateHandle) Create(obj interface{}) {
	if h.head != cid.Undef {
		runtime.Abortf(exitcode.MethodAbort, "Actor state already initialized")
	}

	// store state
	h.head = h.ctx.Storage().Put(obj)

	// update internal ref to state
	h.usedObj = obj
}

// Readonly is the implementation of the ActorStateHandle interface.
func (h *actorStateHandle) Readonly(obj interface{}) {
	// track the variable used by the caller
	if h.usedObj == nil {
		h.usedObj = obj
	} else if h.usedObj != obj {
		runtime.Abortf(exitcode.MethodAbort, "Must use the same state variable on repeated calls")
	}

	// Dragons: needed while we can get actor state modified directly by actor code
	// h.head = h.ctx.LegacyStorage().LegacyHead()

	// load state from storage
	// Note: we copy the head over to `readonlyHead` in case it gets modified afterwards via `Transaction()`.
	readonlyHead := h.head

	if readonlyHead == cid.Undef {
		runtime.Abortf(exitcode.MethodAbort, "Nil state can not be accessed via Readonly(), use Transaction() instead")
	}

	h.ctx.Storage().Get(readonlyHead, obj)
}

type transactionFn = func() (interface{}, error)

// Transaction is the implementation of the ActorStateHandle interface.
func (h *actorStateHandle) Transaction(obj interface{}, f transactionFn) (interface{}, error) {
	if obj == nil {
		runtime.Abortf(exitcode.MethodAbort, "Must not pass nil to Transaction()")
	}

	// track the variable used by the caller
	if h.usedObj == nil {
		h.usedObj = obj
	} else if h.usedObj != obj {
		runtime.Abortf(exitcode.MethodAbort, "Must use the same state variable on repeated calls")
	}

	// Dragons: needed while we can get actor state modified directly by actor code
	// h.head = h.ctx.LegacyStorage().LegacyHead()

	oldcid := h.head

	// load state only if it already exists
	if h.head != cid.Undef {
		h.ctx.Storage().Get(oldcid, obj)
	}

	// call user code allowing mutation but not side-effects
	h.ctx.AllowSideEffects(false)
	out, err := f()
	h.ctx.AllowSideEffects(true)

	// forward user error
	if err != nil {
		if h.head != cid.Undef {
			// re-load state from storage
			// Note: this will throw away any changes done to the object and replace it with Readonly()
			h.ctx.Storage().Get(oldcid, obj)
		}

		return out, err
	}

	// store the new state
	newcid := h.ctx.Storage().Put(obj)

	// update head
	h.head = newcid

	return out, nil
}

// Validate validates that the state was mutated properly.
//
// This method is not part of the public API,
// it is expected to be called by the runtime after each actor method.
func (h *actorStateHandle) Validate() {
	if h.usedObj != nil {
		// verify the obj has not changed
		usedCid := h.ctx.Storage().CidOf(h.usedObj)
		if usedCid != h.head {
			runtime.Abortf(exitcode.MethodAbort, "State mutated outside of Transaction() scope")
		}
	}
}

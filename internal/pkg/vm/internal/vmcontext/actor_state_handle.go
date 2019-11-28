package vmcontext

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
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

// NewActorStateHandle returns a new `actorStateHandle`
//
// Note: just visible for testing.
func NewActorStateHandle(ctx actorStateHandleContext, head cid.Cid) runtime.ActorStateHandle {
	aux := newActorStateHandle(ctx, head)
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

// Readonly is the implementation of the ActorStateHandle interface.
func (h *actorStateHandle) Readonly(obj interface{}) {
	// track the variable used by the caller
	if h.usedObj == nil {
		h.usedObj = obj
	} else if h.usedObj != obj {
		runtime.Abort("Must use the same state variable on repeated calls")
	}

	// load state from storage
	// Note: we copy the head over to `readonlyHead` in case it gets modified afterwards via `Transaction()`.
	readonlyHead := h.head

	if readonlyHead == cid.Undef {
		runtime.Abort("Nil state can not be accessed via Readonly(), use Transaction() instead")
	}

	h.get(readonlyHead, obj)
}

type transactionFn = func() (interface{}, error)

// Transaction is the implementation of the ActorStateHandle interface.
func (h *actorStateHandle) Transaction(obj interface{}, f transactionFn) (interface{}, error) {
	if obj == nil {
		runtime.Abort("Must not pass nil to Transaction()")
	}

	// track the variable used by the caller
	if h.usedObj == nil {
		h.usedObj = obj
	} else if h.usedObj != obj {
		runtime.Abort("Must use the same state variable on repeated calls")
	}

	// load state from storage
	oldcid := h.head
	h.get(oldcid, obj)

	// call user code allowing mutation but not side-effects
	h.ctx.AllowSideEffects(false)
	out, err := f()
	h.ctx.AllowSideEffects(true)

	// forward user error
	if err != nil {
		// re-load state from storage
		// Note: this will throw away any changes done to the object and replace it with Readonly()
		h.get(oldcid, obj)

		return out, err
	}

	// store the new state
	storage := h.ctx.Storage()
	newcid, err := storage.Put(obj)
	if err != nil {
		runtime.Abort("Storage put error")
	}

	// commit the new state
	// Note: this is more of a commit into pending, not a real commit
	err = storage.Commit(newcid, oldcid)
	if err != nil {
		runtime.Abort("Storage commit error")
	}

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
		usedCid, err := h.ctx.Storage().CidOf(h.usedObj)
		if err != nil || usedCid != h.head {
			runtime.Abort("State mutated outside of Transaction() scope")
		}
	}
}

// Dragons: cleanup after changing `storage.Get` to `Get(cid, interface{})`
func (h *actorStateHandle) get(cid cid.Cid, obj interface{}) {
	// load state from storage
	storage := h.ctx.Storage()
	rawstate, err := storage.Get(cid)
	if err != nil {
		runtime.Abort("Storage get error")
	}

	if err := encoding.Decode(rawstate, obj); err != nil {
		runtime.Abort("Could not deserialize state")
	}
}

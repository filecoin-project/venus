package vmcontext

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/ipfs/go-cid"
)

type actorStateHandle struct {
	ctx  actorStateHandleContext
	head *cid.Cid
	// validations is a list of validations that the vm will execute after the actor code finishes.
	//
	// Any validation failure will result in the execution getting aborted.
	validations []validateFn
}

type validateFn = func() bool

type actorStateHandleContext interface {
	Storage() runtime.Storage
	AllowSideEffects(bool)
}

// NewActorStateHandle returns a new `actorStateHandle`
func NewActorStateHandle(ctx actorStateHandleContext, head cid.Cid) runtime.ActorStateHandle {
	aux := newActorStateHandle(ctx, head)
	return &aux
}

func newActorStateHandle(ctx actorStateHandleContext, head cid.Cid) actorStateHandle {
	return actorStateHandle{
		ctx:         ctx,
		head:        &head,
		validations: []validateFn{},
	}
}

var _ runtime.ActorStateHandle = (*actorStateHandle)(nil)

// Readonly loads a readonly copy of the state into the argument.
//
// Any modification to the state is illegal and will result in an `Abort`.
func (h *actorStateHandle) Readonly(obj interface{}) {
	// load state from storage
	// Note: we copy the head over to `readonlyHead` in case it gets modified afterwards via `Transaction()`.
	readonlyHead := *h.head

	if readonlyHead == cid.Undef {
		runtime.Abort("Nil state can not be accessed via Readonly(), use Transaction() instead")
	}

	h.get(readonlyHead, obj)

	// add validation
	h.addCidValidation(obj, readonlyHead)
}

type transactionFn = func() (interface{}, error)

// Transaction loads a mutable version of the state into the `obj` argument and protects
// the execution from side effects.
//
// The second argument is a function which allows the caller to mutate the state.
//
// The new state will be commited if there are no errors returned.
// Note: if an error is returned, the state changes will be DISCARDED and the reference will revert back.
//
// WARNING: If the state is modified AFTER the function returns, the execution will Abort.
//	        The state is mutable ONLY inside the lambda.
//
// Transaction can be thought of as having the following signature:
//
// `Transaction(F) -> (T, Error) where F: Fn(S) -> (T, error), S: ActorState`.
//
// Note: the actual Go signature is a bit different due to the lack of type system magic,
//       and also wanting to avoid some unnecesary reflection.
//
// Review: we might want to spend an hour or four making the signature look like it's supposed to..
// Hack: In order to know `S` and save some code, the actual signature looks like:
//       `Transaction(S, F) where S: ActorState, F: Fn() -> (T, Error)`.
//
// # Usage
//
// ```go
// var state SomeState
// ret, err := ctx.StateHandke().Transaction(&state, func() (interface{}, error) {
//   // make some changes
//	 st.ImLoaded = True
//   return st.Thing, nil
// })
// // state.ImLoaded = False // BAD!! state is readonly outside the lambda
// ```
func (h *actorStateHandle) Transaction(obj interface{}, f transactionFn) (interface{}, error) {
	// load state from storage
	oldcid := *h.head
	h.get(oldcid, obj)

	// call user code allowing mutation but not side-effects
	h.ctx.AllowSideEffects(false)
	out, err := f()
	h.ctx.AllowSideEffects(true)

	// forward user error
	if err != nil {
		// re-load state from storage
		// Note: this will throw away any changes done to the object
		h.get(oldcid, obj)

		h.addCidValidation(obj, oldcid)
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
	h.head = &newcid

	// add validation
	h.addCidValidation(obj, newcid)

	return out, nil
}

func (h *actorStateHandle) addCidValidation(obj interface{}, expected cid.Cid) {
	// checks that the cid of the obj has not changed
	h.validations = append(h.validations, func() bool {
		return h.cidOf(obj) == expected
	})
}

// Validate validates that the state was mutated properly.
//
// This method is not part of the public API,
// it is expected to be called by the runtime after each actor method.
func (h *actorStateHandle) Validate() {
	for _, vfn := range h.validations {
		if !vfn() {
			runtime.Abort("State mutated outside of Transaction() scope")
		}
	}
}

// Dragons: move this to the storage API
func (h *actorStateHandle) cidOf(obj interface{}) cid.Cid {
	// get Cid for object
	// Dragons: this should not be storing
	storage := h.ctx.Storage()
	newcid, err := storage.Put(obj)
	if err != nil {
		runtime.Abort("Storage put error")
	}

	return newcid
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

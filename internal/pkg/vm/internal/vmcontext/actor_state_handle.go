package vmcontext

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/ipfs/go-cid"
)

type actorStateHandle struct {
	ctx  actorStateHandleContext
	head *cid.Cid
}

type actorStateHandleContext interface {
	Storage() runtime.Storage
}

// NewActorStateHandle returns a new `actorStateHandle`
func NewActorStateHandle(ctx actorStateHandleContext, head cid.Cid) runtime.ActorStateHandle {
	return &actorStateHandle{
		ctx:  ctx,
		head: &head,
	}
}

var _ runtime.ActorStateHandle = (*actorStateHandle)(nil)

// Take loads the state and locks it.
//
// Any futre calls to `Take` on this actors state will `abort` the execution.
func (h *actorStateHandle) Take(obj interface{}) {
	// check if the state has already been taken
	if h.head == nil {
		runtime.Abort("Must call Take() only once on actor substate object")
	}

	// load state from storage
	storage := h.ctx.Storage()
	rawstate, err := storage.Get(*h.head)
	if err != nil {
		runtime.Abort("Storage get error")
	}

	// Dragons: cleanup after changing `Get` to `Get(cid, interface{})`
	if err := encoding.Decode(rawstate, obj); err != nil {
		runtime.Abort("Could not deserialize state")
	}

	// clear head value so that it cannot be taken again
	h.head = nil
}

// UpdateRelease updates the actor state and releases the lock on it.
//
// No future calls to `Take` are allowed on this object.
func (h *actorStateHandle) UpdateRelease(obj interface{}) {
	// Review: for @spec why are all this checks here?
	// rt._checkRunning()
	// rt._checkActorStateAcquired()
	if h.head != nil {
		runtime.Abort("Must call Take() before calling UpdateRelease()")
	}

	storage := h.ctx.Storage()
	oldcid := storage.Head()

	// store the new state
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

	// make new head available to take
	h.head = &newcid
}

// Release asserts that the state has not changed and releases the lock on it.
//
// No future calls to `Take` are allowed on this object.
func (h *actorStateHandle) Release(obj interface{}) {
	// Review: for @spec why are all this checks here?
	// rt._checkRunning()
	// rt._checkActorStateAcquired()
	if h.head != nil {
		runtime.Abort("Must call Take() before calling Release()")
	}

	storage := h.ctx.Storage()
	oldcid := storage.Head()

	// store the new state
	// Dragons: we need a `CidOf(interface{})` on the storage that doesnt actually store anything
	newcid, err := storage.Put(obj)
	if err != nil {
		runtime.Abort("Storage put error")
	}

	if newcid != oldcid {
		runtime.Abort("State CID differs upon release call")
	}

	// make new head available to take
	h.head = &newcid
}

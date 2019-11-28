package vmcontext_test

import (
	"errors"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/vmcontext"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
)

func init() {
	encoding.RegisterIpldCborType(testActorStateHandleState{})
}

type testActorStateHandleState struct {
	FieldA string
}

func setup() testSetup {
	initialstate := testActorStateHandleState{FieldA: "fakestate"}

	ctx := fakeActorStateHandleContext{
		storage:          vm.NewTestStorage(initialstate),
		allowSideEffects: true,
	}
	initialhead := ctx.storage.Head()
	h := vmcontext.NewActorStateHandle(&ctx, initialhead)

	cleanup := func() {
		// the vmcontext is supposed to call validate after each actor method
		implH := h.(extendedStateHandle)
		implH.Validate()
	}

	return testSetup{
		initialstate: initialstate,
		ctx:          ctx,
		initialhead:  initialhead,
		h:            h,
		cleanup:      cleanup,
	}
}

func TestActorStateHandle(t *testing.T) {
	tf.UnitTest(t)

	t.Run("readonly", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState
		ts.h.Readonly(&out)

		assert.Equal(t, out, ts.initialstate)
	})

	t.Run("abort on mutating a readonly", func(t *testing.T) {
		defer mustPanic(t)

		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState
		ts.h.Readonly(&out)

		out.FieldA = "changed!"
	})

	t.Run("readonly multiple times", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState
		ts.h.Readonly(&out)
		ts.h.Readonly(&out)

		assert.Equal(t, out, ts.initialstate)
	})

	t.Run("transaction", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState
		expected := "new state"

		_, err := ts.h.Transaction(&out, func() (interface{}, error) {
			// check state is not what we are going to use
			assert.NotEqual(t, out.FieldA, expected)
			out.FieldA = expected

			return nil, nil
		})
		assert.NoError(t, err)
		// check that it changed
		assert.Equal(t, out.FieldA, expected)

		var out2 testActorStateHandleState
		ts.h.Readonly(&out2)
		// really check with a new object
		assert.Equal(t, out2.FieldA, expected)
	})

	t.Run("transaction but no mutation", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState

		// should work, mutating is not compulsory
		_, err := ts.h.Transaction(&out, func() (interface{}, error) {
			return nil, nil
		})
		assert.NoError(t, err)

		assert.Equal(t, out, ts.initialstate)
	})

	t.Run("transaction returning error", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState

		_, err := ts.h.Transaction(&out, func() (interface{}, error) {
			out.FieldA = "changed!"
			return nil, errors.New("some error")
		})
		assert.Error(t, err)
		// check that it did NOT change
		assert.Equal(t, out, ts.initialstate)

		var out2 testActorStateHandleState
		ts.h.Readonly(&out2)
		// really check with a new object
		assert.Equal(t, out2, ts.initialstate)
	})

	t.Run("transaction returning value", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState

		v, err := ts.h.Transaction(&out, func() (interface{}, error) {
			return out.FieldA, nil
		})
		assert.NoError(t, err)

		assert.Equal(t, v, ts.initialstate.FieldA)
	})

	t.Run("mutated after the transaction", func(t *testing.T) {
		defer mustPanic(t)

		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState

		_, err := ts.h.Transaction(&out, func() (interface{}, error) {
			out.FieldA = "changed!"
			return nil, nil
		})
		assert.NoError(t, err)

		out.FieldA = "changed again!"
	})
}

func TestActorStateHandleNilState(t *testing.T) {
	tf.UnitTest(t)

	setup := func() (runtime.ActorStateHandle, func()) {
		ctx := fakeActorStateHandleContext{
			storage:          vm.NewTestStorage(nil),
			allowSideEffects: true,
		}
		initialhead := ctx.storage.Head()
		h := vmcontext.NewActorStateHandle(&ctx, initialhead)

		cleanup := func() {
			// the vmcontext is supposed to call validate after each actor method
			implH := h.(extendedStateHandle)
			implH.Validate()
		}

		return h, cleanup
	}

	t.Run("readonly on nil state is not allowed", func(t *testing.T) {
		defer mustPanic(t)

		h, cleanup := setup()
		defer cleanup()

		var out testActorStateHandleState
		h.Readonly(&out)
	})

	t.Run("transaction on nil state", func(t *testing.T) {
		h, cleanup := setup()
		defer cleanup()

		var out testActorStateHandleState
		_, err := h.Transaction(&out, func() (interface{}, error) {
			return nil, nil
		})
		assert.NoError(t, err)
	})

	t.Run("state initialized after transaction", func(t *testing.T) {
		h, cleanup := setup()
		defer cleanup()

		var out testActorStateHandleState
		_, err := h.Transaction(&out, func() (interface{}, error) {
			return nil, nil
		})
		assert.NoError(t, err)

		var out2 testActorStateHandleState
		h.Readonly(&out2) // should not fail
	})
}

type extendedStateHandle interface {
	Validate()
}

type fakeActorStateHandleContext struct {
	storage          runtime.Storage
	allowSideEffects bool
}

func (ctx *fakeActorStateHandleContext) Storage() runtime.Storage {
	return ctx.storage
}

func (ctx *fakeActorStateHandleContext) AllowSideEffects(allow bool) {
	ctx.allowSideEffects = allow
}

type testSetup struct {
	initialstate testActorStateHandleState
	ctx          fakeActorStateHandleContext
	initialhead  cid.Cid
	h            runtime.ActorStateHandle
	cleanup      func()
}

func mustPanic(t *testing.T) {
	if r := recover(); r == nil {
		t.Fail()
	}
}

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

	storage := vm.NewTestStorage(initialstate)
	ctx := fakeActorStateHandleContext{
		storage:          storage,
		allowSideEffects: true,
	}
	initialhead := storage.CidOf(initialstate)
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

	// this test case verifies that the `Validate` works when nothing was done with the state
	t.Run("noop", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()
	})

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

	t.Run("readonly promotion", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState
		ts.h.Readonly(&out)

		_, err := ts.h.Transaction(&out, func() (interface{}, error) {
			out.FieldA = "changed!"
			return nil, nil
		})
		assert.NoError(t, err)
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

		ts.h.Readonly(&out)
		// really check by loading it again
		assert.Equal(t, out.FieldA, expected)
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

		ts.h.Readonly(&out)
		// really check by loading it again
		assert.Equal(t, out, ts.initialstate)
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

	t.Run("transaction double whammy", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState

		_, err := ts.h.Transaction(&out, func() (interface{}, error) {
			out.FieldA = "changed!"
			return nil, nil
		})
		assert.NoError(t, err)

		v, err := ts.h.Transaction(&out, func() (interface{}, error) {
			out.FieldA = "again!"
			return out.FieldA, nil
		})
		assert.NoError(t, err)

		ts.h.Readonly(&out)
		// really check by loading it again
		assert.Equal(t, out.FieldA, v)
	})
}

func TestActorStateHandleNilState(t *testing.T) {
	tf.UnitTest(t)

	setup := func() (runtime.ActorStateHandle, func()) {
		storage := vm.NewTestStorage(nil)
		ctx := fakeActorStateHandleContext{
			storage:          storage,
			allowSideEffects: true,
		}

		h := vmcontext.NewActorStateHandle(&ctx, cid.Undef)

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

		h.Readonly(&out) // should not fail
	})

	t.Run("readonly nil pointer to state", func(t *testing.T) {
		defer mustPanic(t)

		h, cleanup := setup()
		defer cleanup()

		h.Readonly(nil)
	})

	t.Run("transaction nil pointer to state", func(t *testing.T) {
		defer mustPanic(t)

		h, cleanup := setup()
		defer cleanup()

		_, err := h.Transaction(nil, func() (interface{}, error) {
			return nil, nil
		})
		assert.NoError(t, err)
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

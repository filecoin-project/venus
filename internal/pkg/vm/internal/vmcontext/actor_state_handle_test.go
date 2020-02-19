package vmcontext_test

import (
	"io"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/vmcontext"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
)

func init() {
	encoding.RegisterIpldCborType(testActorStateHandleState{})
}

type testActorStateHandleState struct {
	FieldA string
}

func (t *testActorStateHandleState) MarshalCBOR(w io.Writer) error {
	aux, err := encoding.Encode(t.FieldA)
	if err != nil {
		return err
	}
	if _, err := w.Write(aux); err != nil {
		return err
	}
	return nil
}

func (t *testActorStateHandleState) UnmarshalCBOR(r io.Reader) error {
	bs := make([]byte, 1024)
	n, err := r.Read(bs)
	if err != nil {
		return err
	}
	if err := encoding.Decode(bs[:n], &t.FieldA); err != nil {
		return err
	}
	return nil
}

func setup() testSetup {
	initialstate := testActorStateHandleState{FieldA: "fakestate"}

	store := vm.NewTestStorage(&initialstate)
	ctx := fakeActorStateHandleContext{
		store:            store,
		allowSideEffects: true,
	}
	initialhead := store.CidOf(&initialstate)
	h := vmcontext.NewActorStateHandle(&ctx, initialhead)

	cleanup := func() {
		// the vmcontext is supposed to call validate after each actor method
		implH := h.(extendedStateHandle)
		implH.Validate(func(obj interface{}) cid.Cid { return store.CidOf(obj) })
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

		ts.h.Transaction(&out, func() interface{} {
			out.FieldA = "changed!"
			return nil
		})
	})

	t.Run("transaction", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState
		expected := "new state"

		ts.h.Transaction(&out, func() interface{} {
			// check state is not what we are going to use
			assert.NotEqual(t, out.FieldA, expected)
			out.FieldA = expected

			return nil
		})
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
		ts.h.Transaction(&out, func() interface{} {
			return nil
		})

		assert.Equal(t, out, ts.initialstate)
	})

	t.Run("transaction returning value", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState

		v := ts.h.Transaction(&out, func() interface{} {
			return out.FieldA
		})

		assert.Equal(t, v, ts.initialstate.FieldA)
	})

	t.Run("mutated after the transaction", func(t *testing.T) {
		defer mustPanic(t)

		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState

		ts.h.Transaction(&out, func() interface{} {
			out.FieldA = "changed!"
			return nil
		})

		out.FieldA = "changed again!"
	})

	t.Run("transaction double whammy", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState

		ts.h.Transaction(&out, func() interface{} {
			out.FieldA = "changed!"
			return nil
		})

		v := ts.h.Transaction(&out, func() interface{} {
			out.FieldA = "again!"
			return out.FieldA
		})

		ts.h.Readonly(&out)
		// really check by loading it again
		assert.Equal(t, out.FieldA, v)
	})
}

func TestActorStateHandleNilState(t *testing.T) {
	tf.UnitTest(t)

	setup := func() (runtime.StateHandle, func()) {
		store := vm.NewTestStorage(nil)
		ctx := fakeActorStateHandleContext{
			store:            store,
			allowSideEffects: true,
		}

		h := vmcontext.NewActorStateHandle(&ctx, cid.Undef)

		cleanup := func() {
			// the vmcontext is supposed to call validate after each actor method
			implH := h.(extendedStateHandle)
			implH.Validate(func(obj interface{}) cid.Cid { return store.CidOf(obj) })
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
		h.Transaction(&out, func() interface{} {
			return nil
		})
	})

	t.Run("state initialized after transaction", func(t *testing.T) {
		h, cleanup := setup()
		defer cleanup()

		var out testActorStateHandleState
		h.Transaction(&out, func() interface{} {
			return nil
		})

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

		h.Transaction(nil, func() interface{} {
			return nil
		})
	})
}

type extendedStateHandle interface {
	Validate(func(interface{}) cid.Cid)
}

type fakeActorStateHandleContext struct {
	store            runtime.Store
	allowSideEffects bool
}

func (ctx *fakeActorStateHandleContext) Store() runtime.Store {
	return ctx.store
}

func (ctx *fakeActorStateHandleContext) AllowSideEffects(allow bool) {
	ctx.allowSideEffects = allow
}

type testSetup struct {
	initialstate testActorStateHandleState
	ctx          fakeActorStateHandleContext
	initialhead  cid.Cid
	h            runtime.StateHandle
	cleanup      func()
}

func mustPanic(t *testing.T) {
	if r := recover(); r == nil {
		t.Fail()
	}
}

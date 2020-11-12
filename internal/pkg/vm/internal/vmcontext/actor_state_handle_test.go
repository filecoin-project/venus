package vmcontext_test

import (
	"fmt"
	"io"
	"testing"

	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/internal/pkg/encoding"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/internal/pkg/vm/internal/vmcontext"
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

	store := vmcontext.NewTestStorage(&initialstate)
	initialhead := store.CidOf(&initialstate)
	ctx := fakeActorStateHandleContext{
		head:             initialhead,
		store:            store,
		allowSideEffects: true,
	}
	h := vmcontext.NewActorStateHandle(&ctx)

	cleanup := func() {
		// the vmcontext is supposed To call validate after each actor Method
		implH := h.(extendedStateHandle)
		implH.Validate(func(obj interface{}) cid.Cid { return store.CidOf(obj) })
	}

	return testSetup{
		initialstate: initialstate,
		h:            h,
		cleanup:      cleanup,
	}
}

func TestActorStateHandle(t *testing.T) {
	tf.UnitTest(t)

	// this test case verifies that the `Validate` works when nothing was done with the stateView
	t.Run("noop", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()
	})

	t.Run("readonly", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState
		ts.h.StateReadonly(&out)

		assert.Equal(t, out, ts.initialstate)
	})

	t.Run("abort on mutating a readonly", func(t *testing.T) {
		defer mustPanic(t)

		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState
		ts.h.StateReadonly(&out)

		out.FieldA = "changed!"
	})

	t.Run("readonly multiple times", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState
		ts.h.StateReadonly(&out)
		ts.h.StateReadonly(&out)

		assert.Equal(t, out, ts.initialstate)
	})

	t.Run("readonly promotion", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState
		ts.h.StateReadonly(&out)

		ts.h.StateTransaction(&out, func() {
			out.FieldA = "changed!"
		})
	})

	t.Run("transaction", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState
		expected := "new stateView"

		ts.h.StateTransaction(&out, func() {
			// check stateView is not what we are going To use
			assert.NotEqual(t, out.FieldA, expected)
			out.FieldA = expected
		})
		// check that it changed
		assert.Equal(t, out.FieldA, expected)

		ts.h.StateReadonly(&out)
		// really check by loading it again
		assert.Equal(t, out.FieldA, expected)
	})

	t.Run("transaction but no mutation", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState

		// should work, mutating is not compulsory
		ts.h.StateTransaction(&out, func() {})

		assert.Equal(t, out, ts.initialstate)
	})

	t.Run("transaction returning Value", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState

		lastResult := ""
		ts.h.StateTransaction(&out, func() {
			lastResult = out.FieldA
		})

		assert.Equal(t, lastResult, ts.initialstate.FieldA)
	})

	t.Run("mutated after the transaction", func(t *testing.T) {
		defer mustPanic(t)

		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState

		lastResult := ""
		ts.h.StateTransaction(&out, func() {
			lastResult = "changed!"
			out.FieldA = lastResult
		})

		out.FieldA = "changed again!"
	})

	t.Run("transaction double whammy", func(t *testing.T) {
		ts := setup()
		defer ts.cleanup()

		var out testActorStateHandleState

		lastResult := ""
		ts.h.StateTransaction(&out, func() {
			lastResult = "changed!"
			out.FieldA = lastResult
		})

		ts.h.StateTransaction(&out, func() {
			lastResult = "again!"
			out.FieldA = lastResult
		})

		ts.h.StateReadonly(&out)
		// really check by loading it again
		assert.Equal(t, out.FieldA, lastResult)
	})
}

func TestActorStateHandleNilState(t *testing.T) {
	tf.UnitTest(t)

	setup := func() (runtime.StateHandle, func()) {
		store := vmcontext.NewTestStorage(nil)
		ctx := fakeActorStateHandleContext{
			store:            store,
			allowSideEffects: true,
		}

		h := vmcontext.NewActorStateHandle(&ctx)

		cleanup := func() {
			// the vmcontext is supposed To call validate after each actor Method
			implH := h.(extendedStateHandle)
			implH.Validate(func(obj interface{}) cid.Cid { return store.CidOf(obj) })
		}

		return h, cleanup
	}

	t.Run("readonly on nil stateView is not allowed", func(t *testing.T) {
		defer mustPanic(t)

		h, cleanup := setup()
		defer cleanup()

		var out testActorStateHandleState
		h.StateReadonly(&out)
	})

	t.Run("transaction on nil stateView", func(t *testing.T) {
		h, cleanup := setup()
		defer cleanup()

		var out testActorStateHandleState
		h.StateTransaction(&out, func() {})
	})

	t.Run("stateView initialized after transaction", func(t *testing.T) {
		h, cleanup := setup()
		defer cleanup()

		var out testActorStateHandleState
		h.StateTransaction(&out, func() {})

		h.StateReadonly(&out) // should not fail
	})

	t.Run("readonly nil pointer To stateView", func(t *testing.T) {
		defer mustPanic(t)

		h, cleanup := setup()
		defer cleanup()

		h.StateReadonly(nil)
	})

	t.Run("transaction nil pointer To stateView", func(t *testing.T) {
		defer mustPanic(t)

		h, cleanup := setup()
		defer cleanup()

		h.StateTransaction(nil, func() {})
	})
}

type extendedStateHandle interface {
	Validate(func(interface{}) cid.Cid)
}

type fakeActorStateHandleContext struct {
	store            runtime.Store
	head             cid.Cid
	allowSideEffects bool
}

func (ctx *fakeActorStateHandleContext) AllowSideEffects(allow bool) {
	ctx.allowSideEffects = allow
}

func (ctx *fakeActorStateHandleContext) Create(obj cbor.Marshaler) cid.Cid {
	ctx.head = ctx.store.StorePut(obj)
	return ctx.head
}

func (ctx *fakeActorStateHandleContext) Load(obj cbor.Unmarshaler) cid.Cid {
	found := ctx.store.StoreGet(ctx.head, obj)
	if !found {
		panic("inconsistent stateView")
	}
	return ctx.head
}

func (ctx *fakeActorStateHandleContext) Replace(expected cid.Cid, obj cbor.Marshaler) cid.Cid {
	if !ctx.head.Equals(expected) {
		panic(fmt.Errorf("unexpected prior stateView %s expected %s", ctx.head, expected))
	}
	ctx.head = ctx.store.StorePut(obj)
	return ctx.head
}

type testSetup struct {
	initialstate testActorStateHandleState
	h            runtime.StateHandle
	cleanup      func()
}

func mustPanic(t *testing.T) {
	if r := recover(); r == nil {
		t.Fail()
	}
}

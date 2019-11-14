package vmcontext_test

import (
	"testing"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/vmcontext"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
)

func setup() testSetup {
	initialstate := "fakestate"

	ctx := fakeActorStateHandleContext{
		storage: vm.NewTestStorage(initialstate),
	}
	initialhead := ctx.storage.Head()
	h := vmcontext.NewActorStateHandle(&ctx, initialhead)

	return testSetup{
		initialstate: initialstate,
		ctx:          ctx,
		initialhead:  initialhead,
		h:            h,
	}
}

func TestActorStateHandle(t *testing.T) {
	tf.UnitTest(t)

	t.Run("take", func(t *testing.T) {
		ts := setup()

		var out string
		ts.h.Take(&out)

		assert.Equal(t, out, ts.initialstate)
	})

	t.Run("abort on take twice", func(t *testing.T) {
		defer mustPanic(t)

		ts := setup()

		var out string
		ts.h.Take(&out)
		ts.h.Take(&out)
	})

	t.Run("release", func(t *testing.T) {
		var out string

		ts := setup()

		ts.h.Take(&out)
		ts.h.Release(&out)
	})

	t.Run("abort on releasing new state", func(t *testing.T) {
		defer mustPanic(t)

		ts := setup()

		var out string
		ts.h.Take(&out)
		ts.h.Release("some new state")
	})

	t.Run("release and take", func(t *testing.T) {
		ts := setup()

		var out string
		ts.h.Take(&out)
		ts.h.Release(out)
		ts.h.Take(&out)
		assert.Equal(t, out, ts.initialstate)
	})

	t.Run("abort when calling release without taking", func(t *testing.T) {
		defer mustPanic(t)

		ts := setup()

		ts.h.Release("fake")
	})

	t.Run("update release", func(t *testing.T) {
		var out string

		ts := setup()

		ts.h.Take(&out)
		// check state is not what we are going to use
		expected := "new state"
		assert.NotEqual(t, out, expected)
		// update the state
		ts.h.UpdateRelease(expected)
		// take it out and check that we have what we put in
		ts.h.Take(&out)
		assert.Equal(t, out, expected)
	})

	t.Run("abort when calling update release without taking", func(t *testing.T) {
		defer mustPanic(t)

		ts := setup()

		ts.h.UpdateRelease("fake")
	})
}

type fakeActorStateHandleContext struct {
	storage runtime.Storage
}

func (ctx *fakeActorStateHandleContext) Storage() runtime.Storage {
	return ctx.storage
}

type testSetup struct {
	initialstate string
	ctx          fakeActorStateHandleContext
	initialhead  cid.Cid
	h            runtime.ActorStateHandle
}

func mustPanic(t *testing.T) {
	if r := recover(); r == nil {
		t.Fail()
	}
}

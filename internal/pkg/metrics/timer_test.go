package metrics

import (
	"context"
	"testing"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
	"go.opencensus.io/stats/view"
)

func TestTimerSimple(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)

	ctx := context.Background()

	testTimer := NewTimerMs("testName", "testDesc")
	// some view state is kept around after tests exit, doing this to clean that up.
	// e.g. view will remain registered after a test exits.
	defer view.Unregister(testTimer.view)

	assert.Equal(t, "testName", testTimer.view.Name)
	assert.Equal(t, "testDesc", testTimer.view.Description)

	sw := testTimer.Start(ctx)
	sw.Stop(ctx)
	assert.NotEqual(t, 0, sw.start)

}

func TestDuplicateTimersPanics(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)

	ctx := context.Background()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("code should panic when 2 views with same name are registered")
		}
		// we pass
	}()

	NewTimerMs("testName", "testDesc")
	testTimer := NewTimerMs("testName", "testDesc")
	assert.Equal(t, "testName", testTimer.view.Name)
	assert.Equal(t, "testDesc", testTimer.view.Description)

	sw := testTimer.Start(ctx)
	sw.Stop(ctx)
	assert.NotEqual(t, 0, sw.start)

}

func TestMultipleTimers(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)

	ctx1 := context.Background()
	ctx2 := context.Background()

	tt1 := NewTimerMs("tt1", "ttd1")
	tt2 := NewTimerMs("tt2", "ttd2")

	sw1 := tt1.Start(ctx1)
	sw2 := tt2.Start(ctx2)

	sw1.Stop(ctx1)
	assert.NotEqual(t, 0, sw1.start)
	sw2.Stop(ctx2)
	assert.NotEqual(t, 0, sw2.start)

}

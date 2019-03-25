package metrics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opencensus.io/stats/view"
)

func TestTimerSimple(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()

	testTimer := NewTimer("testName", "testDesc")
	// some view state is kept around after tests exit, doing this to clean that up.
	// e.g. view will remain registered after a test exits.
	defer view.Unregister(testTimer.view)

	assert.Equal("testName", testTimer.view.Name)
	assert.Equal("testDesc", testTimer.view.Description)

	sw := testTimer.Start(ctx)
	sw.Stop(ctx)
	assert.NotEqual(0, sw.start)

}

func TestDuplicateTimersPanics(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("code should panic when 2 views with same name are registered")
		}
		// we pass
	}()

	NewTimer("testName", "testDesc")
	testTimer := NewTimer("testName", "testDesc")
	assert.Equal("testName", testTimer.view.Name)
	assert.Equal("testDesc", testTimer.view.Description)

	sw := testTimer.Start(ctx)
	sw.Stop(ctx)
	assert.NotEqual(0, sw.start)

}

func TestMultipleTimers(t *testing.T) {
	assert := assert.New(t)

	ctx1 := context.Background()
	ctx2 := context.Background()

	tt1 := NewTimer("tt1", "ttd1")
	tt2 := NewTimer("tt2", "ttd2")

	sw1 := tt1.Start(ctx1)
	sw2 := tt2.Start(ctx2)

	sw1.Stop(ctx1)
	assert.NotEqual(0, sw1.start)
	sw2.Stop(ctx2)
	assert.NotEqual(0, sw2.start)

}

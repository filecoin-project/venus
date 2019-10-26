package journal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestSimpleInMemoryJournal(t *testing.T) {
	tf.UnitTest(t)

	mj := NewInMemoryJournal(t, th.NewFakeClock(time.Unix(1234567890, 0)))
	topicJ := mj.Topic("testing")
	topicJ.Write("event1", "foo", "bar")

	memoryWriter, ok := topicJ.(*MemoryWriter)
	assert.True(t, ok)
	assert.Equal(t, 1, len(memoryWriter.journal.topics["testing"]))
	assert.Equal(t, "testing", memoryWriter.topic)

	topicJ.Write("event2", "number", 42)
	assert.Equal(t, 2, len(memoryWriter.journal.topics["testing"]))

	obj := struct {
		Name string
		Arg  int
	}{"bob",
		42,
	}
	topicJ.Write("event3", "object", obj, "name", "bob", "age", 42)
	assert.Equal(t, 3, len(memoryWriter.journal.topics["testing"]))

}

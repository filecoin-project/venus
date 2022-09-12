package journal

import (
	"testing"

	"github.com/stretchr/testify/assert"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestSimpleInMemoryJournal(t *testing.T) {
	tf.UnitTest(t)

	mj := NewInMemoryJournal(t)
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
	}{
		"bob",
		42,
	}
	topicJ.Write("event3", "object", obj, "name", "bob", "age", 42)
	assert.Equal(t, 3, len(memoryWriter.journal.topics["testing"]))
}

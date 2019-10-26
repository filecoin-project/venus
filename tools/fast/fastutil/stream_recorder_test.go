package fastutil

import (
	"bytes"
	"io/ioutil"
	"testing"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/require"
)

type window struct {
	interval *Interval
	expected *bytes.Buffer
}

func TestOverlappingWindows(t *testing.T) {
	tf.UnitTest(t)

	var err error

	sr := NewIntervalRecorder()

	// Fill up the buffer with some extra stuff
	err = writeLines(0, 10, sr)
	require.NoError(t, err)

	// Open our first window
	win1 := window{
		interval: sr.Start(),
		expected: bytes.NewBuffer(nil),
	}

	// Write some data to the window
	err = writeLines(1, 10, sr, win1.expected)
	require.NoError(t, err)

	// Open new window
	win2 := window{
		interval: sr.Start(),
		expected: bytes.NewBuffer(nil),
	}

	// Write data, which should show up in both windows
	err = writeLines(2, 10, sr, win1.expected, win2.expected)
	require.NoError(t, err)

	// Close the first window
	win1.interval.Stop()

	// Write data, which should show up in only the second window
	err = writeLines(3, 10, sr, win2.expected)
	require.NoError(t, err)

	// Close the second window
	win2.interval.Stop()

	for _, win := range []window{win1, win2} {
		data, err := ioutil.ReadAll(win.interval)
		require.NoError(t, err)

		compare(t, win.expected.Bytes(), data)
	}

}

func TestEmbeddedWindows(t *testing.T) {
	tf.UnitTest(t)

	var err error

	sr := NewIntervalRecorder()

	// Fill up the buffer with some extra stuff
	err = writeLines(0, 10, sr)
	require.NoError(t, err)

	// Open our first window
	win1 := window{
		interval: sr.Start(),
		expected: bytes.NewBuffer(nil),
	}

	// Write some data to the window
	err = writeLines(1, 10, sr, win1.expected)
	require.NoError(t, err)

	// Open new window
	win2 := window{
		interval: sr.Start(),
		expected: bytes.NewBuffer(nil),
	}

	// Write data, which should show up in both windows
	err = writeLines(2, 10, sr, win1.expected, win2.expected)
	require.NoError(t, err)

	// Close the second window
	win2.interval.Stop()

	// Write data, which should show up in only the first window
	err = writeLines(3, 10, sr, win1.expected)
	require.NoError(t, err)

	// Close the first window
	win1.interval.Stop()

	for _, win := range []window{win1, win2} {
		data, err := ioutil.ReadAll(win.interval)
		require.NoError(t, err)

		compare(t, win.expected.Bytes(), data)
	}
}

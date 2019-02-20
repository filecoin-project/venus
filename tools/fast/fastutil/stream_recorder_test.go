package fastutil

import (
	"bytes"
	"io/ioutil"
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

type window struct {
	interval *Interval
	expected *bytes.Buffer
}

func TestOverlappingWindows(t *testing.T) {
	var err error

	require := require.New(t)

	sr := NewIntervalRecorder()

	// Fill up the buffer with some extra stuff
	err = writeLines(0, 10, sr)
	require.NoError(err)

	// Open our first window
	win1 := window{
		interval: sr.Start(),
		expected: bytes.NewBuffer(nil),
	}

	// Write some data to the window
	err = writeLines(1, 10, sr, win1.expected)
	require.NoError(err)

	// Open new window
	win2 := window{
		interval: sr.Start(),
		expected: bytes.NewBuffer(nil),
	}

	// Write data, which should show up in both windows
	err = writeLines(2, 10, sr, win1.expected, win2.expected)
	require.NoError(err)

	// Close the first window
	win1.interval.Stop()

	// Write data, which should show up in only the second window
	err = writeLines(3, 10, sr, win2.expected)
	require.NoError(err)

	// Close the second window
	win2.interval.Stop()

	for _, win := range []window{win1, win2} {
		data, err := ioutil.ReadAll(win.interval)
		require.NoError(err)

		compare(t, win.expected.Bytes(), data)
	}

}

func TestEmbeddedWindows(t *testing.T) {
	var err error

	require := require.New(t)

	sr := NewIntervalRecorder()

	// Fill up the buffer with some extra stuff
	err = writeLines(0, 10, sr)
	require.NoError(err)

	// Open our first window
	win1 := window{
		interval: sr.Start(),
		expected: bytes.NewBuffer(nil),
	}

	// Write some data to the window
	err = writeLines(1, 10, sr, win1.expected)
	require.NoError(err)

	// Open new window
	win2 := window{
		interval: sr.Start(),
		expected: bytes.NewBuffer(nil),
	}

	// Write data, which should show up in both windows
	err = writeLines(2, 10, sr, win1.expected, win2.expected)
	require.NoError(err)

	// Close the second window
	win2.interval.Stop()

	// Write data, which should show up in only the first window
	err = writeLines(3, 10, sr, win1.expected)
	require.NoError(err)

	// Close the first window
	win1.interval.Stop()

	for _, win := range []window{win1, win2} {
		data, err := ioutil.ReadAll(win.interval)
		require.NoError(err)

		compare(t, win.expected.Bytes(), data)
	}
}

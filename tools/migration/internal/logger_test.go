package internal_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

func TestLoggerWritesToFile(t *testing.T) {
	f, err := ioutil.TempFile("", "logfile")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.Remove(f.Name()))
	}()

	logger := NewLogger(f, false)
	logger.Print("testing print 1")
	errTest := errors.New("testing error 2")
	logger.Error(errTest)

	// Reopen file so we can read new writes
	out, err := ioutil.ReadFile(f.Name())
	require.NoError(t, err)
	outStr := string(out)
	assert.Contains(t, outStr, "testing print 1")
	expectedErrStr := fmt.Sprintf("ERROR: %s", errTest.Error())
	assert.Contains(t, outStr, expectedErrStr)
}

func TestLoggerWritesToBothVerbose(t *testing.T) {
	// Point os.Stdout to a temp file
	fStdout, err := ioutil.TempFile("", "stdout")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.Remove(fStdout.Name()))
	}()
	old := os.Stdout
	os.Stdout = fStdout
	defer func() { os.Stdout = old }()

	// Create log file
	fLogFile, err := ioutil.TempFile("", "logfile")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.Remove(fLogFile.Name()))
	}()

	// Log verbosely
	logger := NewLogger(fLogFile, true)
	logger.Print("test line")
	errTest := errors.New("test err")
	logger.Error(errTest)
	expectedErrStr := fmt.Sprintf("ERROR: %s", errTest.Error())

	// Check logfile
	outLogFile, err := ioutil.ReadFile(fLogFile.Name())
	require.NoError(t, err)
	outLogFileStr := string(outLogFile)
	assert.Contains(t, outLogFileStr, "test line")
	assert.Contains(t, outLogFileStr, expectedErrStr)

	// Check stdout alias file
	outStdout, err := ioutil.ReadFile(fStdout.Name())
	require.NoError(t, err)
	outStdoutStr := string(outStdout)
	assert.Contains(t, outStdoutStr, "test line")
	assert.Contains(t, outStdoutStr, expectedErrStr)
}

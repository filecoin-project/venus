package testhelpers

import (
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CmdOutput collects the output from a CLI command issued to a process.
type CmdOutput struct {
	// Args is the command and argument sequence executed.
	Args []string

	outbytes []byte
	errbytes []byte

	// The status or exit code of a successfully-executed command. This value is only meaningful
	// after the command completes, and if error is nil.
	status int
	// Any error encountered in arranging execution of the command (e.g. I/O errors reading from
	// streams). An error here indicates that the status and output buffers may not be meaningful.
	// This is distinguished from an "application" error indicated in the status code and output
	// streams.
	error error

	tb testing.TB
}

// ReadOutput reads the `stdout` and `stderr` streams completely and returns a new Output object.
func ReadOutput(tb testing.TB, args []string, stdout io.Reader, stderr io.Reader) *CmdOutput {
	// Consume the streams in parallel to avoid potential deadlock around the remote process
	// blocking on a full error stream buffer (e.g. logs) while we're waiting for the output
	// stream to complete, or vice versa.
	outCh := readAllAsync(tb, stdout)
	errCh := readAllAsync(tb, stderr)

	return &CmdOutput{
		Args:     args,
		outbytes: <-outCh,
		errbytes: <-errCh,
		tb:       tb,
	}
}

// Status returns the status code and any error value from execution.
// The status code and any output streams are valid only if error is nil.
func (o *CmdOutput) Status() (int, error) {
	return o.status, o.error
}

// SetStatus sets the status code for a successful invocation.
// May not be called if a status code has been set (probably indicating a usage error).
func (o *CmdOutput) SetStatus(status int) {
	require.NoError(o.tb, o.error, "invocation error already reported!")
	require.Equal(o.tb, 0, o.status, "non-zero status already reported, probably a usage error!")
	o.status = status
}

// SetInvocationError sets the error for an unsuccessful invocation.
// May not be called if a status code has been set (probably indicating a usage error).
func (o *CmdOutput) SetInvocationError(executionErr error) {
	require.NoError(o.tb, o.error, "invocation error already reported!")
	require.Equal(o.tb, 0, o.status, "non-zero status already reported, probably a usage error!")
	o.error = executionErr
}

// Stderr returns the raw bytes from stderr.
func (o *CmdOutput) Stderr() []byte {
	o.requireNoError()
	return o.errbytes
}

// Stdout returns the raw bytes from stdout.
func (o *CmdOutput) Stdout() []byte {
	o.requireNoError()
	return o.outbytes
}

// ReadStderr returns a string representation of the stderr output.
func (o *CmdOutput) ReadStderr() string {
	return string(o.Stderr())
}

// ReadStdout returns a string representation of the stdout output.
func (o *CmdOutput) ReadStdout() string {
	return string(o.Stdout())
}

// ReadStdoutTrimNewlines returns a string representation of stdout,
// with trailing line breaks removed.
func (o *CmdOutput) ReadStdoutTrimNewlines() string {
	// TODO: handle non unix line breaks
	return strings.Trim(o.ReadStdout(), "\n")
}

// AssertSuccess asserts that the output represents a successful execution.
func (o *CmdOutput) AssertSuccess() *CmdOutput {
	o.tb.Helper()
	oErr := o.ReadStderr() // Also checks no invocation error.

	assert.NotContains(o.tb, oErr, "CRITICAL")
	assert.NotContains(o.tb, oErr, "ERROR")
	assert.NotContains(o.tb, oErr, "WARNING")
	assert.NotContains(o.tb, oErr, "Error:")
	return o
}

// AssertFail asserts that the output represents a failed execution, with the error
// matching the passed in error.
func (o *CmdOutput) AssertFail(err string) *CmdOutput {
	o.tb.Helper()
	assert.Empty(o.tb, o.ReadStdout()) // Also checks no invocation error.
	assert.Contains(o.tb, o.ReadStderr(), err)
	return o
}

// requireNoError requires that no execution error has been recorded, which would render the status
// code and output streams incomplete.
func (o *CmdOutput) requireNoError() {
	require.NoError(o.tb, o.error, "execution error for \"%s\"", strings.Join(o.Args, " "))
}

// Reads everything a reader has to offer into memory, providing the completed buffer on
// the returned channel.
func readAllAsync(tb testing.TB, r io.Reader) chan []byte {
	ch := make(chan []byte, 1)
	go func() {
		bytes, err := ioutil.ReadAll(r)
		require.NoError(tb, err)
		if err == nil {
			ch <- bytes
		} else {
			close(ch)
		}
	}()
	return ch
}

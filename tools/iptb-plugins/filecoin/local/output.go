package pluginlocalfilecoin

import (
	"context"
	"io"
	"os/exec"
)

type Output struct {
	args     []string
	err      error
	exitcode int
	exitset  bool

	ctx context.Context

	cmd *exec.Cmd

	stdout io.ReadCloser
	stderr io.ReadCloser
}

func (o *Output) Args() []string {
	return o.args
}

func (o *Output) Error() error {
	return o.err
}

func (o *Output) ExitCode() (exitcode int) {
	defer func() {
		o.exitset = true
		exitcode = o.exitcode
	}()

	if o.exitset {
		return
	}

	if o.err != nil {
		o.exitcode = -1
		return
	}

	exiterr := o.cmd.Wait()

	// The command may exit because the deadline was exceeded while it was running
	if o.ctx.Err() == context.DeadlineExceeded {
		o.err = context.DeadlineExceeded
		o.exitcode = -1
		return
	}

	switch err := exiterr.(type) {
	case *exec.ExitError:
		o.exitcode = 1
		return
	case nil:
		o.exitcode = 0
		return
	default:
		o.err = err
		o.exitcode = -1
		return
	}
}

func (o *Output) Stdout() io.ReadCloser {
	return o.stdout
}

func (o *Output) Stderr() io.ReadCloser {
	return o.stderr
}

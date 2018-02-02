package testhelpers

import (
	"bytes"
	"context"
	"io"

	cmds "gx/ipfs/Qmc5paX4ECBARnAKkcAmUYHBGor228Tkfxeya3Nu2KRL46/go-ipfs-cmds"
)

type wc struct {
	io.Writer
	io.Closer
}

// nopClose implements io.Close and does nothing
type nopCloser struct{}

func (c nopCloser) Close() error { return nil }

type mockenv int

func (e *mockenv) Context() context.Context {
	return context.Background()
}

// RunCommand is used to simulate calls to the commands library
func RunCommand(root *cmds.Command, args []string) (string, error) {
	env := mockenv(42)
	req, err := cmds.NewRequest(context.Background(), []string{}, nil, []string{"version"}, nil, root)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	re := cmds.NewWriterResponseEmitter(wc{&buf, nopCloser{}}, req, cmds.Encoders[cmds.Text])

	x := cmds.NewExecutor(root)
	err = x.Execute(req, re, &env)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

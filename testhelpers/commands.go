package testhelpers

import (
	"bytes"
	"context"
	"io"

	cmds "gx/ipfs/QmWGgKRz5S24SqaAapF5PPCfYfLT7MexJZewN5M82CQTzs/go-ipfs-cmds"
)

type wc struct {
	io.Writer
	io.Closer
}

// nopClose implements io.Close and does nothing
type nopCloser struct{}

func (c nopCloser) Close() error { return nil }

// RunCommand is used to simulate calls to the commands library
func RunCommand(root *cmds.Command, args []string, env cmds.Environment) (string, error) {
	req, err := cmds.NewRequest(context.Background(), []string{}, nil, []string{"version"}, nil, root)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	re := cmds.NewWriterResponseEmitter(wc{&buf, nopCloser{}}, req, cmds.Encoders[cmds.Text])

	x := cmds.NewExecutor(root)
	err = x.Execute(req, re, env)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

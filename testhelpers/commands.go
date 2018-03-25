package testhelpers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	cmds "gx/ipfs/QmYMj156vnPY7pYvtkvQiMDAzqWDDHkfiW5bYbMpYoHxhB/go-ipfs-cmds"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
)

type wc struct {
	io.Writer
	io.Closer
}

// nopClose implements io.Close and does nothing
type nopCloser struct{}

func (c nopCloser) Close() error { return nil }

// TextOutput stores the result of running a command.
type TextOutput struct {
	Lines []string
	Raw   string
}

// HasLine checks if the given line is part of the output.
func (to *TextOutput) HasLine(s string) error {
	for _, l := range to.Lines {
		if l == s {
			return nil
		}
	}
	return fmt.Errorf("expected %q\n\n\tto have a line matching:\n\n%q", to.Raw, s)
}

// Equals checks if the given string is equal to the raw output.
func (to *TextOutput) Equals(s string) bool {
	return to.Raw == s
}

// RunCommand is used to simulate calls to the commands library
func RunCommand(root *cmds.Command, args []string, opts map[string]interface{}, env cmds.Environment) (*TextOutput, error) {
	ctx := context.Background()
	req, err := cmds.NewRequest(ctx, []string{}, opts, args, nil, root)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	req.Options[cmds.EncLong] = cmds.Text

	re := cmds.NewWriterResponseEmitter(wc{Writer: &buf, Closer: &nopCloser{}}, req, root.Encoders[cmds.Text])
	err = root.Run(req, re, env)

	if err != nil {
		re.SetError(err, cmdkit.ErrNormal)
	}

	return &TextOutput{
		Lines: strings.Split(buf.String(), "\n"),
		Raw:   buf.String(),
	}, nil
}

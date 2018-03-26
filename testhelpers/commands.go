package testhelpers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	cmds "gx/ipfs/QmYMj156vnPY7pYvtkvQiMDAzqWDDHkfiW5bYbMpYoHxhB/go-ipfs-cmds"
)

type writercloser struct {
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

	// if a specific encoding is requested, use it
	// otherwise, default to text

	var etypestr = "text"
	if _, ok := opts[cmds.EncLong]; ok {
		etypestr = opts[cmds.EncLong].(string)
	}
	req.Options[cmds.EncLong] = etypestr

	var buf bytes.Buffer
	wc := writercloser{Writer: &buf, Closer: &nopCloser{}}
	re := cmds.NewWriterResponseEmitter(wc, req, root.Encoders[cmds.EncodingType(etypestr)])

	if err := root.Run(req, re, env); err != nil {
		return nil, err
	}

	return &TextOutput{
		Lines: strings.Split(buf.String(), "\n"),
		Raw:   buf.String(),
	}, nil
}

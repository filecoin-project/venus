package testhelpers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	cmds "gx/ipfs/QmRv6ddf7gkiEgBs1LADv3vC1mkVGPZEfByoiiVybjE9Mc/go-ipfs-cmds"
)

type wc struct {
	io.Writer
	io.Closer
}

// nopClose implements io.Close and does nothing
type nopCloser struct{}

func (c nopCloser) Close() error { return nil }

type TextOutput struct {
	Lines []string
	Raw   string
}

func (to *TextOutput) HasLine(s string) error {
	for _, l := range to.Lines {
		if l == s {
			return nil
		}
	}
	return fmt.Errorf("%q\n\n\tdoes not have a line matching:\n\n%q\n", to.Raw, s)
}

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
	root.Run(req, re, env)

	return &TextOutput{
		Lines: strings.Split(buf.String(), "\n"),
		Raw:   buf.String(),
	}, nil
}

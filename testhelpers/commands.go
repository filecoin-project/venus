package testhelpers

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"gx/ipfs/QmVTmXZC2yE38SDKRihn96LXX6KwBWgzAg8aCDZaMirCHm/go-ipfs-cmds"
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

// RunCommandJSONEnc delegates to RunCommand, configuring opts to use JSON encoding
func RunCommandJSONEnc(root *cmds.Command, args []string, opts map[string]interface{}, env cmds.Environment) (*TextOutput, error) {
	if opts == nil {
		opts = map[string]interface{}{}
	}
	opts[cmds.EncLong] = cmds.JSON
	return RunCommand(root, args, opts, env)
}

// RunCommand is used to simulate calls to the commands library
func RunCommand(root *cmds.Command, args []string, opts map[string]interface{}, env cmds.Environment) (*TextOutput, error) {
	ctx := env.Context()
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

	var encoderFunc cmds.EncoderFunc
	if f, ok := root.Encoders[cmds.EncodingType(etypestr)]; ok {
		encoderFunc = f
	} else {
		encoderFunc = cmds.Encoders[cmds.EncodingType(etypestr)]
	}

	var buf bytes.Buffer
	wc := writercloser{Writer: &buf, Closer: &nopCloser{}}
	re := cmds.NewWriterResponseEmitter(wc, req, encoderFunc)

	root.Run(req, re, env)

	return &TextOutput{
		Lines: strings.Split(buf.String(), "\n"),
		Raw:   buf.String(),
	}, nil
}

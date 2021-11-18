package testutil

import (
	"bytes"
	"testing"

	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/stretchr/testify/assert"
)

type CborErBasicTestOptions struct {
	Buf         *bytes.Buffer
	Prepare     func()
	ProvideOpts []interface{}
	Provided    func()
	Marshaled   func(data []byte)
	Finished    func()
}

func CborErBasicTest(t *testing.T, src, dst cbor.Er, opts CborErBasicTestOptions) {
	if opts.Prepare != nil {
		opts.Prepare()
	}

	Provide(t, src, opts.ProvideOpts...)
	if opts.Provided != nil {
		opts.Provided()
	}

	opts.Buf.Reset()

	err := src.MarshalCBOR(opts.Buf)
	assert.NoErrorf(t, err, "marshal from src of %T", src)

	if opts.Marshaled != nil {
		opts.Marshaled(opts.Buf.Bytes())
	}

	err = dst.UnmarshalCBOR(opts.Buf)
	assert.NoErrorf(t, err, "unmarshal to dst of %T", dst)

	if opts.Finished != nil {
		opts.Finished()
	}
}

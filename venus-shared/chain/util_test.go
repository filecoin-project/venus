package chain

import (
	"bytes"
	"testing"

	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/venus-shared/testutil"
)

type cborBasicTestOptions struct {
	buf *bytes.Buffer

	before func()

	provideOpts []interface{}
	provided    func()

	marshaled func([]byte)

	after func()
}

func cborBasicTest(t *testing.T, src, dst cbor.Er, opt cborBasicTestOptions) {
	if opt.before != nil {
		opt.before()
	}

	testutil.Provide(t, src, opt.provideOpts...)
	if opt.provided != nil {
		opt.provided()
	}

	opt.buf.Reset()
	err := src.MarshalCBOR(opt.buf)
	assert.NoErrorf(t, err, "marshaling from src of %T", src)

	data := opt.buf.Bytes()
	if opt.marshaled != nil {
		opt.marshaled(data)
	}

	err = dst.UnmarshalCBOR(bytes.NewReader(data))
	assert.NoErrorf(t, err, "unmarshaling to dst of %T", dst)

	if opt.after != nil {
		opt.after()
	}
}

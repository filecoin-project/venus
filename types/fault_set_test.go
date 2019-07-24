package types

import (
	"testing"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/stretchr/testify/assert"
)

func TestFaultSetCborMarshaling(t *testing.T) {
	tf.UnitTest(t)

	t.Run("encode/decode symmetry", func(t *testing.T) {
		faultSet := NewFaultSet([]uint64{4096, 1, 2, 3, 8192})
		decoded := NewFaultSet([]uint64{})

		out, err := cbor.DumpObject(faultSet)
		assert.NoError(t, err)

		err = cbor.DecodeInto(out, &decoded)
		assert.NoError(t, err)

		assert.Equal(t, faultSet.SectorIds.Values(), decoded.SectorIds.Values())
	})
}

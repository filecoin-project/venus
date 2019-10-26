package types

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"

	"github.com/stretchr/testify/assert"
)

func TestFaultSetCborMarshaling(t *testing.T) {
	tf.UnitTest(t)

	t.Run("encode/decode symmetry", func(t *testing.T) {
		faultSet := NewFaultSet([]uint64{4096, 1, 2, 3, 8192})
		decoded := NewFaultSet([]uint64{})

		out, err := encoding.Encode(faultSet)
		assert.NoError(t, err)

		err = encoding.Decode(out, &decoded)
		assert.NoError(t, err)

		assert.Equal(t, faultSet.SectorIds.Values(), decoded.SectorIds.Values())
	})
}

package porcelain_test

import (
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

type testProtocolParamsPlumbing struct {
	assert           *assert.Assertions
	autoSealInterval uint
}

func (tppp *testProtocolParamsPlumbing) ConfigGet(path string) (interface{}, error) {
	tppp.assert.Equal("mining.autoSealIntervalSeconds", path)
	return tppp.autoSealInterval, nil
}

func TestProtocolParams(t *testing.T) {
	t.Parallel()

	t.Run("emits the a ProtocolParams object with the correct values", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)

		plumbing := &testProtocolParamsPlumbing{
			assert:           assert,
			autoSealInterval: 120,
		}

		expected := &porcelain.ProtocolParams{
			AutoSealInterval: 120,
		}

		out, err := porcelain.ProtocolParameters(plumbing)
		require.NoError(err)

		assert.Equal(expected, out)
	})
}

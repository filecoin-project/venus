package porcelain_test

import (
	"context"
	"time"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

const protocolTestParamBlockTime = time.Second

type testProtocolParamsPlumbing struct {
	testing          *testing.T
	autoSealInterval uint
}

func (tppp *testProtocolParamsPlumbing) ConfigGet(path string) (interface{}, error) {
	assert.Equal(tppp.testing, "mining.autoSealIntervalSeconds", path)
	return tppp.autoSealInterval, nil
}

func (tppp *testProtocolParamsPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
	return [][]byte{{byte(types.TestProofsMode)}}, nil
}

func (tppp *testProtocolParamsPlumbing) BlockTime() time.Duration {
	return protocolTestParamBlockTime
}

func TestProtocolParams(t *testing.T) {
	t.Parallel()

	t.Run("emits the a ProtocolParams object with the correct values", func(t *testing.T) {
		t.Parallel()

		plumbing := &testProtocolParamsPlumbing{
			testing:          t,
			autoSealInterval: 120,
		}

		expected := &porcelain.ProtocolParams{
			AutoSealInterval:     120,
			ProofsMode:           types.TestProofsMode,
			SupportedSectorSizes: []*types.BytesAmount{types.OneKiBSectorSize},
			BlockTime:            protocolTestParamBlockTime,
		}

		out, err := porcelain.ProtocolParameters(context.TODO(), plumbing)
		require.NoError(t, err)

		assert.Equal(t, expected, out)
	})
}

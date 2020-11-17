package porcelain_test

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-state-types/abi"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/constants"
	"github.com/filecoin-project/venus/internal/pkg/state"
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

func (tppp *testProtocolParamsPlumbing) ChainHeadKey() block.TipSetKey {
	return block.NewTipSetKey()
}

func (tppp *testProtocolParamsPlumbing) BlockTime() time.Duration {
	return protocolTestParamBlockTime
}

func (tppp *testProtocolParamsPlumbing) ProtocolStateView(_ block.TipSetKey) (porcelain.ProtocolStateView, error) {
	return &state.FakeStateView{
		NetworkName: "protocolTest",
	}, nil
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
			Network: "protocolTest",
			SupportedSectors: []porcelain.SectorInfo{
				{constants.DevSectorSize, abi.PaddedPieceSize(constants.DevSectorSize).Unpadded()},
				{constants.FiveHundredTwelveMiBSectorSize, abi.PaddedPieceSize(constants.FiveHundredTwelveMiBSectorSize).Unpadded()},
			},
			BlockTime: protocolTestParamBlockTime,
		}

		out, err := porcelain.ProtocolParameters(context.TODO(), plumbing)
		require.NoError(t, err)

		assert.Equal(t, expected, out)
	})
}

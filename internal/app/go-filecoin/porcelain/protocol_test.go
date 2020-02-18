package porcelain_test

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/specs-actors/actors/abi"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			AutoSealInterval: 120,
			Network:          "protocolTest",
			SupportedSectors: []porcelain.SectorInfo{
				{types.OneKiBSectorSize, types.NewBytesAmount(uint64(abi.PaddedPieceSize(types.OneKiBSectorSize.Uint64()).Unpadded()))},
				{types.TwoHundredFiftySixMiBSectorSize, types.NewBytesAmount(uint64(abi.PaddedPieceSize(types.TwoHundredFiftySixMiBSectorSize.Uint64()).Unpadded()))},
			},
			BlockTime: protocolTestParamBlockTime,
		}

		out, err := porcelain.ProtocolParameters(context.TODO(), plumbing)
		require.NoError(t, err)

		assert.Equal(t, expected, out)
	})
}

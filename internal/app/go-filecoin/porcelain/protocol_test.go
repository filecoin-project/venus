package porcelain_test

import (
	"context"
	"testing"
	"time"

	ffi "github.com/filecoin-project/filecoin-ffi"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/pkg/errors"

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

func (tppp *testProtocolParamsPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, _ block.TipSetKey, params ...interface{}) ([][]byte, error) {
	if to == address.StorageMarketAddress && method == storagemarket.GetProofsMode {
		return [][]byte{{byte(types.TestProofsMode)}}, nil
	} else if to == address.InitAddress && method == initactor.GetNetworkMethodID {
		return [][]byte{[]byte("protocolTest")}, nil
	}
	return [][]byte{}, errors.Errorf("call to unknown method %s", method)
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

		sectorSize := types.OneKiBSectorSize
		maxUserBytes := types.NewBytesAmount(ffi.GetMaxUserBytesPerStagedSector(sectorSize.Uint64()))

		expected := &porcelain.ProtocolParams{
			AutoSealInterval: 120,
			Network:          "protocolTest",
			ProofsMode:       types.TestProofsMode,
			SupportedSectors: []porcelain.SectorInfo{{sectorSize, maxUserBytes}},
			BlockTime:        protocolTestParamBlockTime,
		}

		out, err := porcelain.ProtocolParameters(context.TODO(), plumbing)
		require.NoError(t, err)

		assert.Equal(t, expected, out)
	})
}

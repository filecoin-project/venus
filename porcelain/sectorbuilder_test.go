package porcelain_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sectorbuilderTestPlumbing struct {
	require  *require.Assertions
	sectorID uint64
}

func (stp *sectorbuilderTestPlumbing) MessageQuery(
	ctx context.Context,
	optFrom,
	to address.Address,
	method string,
	params ...interface{},
) ([][]byte, *exec.FunctionSignature, error) {
	signature := &exec.FunctionSignature{
		Params: nil,
		Return: []abi.Type{abi.SectorID},
	}
	val := abi.Value{
		Type: abi.SectorID,
		Val:  stp.sectorID,
	}
	ret, err := val.Serialize()
	stp.require.NoError(err)
	return [][]byte{ret}, signature, nil
}

func TestSectorBuilderGetLastUsedID(t *testing.T) {
	t.Run("return the correct last used ID", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)
		ctx := context.Background()

		expectedSectorID := uint64(12345)
		plumbing := &sectorbuilderTestPlumbing{
			require:  require,
			sectorID: expectedSectorID,
		}
		sectorID, err := porcelain.SectorBuilderGetLastUsedID(ctx, plumbing, address.Address{})
		require.NoError(err)

		assert.Equal(expectedSectorID, sectorID)
	})
}

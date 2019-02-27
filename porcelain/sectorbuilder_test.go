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
	assert       *assert.Assertions
	minerAddress address.Address
	require      *require.Assertions
	sectorID     uint64
}

func (stp *sectorbuilderTestPlumbing) ConfigGet(dottedPath string) (interface{}, error) {
	return stp.minerAddress, nil
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

func (stp *sectorbuilderTestPlumbing) SectorBuilderIsRunning() bool {
	return false
}

func (stp *sectorbuilderTestPlumbing) SectorBuilderStart(addr address.Address, sectorID uint64) error {
	stp.assert.Equal(stp.minerAddress, addr)
	stp.assert.Equal(stp.sectorID, sectorID)
	return nil
}

func TestSectorBuilderSetup(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()

	plumbing := &sectorbuilderTestPlumbing{
		assert:       assert,
		minerAddress: address.Address{},
		require:      require,
		sectorID:     uint64(12345),
	}

	err := porcelain.SectorBuilderSetup(ctx, plumbing)
	require.NoError(err)
}

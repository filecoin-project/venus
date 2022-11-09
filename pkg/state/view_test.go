// stm: #unit
package state_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/testhelpers"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/stretchr/testify/assert"
)

func setupTestMinerView(t *testing.T, numMiners int) (*state.View, map[address.Address]*crypto.KeyInfo) {
	tf.UnitTest(t)
	ctx := context.Background()
	numCommittedSectors := uint64(19)
	kis := testhelpers.MustGenerateBLSKeyInfo(numMiners, 0)

	kiMap := make(map[address.Address]*crypto.KeyInfo)
	for _, k := range kis {
		addr, err := k.Address()
		assert.NoError(t, err)
		kiMap[addr] = &k
	}

	store, _, root := requireMinerWithNumCommittedSectors(ctx, t, numCommittedSectors, kis)
	return state.NewView(store, root), kiMap
}

func TestView(t *testing.T) {
	numMiners := 2
	view, keyMap := setupTestMinerView(t, numMiners)
	ctx := context.Background()

	miners, err := view.StateListMiners(ctx, types.EmptyTSK)
	assert.NoError(t, err)
	assert.Equal(t, len(miners), numMiners)

	for _, m := range miners {
		// stm: @STATE_VIEW_MINER_EXISTS_001
		exist, err := view.MinerExists(ctx, m)
		assert.NoError(t, err)
		assert.True(t, exist)

		// stm: @STATE_VIEW_GET_MINER_INFO_001
		minerInfo, err := view.MinerInfo(ctx, m, network.Version17)
		assert.NoError(t, err)

		ownerPkAddress, err := view.ResolveToKeyAddr(ctx, minerInfo.Owner)
		assert.NoError(t, err)
		_, find := keyMap[ownerPkAddress]
		assert.True(t, find)

		// stm: @STATE_VIEW_GET_MINER_SECTOR_INFO_001
		sectorInfo, err := view.MinerSectorInfo(ctx, m, 0)
		assert.NoError(t, err)
		assert.Equal(t, sectorInfo.SectorNumber, abi.SectorNumber(0))

		// stm: @STATE_VIEW_SECTOR_PRE_COMMIT_INFO_001
		_, err = view.SectorPreCommitInfo(ctx, m, 0)
		assert.NoError(t, err)

		// stm: @STATE_VIEW_MINER_GET_PRECOMMITED_SECTOR
		_, find, err = view.MinerGetPrecommittedSector(ctx, m, abi.SectorNumber(0))
		assert.NoError(t, err)
		assert.True(t, find)

		// stm: @STATE_VIEW_STATE_SECTOR_PARTITION_001
		_, err = view.StateSectorPartition(ctx, m, 0)
		assert.NoError(t, err)

		// stm: @STATE_VIEW_DEADLINE_INFO_001
		_, _, _, _, err = view.MinerDeadlineInfo(ctx, m, abi.ChainEpoch(1))
		assert.NoError(t, err)
	}
}

package slashing_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/pkg/block"
	. "github.com/filecoin-project/venus/pkg/slashing"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/pkg/types"
)

func assertEmptyCh(t *testing.T, faultCh chan ConsensusFault) {
	select {
	case <-faultCh:
		t.Fail()
	default:
	}
}

func TestNoFaults(t *testing.T) {
	tf.UnitTest(t)
	addrGetter := types.NewForTestGetter()
	minerAddr1 := addrGetter()
	minerAddr2 := addrGetter()
	minerAddr3 := addrGetter()

	mockCid := types.CidFromString(t, "mock")
	t.Run("blocks mined by different miners don't slash", func(t *testing.T) {
		parentBlock := &types.BlockHeader{Height: 42, Miner: minerAddr1, Messages: mockCid, ParentMessageReceipts: mockCid, ParentStateRoot: mockCid}
		parentTipSet := block.RequireNewTipSet(t, parentBlock)
		block1 := &types.BlockHeader{Miner: minerAddr1, Height: 43, Messages: mockCid, ParentMessageReceipts: mockCid, ParentStateRoot: mockCid}
		block2 := &types.BlockHeader{Miner: minerAddr2, Height: 43, Messages: mockCid, ParentMessageReceipts: mockCid, ParentStateRoot: mockCid}
		block3 := &types.BlockHeader{Miner: minerAddr3, Height: 43, Messages: mockCid, ParentMessageReceipts: mockCid, ParentStateRoot: mockCid}

		faultCh := make(chan ConsensusFault, 1)
		cfd := NewConsensusFaultDetector(faultCh)
		assert.NoError(t, cfd.CheckBlock(block1, parentTipSet))
		assertEmptyCh(t, faultCh)
		assert.NoError(t, cfd.CheckBlock(block2, parentTipSet))
		assertEmptyCh(t, faultCh)
		assert.NoError(t, cfd.CheckBlock(block3, parentTipSet))
		assertEmptyCh(t, faultCh)
	})

	t.Run("blocks mined at different heights don't slash", func(t *testing.T) {
		parent1Block := &types.BlockHeader{Height: 42, Miner: minerAddr1, Messages: mockCid, ParentMessageReceipts: mockCid, ParentStateRoot: mockCid}
		parent1TipSet := block.RequireNewTipSet(t, parent1Block)
		block1 := &types.BlockHeader{Miner: minerAddr1, Height: 43, Messages: mockCid, ParentMessageReceipts: mockCid, ParentStateRoot: mockCid}

		parent2Block := &types.BlockHeader{Height: 55, Miner: minerAddr1, Messages: mockCid, ParentMessageReceipts: mockCid, ParentStateRoot: mockCid}
		parent2TipSet := block.RequireNewTipSet(t, parent2Block)
		block2 := &types.BlockHeader{Miner: minerAddr1, Height: 56, Messages: mockCid, ParentMessageReceipts: mockCid, ParentStateRoot: mockCid}

		faultCh := make(chan ConsensusFault, 1)
		cfd := NewConsensusFaultDetector(faultCh)
		assert.NoError(t, cfd.CheckBlock(block1, parent1TipSet))
		assertEmptyCh(t, faultCh)
		assert.NoError(t, cfd.CheckBlock(block2, parent2TipSet))
		assertEmptyCh(t, faultCh)
	})

	t.Run("blocks with non-overlapping null intervals don't slash", func(t *testing.T) {
		parent1Block := &types.BlockHeader{Height: 42, Miner: minerAddr1, Messages: mockCid, ParentMessageReceipts: mockCid, ParentStateRoot: mockCid}
		parent1TipSet := block.RequireNewTipSet(t, parent1Block)
		block1 := &types.BlockHeader{Miner: minerAddr1, Height: 46, Messages: mockCid, ParentMessageReceipts: mockCid, ParentStateRoot: mockCid}

		parent2TipSet := block.RequireNewTipSet(t, block1)
		block2 := &types.BlockHeader{Miner: minerAddr1, Height: 56, Messages: mockCid, ParentMessageReceipts: mockCid, ParentStateRoot: mockCid}

		faultCh := make(chan ConsensusFault, 1)
		cfd := NewConsensusFaultDetector(faultCh)
		assert.NoError(t, cfd.CheckBlock(block1, parent1TipSet))
		assertEmptyCh(t, faultCh)
		assert.NoError(t, cfd.CheckBlock(block2, parent2TipSet))
		assertEmptyCh(t, faultCh)
	})

	t.Run("duplicate equal blocks don't slash", func(t *testing.T) {
		parentBlock := &types.BlockHeader{Height: 42, Miner: minerAddr1, Messages: mockCid, ParentMessageReceipts: mockCid, ParentStateRoot: mockCid}
		parentTipSet := block.RequireNewTipSet(t, parentBlock)

		block := &types.BlockHeader{Miner: minerAddr1, Height: 43, Messages: mockCid, ParentMessageReceipts: mockCid, ParentStateRoot: mockCid}
		faultCh := make(chan ConsensusFault, 1)
		cfd := NewConsensusFaultDetector(faultCh)
		assert.NoError(t, cfd.CheckBlock(block, parentTipSet))
		assertEmptyCh(t, faultCh)
		assert.NoError(t, cfd.CheckBlock(block, parentTipSet))
		assertEmptyCh(t, faultCh)
	})
}

func TestFault(t *testing.T) {
	tf.UnitTest(t)
	addrGetter := types.NewForTestGetter()
	minerAddr1 := addrGetter()

	mockCid := types.CidFromString(t, "mock")

	parentBlock := &types.BlockHeader{Height: 42, Miner: minerAddr1, Messages: mockCid, ParentMessageReceipts: mockCid, ParentStateRoot: mockCid}
	parentTipSet := block.RequireNewTipSet(t, parentBlock)

	block1 := &types.BlockHeader{Miner: minerAddr1, Height: 43, ParentStateRoot: types.CidFromString(t, "some-state"), Messages: mockCid, ParentMessageReceipts: mockCid}
	block2 := &types.BlockHeader{Miner: minerAddr1, Height: 43, ParentStateRoot: types.CidFromString(t, "some-other-state"), Messages: mockCid, ParentMessageReceipts: mockCid}

	faultCh := make(chan ConsensusFault, 1)
	cfd := NewConsensusFaultDetector(faultCh)
	assert.NoError(t, cfd.CheckBlock(block1, parentTipSet))
	assertEmptyCh(t, faultCh) // no collision here because index is empty
	assert.NoError(t, cfd.CheckBlock(block2, parentTipSet))
	fault := <-faultCh
	assert.Equal(t, fault.Block1, block2)
	assert.Equal(t, fault.Block2, block1)
}

func TestFaultNullBlocks(t *testing.T) {
	tf.UnitTest(t)
	addrGetter := types.NewForTestGetter()
	minerAddr1 := addrGetter()

	mockCid := types.CidFromString(t, "mock")

	t.Run("same base", func(t *testing.T) {
		parentBlock := &types.BlockHeader{Height: 42, Miner: minerAddr1, Messages: mockCid, ParentMessageReceipts: mockCid, ParentStateRoot: mockCid}
		parentTipSet := block.RequireNewTipSet(t, parentBlock)

		block1 := &types.BlockHeader{Miner: minerAddr1, Height: 45, Messages: mockCid, ParentMessageReceipts: mockCid, ParentStateRoot: mockCid}
		block2 := &types.BlockHeader{Miner: minerAddr1, Height: 49, Messages: mockCid, ParentMessageReceipts: mockCid, ParentStateRoot: mockCid}

		faultCh := make(chan ConsensusFault, 3)
		cfd := NewConsensusFaultDetector(faultCh)
		assert.NoError(t, cfd.CheckBlock(block1, parentTipSet))
		assertEmptyCh(t, faultCh)
		assert.NoError(t, cfd.CheckBlock(block2, parentTipSet))
		for i := 0; i < 3; i++ {
			fault := <-faultCh
			assert.Equal(t, fault.Block1, block2)
			assert.Equal(t, fault.Block2, block1)
		}
	})
}

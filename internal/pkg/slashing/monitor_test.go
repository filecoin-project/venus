package slashing_test

import (
	"github.com/filecoin-project/venus/internal/pkg/enccid"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/internal/pkg/block"
	. "github.com/filecoin-project/venus/internal/pkg/slashing"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/internal/pkg/types"
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

	t.Run("blocks mined by different miners don't slash", func(t *testing.T) {
		parentBlock := &block.Block{Height: 42}
		parentTipSet := block.RequireNewTipSet(t, parentBlock)

		block1 := &block.Block{Miner: minerAddr1, Height: 43}
		block2 := &block.Block{Miner: minerAddr2, Height: 43}
		block3 := &block.Block{Miner: minerAddr3, Height: 43}

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
		parent1Block := &block.Block{Height: 42}
		parent1TipSet := block.RequireNewTipSet(t, parent1Block)
		block1 := &block.Block{Miner: minerAddr1, Height: 43}

		parent2Block := &block.Block{Height: 55}
		parent2TipSet := block.RequireNewTipSet(t, parent2Block)
		block2 := &block.Block{Miner: minerAddr1, Height: 56}

		faultCh := make(chan ConsensusFault, 1)
		cfd := NewConsensusFaultDetector(faultCh)
		assert.NoError(t, cfd.CheckBlock(block1, parent1TipSet))
		assertEmptyCh(t, faultCh)
		assert.NoError(t, cfd.CheckBlock(block2, parent2TipSet))
		assertEmptyCh(t, faultCh)
	})

	t.Run("blocks with non-overlapping null intervals don't slash", func(t *testing.T) {
		parent1Block := &block.Block{Height: 42}
		parent1TipSet := block.RequireNewTipSet(t, parent1Block)
		block1 := &block.Block{Miner: minerAddr1, Height: 46}

		parent2TipSet := block.RequireNewTipSet(t, block1)
		block2 := &block.Block{Miner: minerAddr1, Height: 56}

		faultCh := make(chan ConsensusFault, 1)
		cfd := NewConsensusFaultDetector(faultCh)
		assert.NoError(t, cfd.CheckBlock(block1, parent1TipSet))
		assertEmptyCh(t, faultCh)
		assert.NoError(t, cfd.CheckBlock(block2, parent2TipSet))
		assertEmptyCh(t, faultCh)
	})

	t.Run("duplicate equal blocks don't slash", func(t *testing.T) {
		parentBlock := &block.Block{Height: 42}
		parentTipSet := block.RequireNewTipSet(t, parentBlock)

		block := &block.Block{Miner: minerAddr1, Height: 43}
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

	parentBlock := &block.Block{Height: 42}
	parentTipSet := block.RequireNewTipSet(t, parentBlock)

	block1 := &block.Block{Miner: minerAddr1, Height: 43, StateRoot: enccid.NewCid(types.CidFromString(t, "some-state"))}
	block2 := &block.Block{Miner: minerAddr1, Height: 43, StateRoot: enccid.NewCid(types.CidFromString(t, "some-other-state"))}

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

	t.Run("same base", func(t *testing.T) {
		parentBlock := &block.Block{Height: 42}
		parentTipSet := block.RequireNewTipSet(t, parentBlock)

		block1 := &block.Block{Miner: minerAddr1, Height: 45}
		block2 := &block.Block{Miner: minerAddr1, Height: 49}

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

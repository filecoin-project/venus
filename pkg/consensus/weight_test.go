// stm: #unit
package consensus_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/venus/pkg/testhelpers"

	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/go-state-types/abi"
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/consensus"
	appstate "github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/state/tree"
)

func TestWeight(t *testing.T) {
	cst := cbor.NewMemCborStore()
	ctx := context.Background()
	fakeTree := tree.NewFromString(t, "test-Weight-StateCid", cst)
	fakeRoot, err := fakeTree.Flush(ctx)
	require.NoError(t, err)
	addrGetter := testhelpers.NewForTestGetter()
	minerAddr := addrGetter()
	// We only care about total power for the weight function
	// Total is 16, so bitlen is 5, log2b is 4
	viewer := makeStateViewer(fakeRoot, abi.NewStoragePower(16))
	ticket := consensus.MakeFakeTicketForTest()
	toWeigh := testhelpers.RequireNewTipSet(t, &types.BlockHeader{
		Miner:        minerAddr,
		ParentWeight: fbig.Zero(),
		Ticket:       ticket,
		ElectionProof: &types.ElectionProof{
			WinCount: 1,
		},
		ParentStateRoot:       fakeRoot,
		Messages:              testhelpers.EmptyMessagesCID,
		ParentMessageReceipts: testhelpers.EmptyReceiptsCID,
	})

	sel := consensus.NewChainSelector(cst, &viewer)
	// sel := consensus.NewChainSelector(cst, &viewer, types.CidFromString(t, "genesisCid"))

	t.Run("basic happy path", func(t *testing.T) {
		// 0 + (4*256 + (4*1*1*256/5*2))
		// 1024 + 102 = 1126
		// stm: @CONSENSUS_CHAIN_SELECTOR_WEIGHT_001
		w, err := sel.Weight(ctx, toWeigh)
		// w, err := sel.Weight(ctx, toWeigh, fakeRoot)
		assert.NoError(t, err)
		assert.Equal(t, fbig.NewInt(1126), w)
	})

	t.Run("total power adjusts as expected", func(t *testing.T) {
		asLowerX := makeStateViewer(fakeRoot, abi.NewStoragePower(15))
		asSameX := makeStateViewer(fakeRoot, abi.NewStoragePower(31))
		asHigherX := makeStateViewer(fakeRoot, abi.NewStoragePower(32))

		// 0 + (3*256) + (3*1*1*256/2*5) = 844 (truncating not rounding division)
		selLower := consensus.NewChainSelector(cst, &asLowerX)
		fixWeight, err := selLower.Weight(ctx, toWeigh)
		assert.NoError(t, err)
		assert.Equal(t, fbig.NewInt(844), fixWeight)

		// Weight is same when total bytes = 16 as when total bytes = 31
		selSame := consensus.NewChainSelector(cst, &asSameX)
		fixWeight, err = selSame.Weight(ctx, toWeigh)
		assert.NoError(t, err)
		assert.Equal(t, fbig.NewInt(1126), fixWeight)

		// 0 + (5*256) + (5*1*1*256/2*5) = 1408
		selHigher := consensus.NewChainSelector(cst, &asHigherX)
		fixWeight, err = selHigher.Weight(ctx, toWeigh)
		assert.NoError(t, err)
		assert.Equal(t, fbig.NewInt(1408), fixWeight)
	})

	t.Run("non-zero parent weight", func(t *testing.T) {
		parentWeight := fbig.NewInt(int64(49))
		toWeighWithParent := testhelpers.RequireNewTipSet(t, &types.BlockHeader{
			Miner:        minerAddr,
			ParentWeight: parentWeight,
			Ticket:       ticket,
			ElectionProof: &types.ElectionProof{
				WinCount: 1,
			},
			ParentStateRoot:       fakeRoot,
			Messages:              testhelpers.EmptyMessagesCID,
			ParentMessageReceipts: testhelpers.EmptyReceiptsCID,
		})

		// 49 + (4*256) + (4*1*1*256/2*5) = 1175
		w, err := sel.Weight(ctx, toWeighWithParent)
		assert.NoError(t, err)
		assert.Equal(t, fbig.NewInt(1175), w)
	})

	t.Run("many blocks", func(t *testing.T) {
		toWeighThreeBlock := testhelpers.RequireNewTipSet(t,
			&types.BlockHeader{
				Miner:        minerAddr,
				ParentWeight: fbig.Zero(),
				Ticket:       ticket,
				Timestamp:    0,
				ElectionProof: &types.ElectionProof{
					WinCount: 1,
				},
				ParentStateRoot:       fakeRoot,
				Messages:              testhelpers.EmptyMessagesCID,
				ParentMessageReceipts: testhelpers.EmptyReceiptsCID,
			},
			&types.BlockHeader{
				Miner:        minerAddr,
				ParentWeight: fbig.Zero(),
				Ticket:       ticket,
				Timestamp:    1,
				ElectionProof: &types.ElectionProof{
					WinCount: 1,
				},
				ParentStateRoot:       fakeRoot,
				Messages:              testhelpers.EmptyMessagesCID,
				ParentMessageReceipts: testhelpers.EmptyReceiptsCID,
			},
			&types.BlockHeader{
				Miner:        minerAddr,
				ParentWeight: fbig.Zero(),
				Ticket:       ticket,
				Timestamp:    2,
				ElectionProof: &types.ElectionProof{
					WinCount: 1,
				},
				ParentStateRoot:       fakeRoot,
				Messages:              testhelpers.EmptyMessagesCID,
				ParentMessageReceipts: testhelpers.EmptyReceiptsCID,
			},
		)
		// 0 + (4*256) + (4*3*1*256/2*5) = 1331
		w, err := sel.Weight(ctx, toWeighThreeBlock)
		assert.NoError(t, err)
		assert.Equal(t, fbig.NewInt(1331), w)

		// stm: @CONSENSUS_CHAIN_SELECTOR_WEIGHT_001
		toWeighTwoBlock := testhelpers.RequireNewTipSet(t, toWeighThreeBlock.At(0), toWeighThreeBlock.At(1))
		isHeavier, err := sel.IsHeavier(ctx, toWeighThreeBlock, toWeighTwoBlock)
		assert.NoError(t, err)
		assert.True(t, isHeavier)
	})
}

func makeStateViewer(stateRoot cid.Cid, networkPower abi.StoragePower) consensus.FakeConsensusStateViewer {
	return consensus.FakeConsensusStateViewer{
		Views: map[cid.Cid]*appstate.FakeStateView{
			stateRoot: appstate.NewFakeStateView(networkPower, networkPower, 0, 0),
		},
	}
}

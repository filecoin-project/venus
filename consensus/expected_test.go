package consensus_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-exchange-offline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

func TestNewExpected(t *testing.T) {
	tf.UnitTest(t)

	t.Run("a new Expected can be created", func(t *testing.T) {
		cst, bstore := setupCborBlockstore()
		as := consensus.NewFakeActorStateStore(types.NewBytesAmount(1), types.NewBytesAmount(5), make(map[address.Address]address.Address))
		exp := consensus.NewExpected(cst, bstore, consensus.NewDefaultProcessor(), th.NewFakeBlockValidator(), as, types.CidFromString(t, "somecid"), th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{})
		assert.NotNil(t, exp)
	})
}

func requireNewValidTestBlock(t *testing.T, baseTipSet types.TipSet, stateRootCid cid.Cid, height uint64, minerAddr address.Address, minerWorker address.Address, signer types.Signer) *types.Block {
	b, err := th.NewValidTestBlockFromTipSet(baseTipSet, stateRootCid, height, minerAddr, minerWorker, signer)
	require.NoError(t, err)
	return b
}

// requireMakeBlocks sets up 3 blocks with 3 owner actors and 3 miner actors and puts them in the state tree.
// the owner actors have associated mockSigners for signing blocks and tickets.
func requireMakeBlocks(ctx context.Context, t *testing.T, pTipSet types.TipSet, tree state.Tree, vms vm.StorageMap) ([]*types.Block, map[address.Address]address.Address) {
	// make a set of owner keypairs so they can sign blocks
	mockSigner, kis := types.NewMockSignersAndKeyInfo(3)

	// iterate over the keypairs and set up owner actors and miner actors with their own addresses
	// and add them to the state tree
	minerWorkers := make([]address.Address, 3)
	minerAddrs := make([]address.Address, 3)
	minerToWorker := make(map[address.Address]address.Address)
	for i, name := range kis {
		addr, err := kis[i].Address()
		require.NoError(t, err)

		minerWorkers[i], err = kis[i].Address()
		require.NoError(t, err)

		ownerActor := th.RequireNewAccountActor(t, types.ZeroAttoFIL)
		require.NoError(t, tree.SetActor(ctx, addr, ownerActor))

		minerAddrs[i], err = address.NewActorAddress([]byte(fmt.Sprintf("%s%s", name, "Miner")))
		require.NoError(t, err)

		minerToWorker[minerAddrs[i]] = minerWorkers[i]

		minerActor := th.RequireNewMinerActor(t, vms, minerAddrs[i], addr,
			10000, th.RequireRandomPeerID(t), types.ZeroAttoFIL)
		require.NoError(t, tree.SetActor(ctx, minerAddrs[i], minerActor))
	}
	stateRoot, err := tree.Flush(ctx)
	require.NoError(t, err)

	blocks := []*types.Block{
		requireNewValidTestBlock(t, pTipSet, stateRoot, 1, minerAddrs[0], minerWorkers[0], mockSigner),
		requireNewValidTestBlock(t, pTipSet, stateRoot, 1, minerAddrs[1], minerWorkers[1], mockSigner),
		requireNewValidTestBlock(t, pTipSet, stateRoot, 1, minerAddrs[2], minerWorkers[2], mockSigner),
	}
	return blocks, minerToWorker
}

// TestExpected_RunStateTransition_validateMining is concerned only with validateMining behavior.
// Fully unit-testing RunStateTransition is difficult due to this requiring that you
// completely set up a valid state tree with a valid matching TipSet.  RunStateTransition is tested
// with integration tests (see chain_daemon_test.go for example)
func TestExpected_RunStateTransition_validateMining(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	cistore, bstore := setupCborBlockstore()
	genesisBlock, err := th.DefaultGenesis(cistore, bstore)
	require.NoError(t, err)
	minerPower := types.NewBytesAmount(1)
	totalPower := types.NewBytesAmount(1)

	t.Run("passes the validateMining section when given valid mining blocks", func(t *testing.T) {
		pTipSet := types.RequireNewTipSet(t, genesisBlock)
		stateTree, err := state.LoadStateTree(ctx, cistore, genesisBlock.StateRoot)
		require.NoError(t, err)
		vms := vm.NewStorageMap(bstore)

		blocks, minerToWorker := requireMakeBlocks(ctx, t, pTipSet, stateTree, vms)

		tipSet := types.RequireNewTipSet(t, blocks...)
		// Add the miner worker mapping into the actor state
		as := consensus.NewFakeActorStateStore(minerPower, totalPower, minerToWorker)

		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), th.NewFakeBlockValidator(), as, genesisBlock.Cid(), th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{})

		var emptyMessages [][]*types.SignedMessage
		var emptyReceipts [][]*types.MessageReceipt
		for i := 0; i < len(blocks); i++ {
			emptyMessages = append(emptyMessages, []*types.SignedMessage{})
			emptyReceipts = append(emptyReceipts, []*types.MessageReceipt{})
		}

		_, err = exp.RunStateTransition(ctx, tipSet, emptyMessages, emptyReceipts, []types.TipSet{pTipSet}, 0, blocks[0].StateRoot)
		assert.NoError(t, err)
	})

	t.Run("returns nil + mining error when election proof validation fails", func(t *testing.T) {
		pTipSet := types.RequireNewTipSet(t, genesisBlock)

		stateTree, err := state.LoadStateTree(ctx, cistore, genesisBlock.StateRoot)
		require.NoError(t, err)

		vms := vm.NewStorageMap(bstore)

		blocks, minerToWorker := requireMakeBlocks(ctx, t, pTipSet, stateTree, vms)

		as := consensus.NewFakeActorStateStore(minerPower, totalPower, minerToWorker)
		exp := consensus.NewExpected(cistore, bstore, consensus.NewDefaultProcessor(), th.NewFakeBlockValidator(), as, types.CidFromString(t, "somecid"), th.BlockTimeTest, &consensus.FailingElectionValidator{}, &consensus.FakeTicketMachine{})

		tipSet := types.RequireNewTipSet(t, blocks...)

		var emptyMessages [][]*types.SignedMessage
		var emptyReceipts [][]*types.MessageReceipt
		for i := 0; i < len(blocks); i++ {
			emptyMessages = append(emptyMessages, []*types.SignedMessage{})
			emptyReceipts = append(emptyReceipts, []*types.MessageReceipt{})
		}

		_, err = exp.RunStateTransition(ctx, tipSet, emptyMessages, emptyReceipts, []types.TipSet{pTipSet}, 0, genesisBlock.StateRoot)
		assert.EqualError(t, err, "block author did not win election")
	})

	t.Run("returns nil + mining error when ticket validation fails", func(t *testing.T) {

		pTipSet := types.RequireNewTipSet(t, genesisBlock)

		stateTree, err := state.LoadStateTree(ctx, cistore, genesisBlock.StateRoot)
		require.NoError(t, err)

		vms := vm.NewStorageMap(bstore)

		blocks, minerToWorker := requireMakeBlocks(ctx, t, pTipSet, stateTree, vms)

		as := consensus.NewFakeActorStateStore(minerPower, totalPower, minerToWorker)
		exp := consensus.NewExpected(cistore, bstore, consensus.NewDefaultProcessor(), th.NewFakeBlockValidator(), as, types.CidFromString(t, "somecid"), th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FailingTicketValidator{})

		tipSet := types.RequireNewTipSet(t, blocks...)

		var emptyMessages [][]*types.SignedMessage
		var emptyReceipts [][]*types.MessageReceipt
		for i := 0; i < len(blocks); i++ {
			emptyMessages = append(emptyMessages, []*types.SignedMessage{})
			emptyReceipts = append(emptyReceipts, []*types.MessageReceipt{})
		}

		_, err = exp.RunStateTransition(ctx, tipSet, emptyMessages, emptyReceipts, []types.TipSet{pTipSet}, 0, genesisBlock.StateRoot)
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "invalid ticket")
		assert.Contains(t, err.Error(), "position 0")
	})

	t.Run("fails when ticket array length inconsistent with block height", func(t *testing.T) {
		pTipSet := types.RequireNewTipSet(t, genesisBlock)

		stateTree, err := state.LoadStateTree(ctx, cistore, genesisBlock.StateRoot)
		require.NoError(t, err)
		vms := vm.NewStorageMap(bstore)

		blocks, minerToWorker := requireMakeBlocks(ctx, t, pTipSet, stateTree, vms)
		// change ticket array length but not height
		blocks[0].Tickets = append(blocks[0].Tickets, consensus.MakeFakeTicketForTest())
		tipSet := types.RequireNewTipSet(t, blocks[0])

		as := consensus.NewFakeActorStateStore(minerPower, totalPower, minerToWorker)
		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), th.NewFakeBlockValidator(), as, genesisBlock.Cid(), th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{})

		var emptyMessages [][]*types.SignedMessage
		var emptyReceipts [][]*types.MessageReceipt
		emptyMessages = append(emptyMessages, []*types.SignedMessage{})
		emptyReceipts = append(emptyReceipts, []*types.MessageReceipt{})

		_, err = exp.RunStateTransition(ctx, tipSet, emptyMessages, emptyReceipts, []types.TipSet{pTipSet}, 0, blocks[0].StateRoot)
		assert.Error(t, err)
	})

	t.Run("returns nil + mining error when signature is invalid", func(t *testing.T) {
		pTipSet := types.RequireNewTipSet(t, genesisBlock)
		stateTree, err := state.LoadStateTree(ctx, cistore, genesisBlock.StateRoot)
		require.NoError(t, err)
		vms := vm.NewStorageMap(bstore)

		blocks, minerToWorker := requireMakeBlocks(ctx, t, pTipSet, stateTree, vms)
		// Give block 0 an invalid signature
		blocks[0].BlockSig = blocks[1].BlockSig

		tipSet := types.RequireNewTipSet(t, blocks...)
		as := consensus.NewFakeActorStateStore(minerPower, totalPower, minerToWorker)

		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), th.NewFakeBlockValidator(), as, genesisBlock.Cid(), th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{})

		var emptyMessages [][]*types.SignedMessage
		var emptyReceipts [][]*types.MessageReceipt
		for i := 0; i < len(blocks); i++ {
			emptyMessages = append(emptyMessages, []*types.SignedMessage{})
			emptyReceipts = append(emptyReceipts, []*types.MessageReceipt{})
		}

		_, err = exp.RunStateTransition(ctx, tipSet, emptyMessages, emptyReceipts, []types.TipSet{pTipSet}, 0, blocks[0].StateRoot)
		assert.EqualError(t, err, "block signature invalid")
	})
}

func setupCborBlockstore() (*hamt.CborIpldStore, blockstore.Blockstore) {
	mds := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(mds)
	offl := offline.Exchange(bs)
	blkserv := blockservice.New(bs, offl)
	cis := &hamt.CborIpldStore{Blocks: blkserv}

	return cis, bs
}

package consensus_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-exchange-offline"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExpected(t *testing.T) {
	tf.UnitTest(t)

	t.Run("a new Expected can be created", func(t *testing.T) {
		cst, bstore := setupCborBlockstore()
		ptv := th.NewTestPowerTableView(types.NewBytesAmount(1), types.NewBytesAmount(5))
		exp := consensus.NewExpected(cst, bstore, consensus.NewDefaultProcessor(), th.NewFakeBlockValidator(), ptv, types.CidFromString(t, "somecid"), th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{})
		assert.NotNil(t, exp)
	})
}

// requireMakeBlocks sets up 3 blocks with 3 owner actors and 3 miner actors and puts them in the state tree.
// the owner actors have associated mockSigners for signing blocks (not implemented yet) and tickets.
func requireMakeBlocks(ctx context.Context, t *testing.T, pTipSet types.TipSet, tree state.Tree, vms vm.StorageMap) []*types.Block {
	// make  a set of owner keypairs so they can sign blocks
	mockSigner, kis := types.NewMockSignersAndKeyInfo(3)

	// iterate over the keypairs and set up owner actors and miner actors with their own addresses
	// and add them to the state tree
	minerWorkers := make([]address.Address, 3)
	minerAddrs := make([]address.Address, 3)
	for i, name := range kis {
		addr, err := kis[i].Address()
		require.NoError(t, err)

		minerWorkers[i], err = kis[i].Address()
		require.NoError(t, err)

		ownerActor := th.RequireNewAccountActor(t, types.ZeroAttoFIL)
		require.NoError(t, tree.SetActor(ctx, addr, ownerActor))

		minerAddrs[i], err = address.NewActorAddress([]byte(fmt.Sprintf("%s%s", name, "Miner")))
		require.NoError(t, err)
		minerActor := th.RequireNewMinerActor(t, vms, minerAddrs[i], addr,
			10000, th.RequireRandomPeerID(t), types.ZeroAttoFIL)
		require.NoError(t, tree.SetActor(ctx, minerAddrs[i], minerActor))
	}
	stateRoot, err := tree.Flush(ctx)
	require.NoError(t, err)

	blocks := []*types.Block{
		th.NewValidTestBlockFromTipSet(pTipSet, stateRoot, 1, minerAddrs[0], minerWorkers[0], mockSigner),
		th.NewValidTestBlockFromTipSet(pTipSet, stateRoot, 1, minerAddrs[1], minerWorkers[1], mockSigner),
		th.NewValidTestBlockFromTipSet(pTipSet, stateRoot, 1, minerAddrs[2], minerWorkers[2], mockSigner),
	}
	return blocks
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

		ptv := th.NewTestPowerTableView(minerPower, totalPower)
		exp := consensus.NewExpected(cistore, bstore, th.NewTestProcessor(), th.NewFakeBlockValidator(), ptv, genesisBlock.Cid(), th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{})

		pTipSet := types.RequireNewTipSet(t, genesisBlock)

		stateTree, err := state.LoadStateTree(ctx, cistore, genesisBlock.StateRoot, builtin.Actors)
		require.NoError(t, err)
		vms := vm.NewStorageMap(bstore)

		blocks := requireMakeBlocks(ctx, t, pTipSet, stateTree, vms)

		tipSet := types.RequireNewTipSet(t, blocks...)

		var emptyMessages [][]*types.SignedMessage
		var emptyReceipts [][]*types.MessageReceipt
		for i := 0; i < len(blocks); i++ {
			emptyMessages = append(emptyMessages, []*types.SignedMessage{})
			emptyReceipts = append(emptyReceipts, []*types.MessageReceipt{})
		}

		_, err = exp.RunStateTransition(ctx, tipSet, emptyMessages, emptyReceipts, []types.TipSet{pTipSet}, blocks[0].StateRoot)
		assert.NoError(t, err)
	})

	t.Run("returns nil + mining error when election proof validation fails", func(t *testing.T) {
		ptv := th.NewTestPowerTableView(minerPower, totalPower)
		exp := consensus.NewExpected(cistore, bstore, consensus.NewDefaultProcessor(), th.NewFakeBlockValidator(), ptv, types.CidFromString(t, "somecid"), th.BlockTimeTest, &consensus.FailingElectionValidator{}, &consensus.FakeTicketMachine{})

		pTipSet := types.RequireNewTipSet(t, genesisBlock)

		stateTree, err := state.LoadStateTree(ctx, cistore, genesisBlock.StateRoot, builtin.Actors)
		require.NoError(t, err)

		vms := vm.NewStorageMap(bstore)

		blocks := requireMakeBlocks(ctx, t, pTipSet, stateTree, vms)

		tipSet := types.RequireNewTipSet(t, blocks...)

		var emptyMessages [][]*types.SignedMessage
		var emptyReceipts [][]*types.MessageReceipt
		for i := 0; i < len(blocks); i++ {
			emptyMessages = append(emptyMessages, []*types.SignedMessage{})
			emptyReceipts = append(emptyReceipts, []*types.MessageReceipt{})
		}

		_, err = exp.RunStateTransition(ctx, tipSet, emptyMessages, emptyReceipts, []types.TipSet{pTipSet}, genesisBlock.StateRoot)
		assert.EqualError(t, err, "block author did not win election")
	})

	t.Run("returns nil + mining error when ticket validation fails", func(t *testing.T) {
		ptv := th.NewTestPowerTableView(minerPower, totalPower)
		exp := consensus.NewExpected(cistore, bstore, consensus.NewDefaultProcessor(), th.NewFakeBlockValidator(), ptv, types.CidFromString(t, "somecid"), th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FailingTicketValidator{})

		pTipSet := types.RequireNewTipSet(t, genesisBlock)

		stateTree, err := state.LoadStateTree(ctx, cistore, genesisBlock.StateRoot, builtin.Actors)
		require.NoError(t, err)

		vms := vm.NewStorageMap(bstore)

		blocks := requireMakeBlocks(ctx, t, pTipSet, stateTree, vms)

		tipSet := types.RequireNewTipSet(t, blocks...)

		var emptyMessages [][]*types.SignedMessage
		var emptyReceipts [][]*types.MessageReceipt
		for i := 0; i < len(blocks); i++ {
			emptyMessages = append(emptyMessages, []*types.SignedMessage{})
			emptyReceipts = append(emptyReceipts, []*types.MessageReceipt{})
		}

		_, err = exp.RunStateTransition(ctx, tipSet, emptyMessages, emptyReceipts, []types.TipSet{pTipSet}, genesisBlock.StateRoot)
		assert.EqualError(t, err, "invalid ticket extension")
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

type FailingTestPowerTableView struct{ minerPower, totalPower *types.BytesAmount }

func NewFailingTestPowerTableView(minerPower, totalPower *types.BytesAmount) *FailingTestPowerTableView {
	return &FailingTestPowerTableView{minerPower: minerPower, totalPower: totalPower}
}

func (tv *FailingTestPowerTableView) Total(ctx context.Context, st state.Tree, bstore blockstore.Blockstore) (*types.BytesAmount, error) {
	return tv.totalPower, errors.New("something went wrong with the total power")
}

func (tv *FailingTestPowerTableView) Miner(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) (*types.BytesAmount, error) {
	return tv.minerPower, nil
}

func (tv *FailingTestPowerTableView) WorkerAddr(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) (address.Address, error) {
	return mAddr, nil
}

func (tv *FailingTestPowerTableView) HasPower(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) bool {
	return true
}

type FailingMinerTestPowerTableView struct{ minerPower, totalPower *types.BytesAmount }

func NewFailingMinerTestPowerTableView(minerPower, totalPower *types.BytesAmount) *FailingMinerTestPowerTableView {
	return &FailingMinerTestPowerTableView{minerPower: minerPower, totalPower: totalPower}
}

func (tv *FailingMinerTestPowerTableView) Total(ctx context.Context, st state.Tree, bstore blockstore.Blockstore) (*types.BytesAmount, error) {
	return tv.totalPower, nil
}

func (tv *FailingMinerTestPowerTableView) Miner(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) (*types.BytesAmount, error) {
	return tv.minerPower, errors.New("something went wrong with the miner power")
}

func (tv *FailingMinerTestPowerTableView) WorkerAddr(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) (address.Address, error) {
	return mAddr, nil
}

func (tv *FailingMinerTestPowerTableView) HasPower(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) bool {
	return true
}

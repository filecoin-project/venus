package consensus_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-bls-sigs"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-exchange-offline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
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

func requireNewValidTestBlock(t *testing.T, baseTipSet block.TipSet, stateRootCid cid.Cid, height uint64, minerAddr address.Address, minerWorker address.Address, signer types.Signer) *block.Block {
	b, err := th.NewValidTestBlockFromTipSet(baseTipSet, stateRootCid, height, minerAddr, minerWorker, signer)
	require.NoError(t, err)
	return b
}

// requireMakeBlocks sets up 3 blocks with 3 owner actors and 3 miner actors and puts them in the state tree.
// the owner actors have associated mockSigners for signing blocks and tickets.
func requireMakeBlocks(ctx context.Context, t *testing.T, pTipSet block.TipSet, tree state.Tree, vms vm.StorageMap) ([]*block.Block, map[address.Address]address.Address) {
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

	blocks := []*block.Block{
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
		pTipSet := th.RequireNewTipSet(t, genesisBlock)
		stateTree, err := state.LoadStateTree(ctx, cistore, genesisBlock.StateRoot)
		require.NoError(t, err)
		vms := vm.NewStorageMap(bstore)

		blocks, minerToWorker := requireMakeBlocks(ctx, t, pTipSet, stateTree, vms)

		tipSet := th.RequireNewTipSet(t, blocks...)
		// Add the miner worker mapping into the actor state
		as := consensus.NewFakeActorStateStore(minerPower, totalPower, minerToWorker)

		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), th.NewFakeBlockValidator(), as, genesisBlock.Cid(), th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{})

		emptyBLSMessages, emptyMessages, emptyReceipts := emptyMessagesAndReceipts(len(blocks))

		_, err = exp.RunStateTransition(ctx, tipSet, emptyBLSMessages, emptyMessages, emptyReceipts, []block.TipSet{pTipSet}, 0, blocks[0].StateRoot)
		assert.NoError(t, err)
	})

	t.Run("returns nil + mining error when election proof validation fails", func(t *testing.T) {
		pTipSet := th.RequireNewTipSet(t, genesisBlock)

		stateTree, err := state.LoadStateTree(ctx, cistore, genesisBlock.StateRoot)
		require.NoError(t, err)

		vms := vm.NewStorageMap(bstore)

		blocks, minerToWorker := requireMakeBlocks(ctx, t, pTipSet, stateTree, vms)

		as := consensus.NewFakeActorStateStore(minerPower, totalPower, minerToWorker)
		exp := consensus.NewExpected(cistore, bstore, consensus.NewDefaultProcessor(), th.NewFakeBlockValidator(), as, types.CidFromString(t, "somecid"), th.BlockTimeTest, &consensus.FailingElectionValidator{}, &consensus.FakeTicketMachine{})

		tipSet := th.RequireNewTipSet(t, blocks...)

		emptyBLSMessages, emptyMessages, emptyReceipts := emptyMessagesAndReceipts(len(blocks))

		_, err = exp.RunStateTransition(ctx, tipSet, emptyBLSMessages, emptyMessages, emptyReceipts, []block.TipSet{pTipSet}, 0, genesisBlock.StateRoot)
		assert.EqualError(t, err, "block author did not win election")
	})

	t.Run("fails when bls signature is not valid across bls messages", func(t *testing.T) {
		pTipSet := th.RequireNewTipSet(t, genesisBlock)
		stateTree, err := state.LoadStateTree(ctx, cistore, genesisBlock.StateRoot)
		require.NoError(t, err)
		vms := vm.NewStorageMap(bstore)

		blocks, minerToWorker := requireMakeBlocks(ctx, t, pTipSet, stateTree, vms)

		tipSet := th.RequireNewTipSet(t, blocks...)
		// Add the miner worker mapping into the actor state
		as := consensus.NewFakeActorStateStore(minerPower, totalPower, minerToWorker)

		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), th.NewFakeBlockValidator(), as, genesisBlock.Cid(), th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{})

		_, emptyMessages, emptyReceipts := emptyMessagesAndReceipts(len(blocks))

		// Create BLS messages but do not update signature
		blsKey := bls.PrivateKeyPublicKey(bls.PrivateKeyGenerate())
		blsAddr, err := address.NewBLSAddress(blsKey[:])
		require.NoError(t, err)

		blsMessages := make([][]*types.UnsignedMessage, tipSet.Len())
		msg := types.NewUnsignedMessage(blsAddr, address.TestAddress2, 0, types.NewAttoFILFromFIL(0), "", []byte{})
		blsMessages[0] = append(blsMessages[0], msg)

		_, err = exp.RunStateTransition(ctx, tipSet, blsMessages, emptyMessages, emptyReceipts, []block.TipSet{pTipSet}, 0, blocks[0].StateRoot)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "block BLS signature does not validate")
	})

	t.Run("fails when secp message has invalid signature", func(t *testing.T) {
		pTipSet := th.RequireNewTipSet(t, genesisBlock)
		stateTree, err := state.LoadStateTree(ctx, cistore, genesisBlock.StateRoot)
		require.NoError(t, err)
		vms := vm.NewStorageMap(bstore)

		blocks, minerToWorker := requireMakeBlocks(ctx, t, pTipSet, stateTree, vms)

		tipSet := th.RequireNewTipSet(t, blocks...)
		// Add the miner worker mapping into the actor state
		as := consensus.NewFakeActorStateStore(minerPower, totalPower, minerToWorker)

		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), th.NewFakeBlockValidator(), as, genesisBlock.Cid(), th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{})

		emptyBLSMessages, _, emptyReceipts := emptyMessagesAndReceipts(len(blocks))

		// Create secp message with invalid signature
		keys := types.MustGenerateKeyInfo(1, 42)
		blsAddr, err := address.NewSecp256k1Address(keys[0].PublicKey())
		require.NoError(t, err)

		secpMessages := make([][]*types.SignedMessage, tipSet.Len())
		msg := types.NewUnsignedMessage(blsAddr, address.TestAddress2, 0, types.NewAttoFILFromFIL(0), "", []byte{})
		smsg := &types.SignedMessage{
			Message:   *msg,
			Signature: []byte("not a signature"),
		}
		secpMessages[0] = append(secpMessages[0], smsg)

		_, err = exp.RunStateTransition(ctx, tipSet, emptyBLSMessages, secpMessages, emptyReceipts, []block.TipSet{pTipSet}, 0, blocks[0].StateRoot)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "secp message signature invalid")
	})

	t.Run("returns nil + mining error when ticket validation fails", func(t *testing.T) {

		pTipSet := th.RequireNewTipSet(t, genesisBlock)

		stateTree, err := state.LoadStateTree(ctx, cistore, genesisBlock.StateRoot)
		require.NoError(t, err)

		vms := vm.NewStorageMap(bstore)

		blocks, minerToWorker := requireMakeBlocks(ctx, t, pTipSet, stateTree, vms)

		as := consensus.NewFakeActorStateStore(minerPower, totalPower, minerToWorker)
		exp := consensus.NewExpected(cistore, bstore, consensus.NewDefaultProcessor(), th.NewFakeBlockValidator(), as, types.CidFromString(t, "somecid"), th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FailingTicketValidator{})

		tipSet := th.RequireNewTipSet(t, blocks...)

		emptyBLSMessages, emptyMessages, emptyReceipts := emptyMessagesAndReceipts(len(blocks))

		_, err = exp.RunStateTransition(ctx, tipSet, emptyBLSMessages, emptyMessages, emptyReceipts, []block.TipSet{pTipSet}, 0, genesisBlock.StateRoot)
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "invalid ticket")
	})

	t.Run("returns nil + mining error when signature is invalid", func(t *testing.T) {
		pTipSet := th.RequireNewTipSet(t, genesisBlock)
		stateTree, err := state.LoadStateTree(ctx, cistore, genesisBlock.StateRoot)
		require.NoError(t, err)
		vms := vm.NewStorageMap(bstore)

		blocks, minerToWorker := requireMakeBlocks(ctx, t, pTipSet, stateTree, vms)
		// Give block 0 an invalid signature
		blocks[0].BlockSig = blocks[1].BlockSig

		tipSet := th.RequireNewTipSet(t, blocks...)
		as := consensus.NewFakeActorStateStore(minerPower, totalPower, minerToWorker)

		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), th.NewFakeBlockValidator(), as, genesisBlock.Cid(), th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{})

		emptyBLSMessages, emptyMessages, emptyReceipts := emptyMessagesAndReceipts(len(blocks))

		_, err = exp.RunStateTransition(ctx, tipSet, emptyBLSMessages, emptyMessages, emptyReceipts, []block.TipSet{pTipSet}, 0, blocks[0].StateRoot)
		assert.EqualError(t, err, "block signature invalid")
	})
}

func emptyMessagesAndReceipts(numBlocks int) ([][]*types.UnsignedMessage, [][]*types.SignedMessage, [][]*types.MessageReceipt) {
	var emptyBLSMessages [][]*types.UnsignedMessage
	var emptyMessages [][]*types.SignedMessage
	var emptyReceipts [][]*types.MessageReceipt
	for i := 0; i < numBlocks; i++ {
		emptyBLSMessages = append(emptyBLSMessages, []*types.UnsignedMessage{})
		emptyMessages = append(emptyMessages, []*types.SignedMessage{})
		emptyReceipts = append(emptyReceipts, []*types.MessageReceipt{})
	}
	return emptyBLSMessages, emptyMessages, emptyReceipts
}

func setupCborBlockstore() (*hamt.CborIpldStore, blockstore.Blockstore) {
	mds := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(mds)
	offl := offline.Exchange(bs)
	blkserv := blockservice.New(bs, offl)
	cis := &hamt.CborIpldStore{Blocks: blkserv}

	return cis, bs
}

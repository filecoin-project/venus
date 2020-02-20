package consensus_test

import (
	"context"
	"strings"
	"testing"

	"github.com/filecoin-project/go-address"
	fbig "github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// TestExpected_RunStateTransition_validateMining is concerned only with validateMining behavior.
// Fully unit-testing RunStateTransition is difficult due to this requiring that you
// completely set up a valid state tree with a valid matching TipSet.  RunStateTransition is tested
// with integration tests (see chain_daemon_test.go for example)
func TestExpected_RunStateTransition_validateMining(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("requires VM to support state construction by messages")

	ctx := context.Background()
	mockSigner, kis := types.NewMockSignersAndKeyInfo(3)

	t.Run("passes the validateMining section when given valid mining blocks", func(t *testing.T) {
		cistore, bstore := setupCborBlockstore()
		genesisBlock, err := th.DefaultGenesis(cistore, bstore)
		require.NoError(t, err)

		//Set miner actor

		pTipSet := th.RequireNewTipSet(t, genesisBlock)
		nextRoot, miners, m2w := setTree(ctx, t, kis, cistore, bstore, genesisBlock.StateRoot.Cid)

		views := consensus.AsPowerStateViewer(appstate.NewViewer(cistore))
		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), &views, th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{}, &proofs.ElectionPoster{})

		nextBlocks := requireMakeNBlocks(t, 3, pTipSet, nextRoot, types.EmptyReceiptsCID, miners, m2w, mockSigner)
		tipSet := th.RequireNewTipSet(t, nextBlocks...)

		emptyBLSMessages, emptyMessages := emptyMessages(len(nextBlocks))
		_, _, err = exp.RunStateTransition(ctx, tipSet, emptyBLSMessages, emptyMessages, []block.TipSet{pTipSet}, nextBlocks[0].ParentWeight, nextBlocks[0].StateRoot.Cid, nextBlocks[0].MessageReceipts.Cid)
		assert.NoError(t, err)
	})

	t.Run("returns nil + mining error when election proof validation fails", func(t *testing.T) {
		cistore, bstore := setupCborBlockstore()
		genesisBlock, err := th.DefaultGenesis(cistore, bstore)
		require.NoError(t, err)

		pTipSet := th.RequireNewTipSet(t, genesisBlock)

		miners, minerToWorker := minerToWorkerFromAddrs(ctx, t, state.NewTree(cistore), vm.NewStorage(bstore), kis)
		views := consensus.AsPowerStateViewer(appstate.NewViewer(cistore))
		exp := consensus.NewExpected(cistore, bstore, consensus.NewDefaultProcessor(), &views, th.BlockTimeTest, &consensus.FailingElectionValidator{}, &consensus.FakeTicketMachine{}, &proofs.ElectionPoster{})

		nextBlocks := requireMakeNBlocks(t, 3, pTipSet, genesisBlock.StateRoot.Cid, types.EmptyReceiptsCID, miners, minerToWorker, mockSigner)
		tipSet := th.RequireNewTipSet(t, nextBlocks...)

		emptyBLSMessages, emptyMessages := emptyMessages(len(nextBlocks))

		_, _, err = exp.RunStateTransition(ctx, tipSet, emptyBLSMessages, emptyMessages, []block.TipSet{pTipSet}, nextBlocks[0].ParentWeight, genesisBlock.StateRoot.Cid, genesisBlock.MessageReceipts.Cid)
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "lost election"))
	})

	t.Run("correct tickets processed in election and next ticket", func(t *testing.T) {
		cistore, bstore := setupCborBlockstore()
		genesisBlock, err := th.DefaultGenesis(cistore, bstore)
		require.NoError(t, err)

		pTipSet := th.RequireNewTipSet(t, genesisBlock)

		nextRoot, miners, m2w := setTree(ctx, t, kis, cistore, bstore, genesisBlock.StateRoot.Cid)
		views := consensus.AsPowerStateViewer(appstate.NewViewer(cistore))
		ancestors := make([]block.TipSet, 5)
		for i := 0; i < int(miner.ElectionLookback); i++ {
			ancestorBlk := requireMakeNBlocks(t, 1, pTipSet, nextRoot, types.EmptyReceiptsCID, miners, m2w, mockSigner)
			ancestors[i] = th.RequireNewTipSet(t, ancestorBlk...)
			pTipSet = ancestors[i]
		}

		isLookingBack := func(ticket block.Ticket) {
			expTicket, err := ancestors[miner.ElectionLookback-1].MinTicket()
			require.NoError(t, err)
			assert.Equal(t, expTicket, ticket)
		}
		mockElection := consensus.NewMockElectionMachine(isLookingBack)

		isOneBack := func(ticket block.Ticket) {
			expTicket, err := ancestors[0].MinTicket()
			require.NoError(t, err)
			assert.Equal(t, expTicket, ticket)
		}
		mockTicketGen := consensus.NewMockTicketMachine(isOneBack)

		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), &views, th.BlockTimeTest, mockElection, mockTicketGen, &proofs.ElectionPoster{})

		nextBlocks := requireMakeNBlocks(t, 3, pTipSet, nextRoot, types.EmptyReceiptsCID, miners, m2w, mockSigner)
		tipSet := th.RequireNewTipSet(t, nextBlocks...)

		emptyBLSMessages, emptyMessages := emptyMessages(len(nextBlocks))
		_, _, err = exp.RunStateTransition(ctx, tipSet, emptyBLSMessages, emptyMessages, ancestors, nextBlocks[0].ParentWeight, nextRoot, nextBlocks[0].MessageReceipts.Cid)
		assert.NoError(t, err)
	})

	t.Run("fails when bls signature is not valid across bls messages", func(t *testing.T) {
		cistore, bstore := setupCborBlockstore()
		genesisBlock, err := th.DefaultGenesis(cistore, bstore)
		require.NoError(t, err)

		miners, minerToWorker := minerToWorkerFromAddrs(ctx, t, state.NewTree(cistore), vm.NewStorage(bstore), kis)
		views := consensus.AsPowerStateViewer(appstate.NewViewer(cistore))
		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), &views, th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{}, &proofs.ElectionPoster{})

		pTipSet := th.RequireNewTipSet(t, genesisBlock)
		nextBlocks := requireMakeNBlocks(t, 3, pTipSet, genesisBlock.StateRoot.Cid, types.EmptyReceiptsCID, miners, minerToWorker, mockSigner)
		tipSet := th.RequireNewTipSet(t, nextBlocks...)

		_, emptyMessages := emptyMessages(len(nextBlocks))

		// Create BLS messages but do not update signature
		blsKey := bls.PrivateKeyPublicKey(bls.PrivateKeyGenerate())
		blsAddr, err := address.NewBLSAddress(blsKey[:])
		require.NoError(t, err)

		blsMessages := make([][]*types.UnsignedMessage, tipSet.Len())
		msg := types.NewUnsignedMessage(blsAddr, vmaddr.TestAddress2, 0, types.NewAttoFILFromFIL(0), builtin.MethodSend, []byte{})
		blsMessages[0] = append(blsMessages[0], msg)

		_, _, err = exp.RunStateTransition(ctx, tipSet, blsMessages, emptyMessages, []block.TipSet{pTipSet}, nextBlocks[0].ParentWeight, nextBlocks[0].StateRoot.Cid, nextBlocks[0].MessageReceipts.Cid)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "block BLS signature does not validate")
	})

	t.Run("fails when secp message has invalid signature", func(t *testing.T) {
		cistore, bstore := setupCborBlockstore()
		genesisBlock, err := th.DefaultGenesis(cistore, bstore)
		require.NoError(t, err)

		miners, minerToWorker := minerToWorkerFromAddrs(ctx, t, state.NewTree(cistore), vm.NewStorage(bstore), kis)
		views := consensus.AsPowerStateViewer(appstate.NewViewer(cistore))
		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), &views, th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{}, &proofs.ElectionPoster{})

		pTipSet := th.RequireNewTipSet(t, genesisBlock)
		nextBlocks := requireMakeNBlocks(t, 3, pTipSet, genesisBlock.StateRoot.Cid, types.EmptyReceiptsCID, miners, minerToWorker, mockSigner)
		tipSet := th.RequireNewTipSet(t, nextBlocks...)

		emptyBLSMessages, _ := emptyMessages(len(nextBlocks))

		// Create secp message with invalid signature
		keys := types.MustGenerateKeyInfo(1, 42)
		blsAddr, err := address.NewSecp256k1Address(keys[0].PublicKey())
		require.NoError(t, err)

		secpMessages := make([][]*types.SignedMessage, tipSet.Len())
		msg := types.NewUnsignedMessage(blsAddr, vmaddr.TestAddress2, 0, types.NewAttoFILFromFIL(0), builtin.MethodSend, []byte{})
		smsg := &types.SignedMessage{
			Message:   *msg,
			Signature: []byte("not a signature"),
		}
		secpMessages[0] = append(secpMessages[0], smsg)

		_, _, err = exp.RunStateTransition(ctx, tipSet, emptyBLSMessages, secpMessages, []block.TipSet{pTipSet}, nextBlocks[0].ParentWeight, nextBlocks[0].StateRoot.Cid, nextBlocks[0].MessageReceipts.Cid)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "secp message signature invalid")
	})

	t.Run("returns nil + mining error when ticket validation fails", func(t *testing.T) {
		cistore, bstore := setupCborBlockstore()
		genesisBlock, err := th.DefaultGenesis(cistore, bstore)
		require.NoError(t, err)

		miners, minerToWorker := minerToWorkerFromAddrs(ctx, t, state.NewTree(cistore), vm.NewStorage(bstore), kis)
		views := consensus.AsPowerStateViewer(appstate.NewViewer(cistore))
		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), &views, th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FailingTicketValidator{}, &proofs.ElectionPoster{})

		pTipSet := th.RequireNewTipSet(t, genesisBlock)
		nextBlocks := requireMakeNBlocks(t, 3, pTipSet, genesisBlock.StateRoot.Cid, types.EmptyReceiptsCID, miners, minerToWorker, mockSigner)
		tipSet := th.RequireNewTipSet(t, nextBlocks...)

		emptyBLSMessages, emptyMessages := emptyMessages(len(nextBlocks))

		_, _, err = exp.RunStateTransition(ctx, tipSet, emptyBLSMessages, emptyMessages, []block.TipSet{pTipSet}, nextBlocks[0].ParentWeight, genesisBlock.StateRoot.Cid, genesisBlock.MessageReceipts.Cid)
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "invalid ticket")
	})

	t.Run("returns nil + mining error when signature is invalid", func(t *testing.T) {
		cistore, bstore := setupCborBlockstore()
		genesisBlock, err := th.DefaultGenesis(cistore, bstore)
		require.NoError(t, err)

		miners, minerToWorker := minerToWorkerFromAddrs(ctx, t, state.NewTree(cistore), vm.NewStorage(bstore), kis)
		views := consensus.AsPowerStateViewer(appstate.NewViewer(cistore))
		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), &views, th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{}, &proofs.ElectionPoster{})

		pTipSet := th.RequireNewTipSet(t, genesisBlock)
		nextBlocks := requireMakeNBlocks(t, 3, pTipSet, genesisBlock.StateRoot.Cid, types.EmptyReceiptsCID, miners, minerToWorker, mockSigner)

		// Give block 0 an invalid signature
		nextBlocks[0].BlockSig = nextBlocks[1].BlockSig

		tipSet := th.RequireNewTipSet(t, nextBlocks...)
		emptyBLSMessages, emptyMessages := emptyMessages(len(nextBlocks))

		_, _, err = exp.RunStateTransition(ctx, tipSet, emptyBLSMessages, emptyMessages, []block.TipSet{pTipSet}, nextBlocks[0].ParentWeight, nextBlocks[0].StateRoot.Cid, nextBlocks[0].MessageReceipts.Cid)
		assert.EqualError(t, err, "block signature invalid")
	})

	t.Run("returns nil + error when parent weight invalid", func(t *testing.T) {
		cistore, bstore := setupCborBlockstore()
		genesisBlock, err := th.DefaultGenesis(cistore, bstore)
		require.NoError(t, err)

		miners, minerToWorker := minerToWorkerFromAddrs(ctx, t, state.NewTree(cistore), vm.NewStorage(bstore), kis)
		views := consensus.AsPowerStateViewer(appstate.NewViewer(cistore))
		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), &views, th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{}, &proofs.ElectionPoster{})

		pTipSet := th.RequireNewTipSet(t, genesisBlock)
		nextBlocks := requireMakeNBlocks(t, 3, pTipSet, genesisBlock.StateRoot.Cid, types.EmptyReceiptsCID, miners, minerToWorker, mockSigner)
		tipSet := th.RequireNewTipSet(t, nextBlocks...)

		invalidParentWeight := fbig.NewInt(6)

		emptyBLSMessages, emptyMessages := emptyMessages(len(nextBlocks))

		_, _, err = exp.RunStateTransition(ctx, tipSet, emptyBLSMessages, emptyMessages, []block.TipSet{pTipSet}, invalidParentWeight, nextBlocks[0].StateRoot.Cid, nextBlocks[0].MessageReceipts.Cid)
		assert.Contains(t, err.Error(), "invalid parent weight")
	})
}

func emptyMessages(numBlocks int) ([][]*types.UnsignedMessage, [][]*types.SignedMessage) {
	var emptyBLSMessages [][]*types.UnsignedMessage
	var emptyMessages [][]*types.SignedMessage
	for i := 0; i < numBlocks; i++ {
		emptyBLSMessages = append(emptyBLSMessages, []*types.UnsignedMessage{})
		emptyMessages = append(emptyMessages, []*types.SignedMessage{})
	}
	return emptyBLSMessages, emptyMessages
}

func setupCborBlockstore() (*cborutil.IpldStore, blockstore.Blockstore) {
	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	cis := cborutil.NewIpldStore(bs)

	return cis, bs
}

// requireMakeNBlocks sets up 3 blocks with 3 owner actors and 3 miner actors and puts them in the state tree.
// the owner actors have associated mockSigners for signing blocks and tickets.
func requireMakeNBlocks(t *testing.T, n int, pTipSet block.TipSet, root cid.Cid, receiptRoot cid.Cid, minerAddrs []address.Address, m2w map[address.Address]address.Address, signer types.Signer) []*block.Block {
	require.True(t, n <= len(minerAddrs))
	blocks := make([]*block.Block, n)
	for i := 0; i < n; i++ {
		blocks[i] = th.RequireSignedTestBlockFromTipSet(t, pTipSet, root, receiptRoot, 1, minerAddrs[i], m2w[minerAddrs[i]], signer)
	}
	return blocks
}

func minerToWorkerFromAddrs(ctx context.Context, t *testing.T, tree state.Tree, vms vm.Storage, kis []crypto.KeyInfo) ([]address.Address, map[address.Address]address.Address) {
	minerAddrs := make([]address.Address, len(kis))
	require.Equal(t, len(kis), len(minerAddrs))
	minerToWorker := make(map[address.Address]address.Address, len(kis))
	for i := 0; i < len(kis); i++ {
		addr, err := kis[i].Address()
		require.NoError(t, err)

		_, minerAddrs[i] = th.RequireNewMinerActor(ctx, t, tree, vms, addr, 10000, th.RequireRandomPeerID(t), types.ZeroAttoFIL)

		minerToWorker[minerAddrs[i]] = addr
	}
	return minerAddrs, minerToWorker
}

func setTree(ctx context.Context, t *testing.T, kis []crypto.KeyInfo, cstore cbor.IpldStore, bstore blockstore.Blockstore, inRoot cid.Cid) (cid.Cid, []address.Address, map[address.Address]address.Address) {
	tree, err := state.NewTreeLoader().LoadStateTree(ctx, cstore, inRoot)
	require.NoError(t, err)
	miners := make([]address.Address, len(kis))
	m2w := make(map[address.Address]address.Address, len(kis))
	vms := vm.NewStorage(bstore)
	for i, ki := range kis {
		workerAddr, err := ki.Address()
		require.NoError(t, err)
		th.RequireInitAccountActor(ctx, t, tree, vms, workerAddr, types.ZeroAttoFIL)

		_, minerAddr := th.RequireNewMinerActor(ctx, t, tree, vms, workerAddr, 10000, th.RequireRandomPeerID(t), types.ZeroAttoFIL)
		miners[i] = minerAddr
		m2w[minerAddr] = workerAddr
	}
	root, err := tree.Flush(ctx)
	require.NoError(t, err)
	return root, miners, m2w
}

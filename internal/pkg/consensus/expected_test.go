package consensus_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-bls-sigs"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

func TestNewExpected(t *testing.T) {
	tf.UnitTest(t)

	t.Run("a new Expected can be created", func(t *testing.T) {
		cst, bstore := setupCborBlockstore()
		as := consensus.NewFakeActorStateStore(types.NewBytesAmount(1), types.NewBytesAmount(5), make(map[address.Address]address.Address))
		exp := consensus.NewExpected(cst, bstore, consensus.NewDefaultProcessor(), as, th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{})
		assert.NotNil(t, exp)
	})
}

// TestExpected_RunStateTransition_validateMining is concerned only with validateMining behavior.
// Fully unit-testing RunStateTransition is difficult due to this requiring that you
// completely set up a valid state tree with a valid matching TipSet.  RunStateTransition is tested
// with integration tests (see chain_daemon_test.go for example)
func TestExpected_RunStateTransition_validateMining(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	mockSigner, kis := types.NewMockSignersAndKeyInfo(3)

	cistore, bstore := setupCborBlockstore()
	genesisBlock, err := th.DefaultGenesis(cistore, bstore)
	require.NoError(t, err)

	t.Run("passes the validateMining section when given valid mining blocks", func(t *testing.T) {
		as := testActorState(t, kis)
		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), as, th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{})

		// Set miner actor

		pTipSet := th.RequireNewTipSet(t, genesisBlock)
		nextRoot := setTree(ctx, t, kis, cistore, bstore, genesisBlock.StateRoot)
		nextBlocks := requireMakeNBlocks(t, 3, pTipSet, nextRoot, types.EmptyReceiptsCID, kis, mockSigner)
		tipSet := th.RequireNewTipSet(t, nextBlocks...)

		emptyBLSMessages, emptyMessages := emptyMessages(len(nextBlocks))
		_, _, err = exp.RunStateTransition(ctx, tipSet, emptyBLSMessages, emptyMessages, []block.TipSet{pTipSet}, uint64(nextBlocks[0].ParentWeight), nextBlocks[0].StateRoot, nextBlocks[0].MessageReceipts)
		assert.NoError(t, err)
	})

	t.Run("returns nil + mining error when election proof validation fails", func(t *testing.T) {
		pTipSet := th.RequireNewTipSet(t, genesisBlock)

		as := testActorState(t, kis)
		exp := consensus.NewExpected(cistore, bstore, consensus.NewDefaultProcessor(), as, th.BlockTimeTest, &consensus.FailingElectionValidator{}, &consensus.FakeTicketMachine{})

		nextBlocks := requireMakeNBlocks(t, 3, pTipSet, genesisBlock.StateRoot, types.EmptyReceiptsCID, kis, mockSigner)
		tipSet := th.RequireNewTipSet(t, nextBlocks...)

		emptyBLSMessages, emptyMessages := emptyMessages(len(nextBlocks))

		_, _, err = exp.RunStateTransition(ctx, tipSet, emptyBLSMessages, emptyMessages, []block.TipSet{pTipSet}, uint64(nextBlocks[0].ParentWeight), genesisBlock.StateRoot, genesisBlock.MessageReceipts)
		assert.EqualError(t, err, "block author did not win election")
	})

	t.Run("correct tickets processed in election and next ticket", func(t *testing.T) {
		pTipSet := th.RequireNewTipSet(t, genesisBlock)
		as := testActorState(t, kis)

		nextRoot := setTree(ctx, t, kis, cistore, bstore, genesisBlock.StateRoot)
		ancestors := make([]block.TipSet, 5)
		for i := 0; i < consensus.ElectionLookback; i++ {
			ancestorBlk := requireMakeNBlocks(t, 1, pTipSet, nextRoot, types.EmptyReceiptsCID, kis, mockSigner)
			ancestors[i] = th.RequireNewTipSet(t, ancestorBlk...)
			pTipSet = ancestors[i]
		}

		isLookingBack := func(ticket block.Ticket) {
			expTicket, err := ancestors[consensus.ElectionLookback-1].MinTicket()
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

		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), as, th.BlockTimeTest, mockElection, mockTicketGen)

		nextBlocks := requireMakeNBlocks(t, 3, pTipSet, nextRoot, types.EmptyReceiptsCID, kis, mockSigner)
		tipSet := th.RequireNewTipSet(t, nextBlocks...)

		emptyBLSMessages, emptyMessages := emptyMessages(len(nextBlocks))
		_, _, err = exp.RunStateTransition(ctx, tipSet, emptyBLSMessages, emptyMessages, ancestors, uint64(nextBlocks[0].ParentWeight), nextRoot, nextBlocks[0].MessageReceipts)
		assert.NoError(t, err)
	})

	t.Run("fails when bls signature is not valid across bls messages", func(t *testing.T) {
		as := testActorState(t, kis)
		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), as, th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{})

		pTipSet := th.RequireNewTipSet(t, genesisBlock)
		nextBlocks := requireMakeNBlocks(t, 3, pTipSet, genesisBlock.StateRoot, types.EmptyReceiptsCID, kis, mockSigner)
		tipSet := th.RequireNewTipSet(t, nextBlocks...)

		_, emptyMessages := emptyMessages(len(nextBlocks))

		// Create BLS messages but do not update signature
		blsKey := bls.PrivateKeyPublicKey(bls.PrivateKeyGenerate())
		blsAddr, err := address.NewBLSAddress(blsKey[:])
		require.NoError(t, err)

		blsMessages := make([][]*types.UnsignedMessage, tipSet.Len())
		msg := types.NewUnsignedMessage(blsAddr, address.TestAddress2, 0, types.NewAttoFILFromFIL(0), types.InvalidMethodID, []byte{})
		blsMessages[0] = append(blsMessages[0], msg)

		_, _, err = exp.RunStateTransition(ctx, tipSet, blsMessages, emptyMessages, []block.TipSet{pTipSet}, uint64(nextBlocks[0].ParentWeight), nextBlocks[0].StateRoot, nextBlocks[0].MessageReceipts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "block BLS signature does not validate")
	})

	t.Run("fails when secp message has invalid signature", func(t *testing.T) {
		as := testActorState(t, kis)
		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), as, th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{})

		pTipSet := th.RequireNewTipSet(t, genesisBlock)
		nextBlocks := requireMakeNBlocks(t, 3, pTipSet, genesisBlock.StateRoot, types.EmptyReceiptsCID, kis, mockSigner)
		tipSet := th.RequireNewTipSet(t, nextBlocks...)

		emptyBLSMessages, _ := emptyMessages(len(nextBlocks))

		// Create secp message with invalid signature
		keys := types.MustGenerateKeyInfo(1, 42)
		blsAddr, err := address.NewSecp256k1Address(keys[0].PublicKey())
		require.NoError(t, err)

		secpMessages := make([][]*types.SignedMessage, tipSet.Len())
		msg := types.NewUnsignedMessage(blsAddr, address.TestAddress2, 0, types.NewAttoFILFromFIL(0), types.InvalidMethodID, []byte{})
		smsg := &types.SignedMessage{
			Message:   *msg,
			Signature: []byte("not a signature"),
		}
		secpMessages[0] = append(secpMessages[0], smsg)

		_, _, err = exp.RunStateTransition(ctx, tipSet, emptyBLSMessages, secpMessages, []block.TipSet{pTipSet}, uint64(nextBlocks[0].ParentWeight), nextBlocks[0].StateRoot, nextBlocks[0].MessageReceipts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "secp message signature invalid")
	})

	t.Run("returns nil + mining error when ticket validation fails", func(t *testing.T) {
		as := testActorState(t, kis)
		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), as, th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FailingTicketValidator{})

		pTipSet := th.RequireNewTipSet(t, genesisBlock)
		nextBlocks := requireMakeNBlocks(t, 3, pTipSet, genesisBlock.StateRoot, types.EmptyReceiptsCID, kis, mockSigner)
		tipSet := th.RequireNewTipSet(t, nextBlocks...)

		emptyBLSMessages, emptyMessages := emptyMessages(len(nextBlocks))

		_, _, err = exp.RunStateTransition(ctx, tipSet, emptyBLSMessages, emptyMessages, []block.TipSet{pTipSet}, uint64(nextBlocks[0].ParentWeight), genesisBlock.StateRoot, genesisBlock.MessageReceipts)
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "invalid ticket")
	})

	t.Run("returns nil + mining error when signature is invalid", func(t *testing.T) {
		as := testActorState(t, kis)
		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), as, th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{})

		pTipSet := th.RequireNewTipSet(t, genesisBlock)
		nextBlocks := requireMakeNBlocks(t, 3, pTipSet, genesisBlock.StateRoot, types.EmptyReceiptsCID, kis, mockSigner)

		// Give block 0 an invalid signature
		nextBlocks[0].BlockSig = nextBlocks[1].BlockSig

		tipSet := th.RequireNewTipSet(t, nextBlocks...)
		emptyBLSMessages, emptyMessages := emptyMessages(len(nextBlocks))

		_, _, err = exp.RunStateTransition(ctx, tipSet, emptyBLSMessages, emptyMessages, []block.TipSet{pTipSet}, uint64(nextBlocks[0].ParentWeight), nextBlocks[0].StateRoot, nextBlocks[0].MessageReceipts)
		assert.EqualError(t, err, "block signature invalid")
	})

	t.Run("returns nil + error when parent weight invalid", func(t *testing.T) {
		as := testActorState(t, kis)
		exp := consensus.NewExpected(cistore, bstore, th.NewFakeProcessor(), as, th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{})

		pTipSet := th.RequireNewTipSet(t, genesisBlock)
		nextBlocks := requireMakeNBlocks(t, 3, pTipSet, genesisBlock.StateRoot, types.EmptyReceiptsCID, kis, mockSigner)
		tipSet := th.RequireNewTipSet(t, nextBlocks...)

		invalidParentWeight := uint64(6)

		emptyBLSMessages, emptyMessages := emptyMessages(len(nextBlocks))

		_, _, err = exp.RunStateTransition(ctx, tipSet, emptyBLSMessages, emptyMessages, []block.TipSet{pTipSet}, invalidParentWeight, nextBlocks[0].StateRoot, nextBlocks[0].MessageReceipts)
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

func setupCborBlockstore() (*hamt.CborIpldStore, blockstore.Blockstore) {
	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	cis := hamt.CSTFromBstore(bs)

	return cis, bs
}

// requireMakeNBlocks sets up 3 blocks with 3 owner actors and 3 miner actors and puts them in the state tree.
// the owner actors have associated mockSigners for signing blocks and tickets.
func requireMakeNBlocks(t *testing.T, n int, pTipSet block.TipSet, root cid.Cid, receiptRoot cid.Cid, kis []types.KeyInfo, signer types.Signer) []*block.Block {
	require.True(t, n <= len(kis))
	minerAddrs := minerAddrsFromKis(t, kis)
	m2w := minerToWorkerFromKis(t, kis)
	blocks := make([]*block.Block, n)
	for i := 0; i < n; i++ {
		blocks[i] = th.RequireSignedTestBlockFromTipSet(t, pTipSet, root, receiptRoot, 1, minerAddrs[i], m2w[minerAddrs[i]], signer)
	}
	return blocks
}

func minerAddrsFromKis(t *testing.T, kis []types.KeyInfo) []address.Address {
	minerAddrs := make([]address.Address, len(kis))
	var err error
	for i := 0; i < len(kis); i++ {
		minerAddrs[i], err = address.NewSecp256k1Address([]byte(fmt.Sprintf("%s%s", kis[i], "Miner")))

		require.NoError(t, err)
	}
	return minerAddrs
}

func minerToWorkerFromKis(t *testing.T, kis []types.KeyInfo) map[address.Address]address.Address {
	minerAddrs := minerAddrsFromKis(t, kis)
	require.Equal(t, len(kis), len(minerAddrs))
	minerToWorker := make(map[address.Address]address.Address, len(kis))
	for i := 0; i < len(kis); i++ {
		addr, err := kis[i].Address()
		require.NoError(t, err)
		minerToWorker[minerAddrs[i]] = addr
	}
	return minerToWorker
}

func testActorState(t *testing.T, kis []types.KeyInfo) consensus.SnapshotGenerator {
	minerPower := types.NewBytesAmount(1)
	totalPower := types.NewBytesAmount(1)
	minerToWorker := minerToWorkerFromKis(t, kis)
	return consensus.NewFakeActorStateStore(minerPower, totalPower, minerToWorker)
}

func setTree(ctx context.Context, t *testing.T, kis []types.KeyInfo, cstore *hamt.CborIpldStore, bstore blockstore.Blockstore, inRoot cid.Cid) cid.Cid {
	tree, err := state.NewTreeLoader().LoadStateTree(ctx, cstore, inRoot)
	require.NoError(t, err)
	m2w := minerToWorkerFromKis(t, kis)
	vms := vm.NewStorageMap(bstore)
	for minerAddr, workerAddr := range m2w {
		ownerActor := th.RequireNewAccountActor(t, types.ZeroAttoFIL)
		require.NoError(t, tree.SetActor(ctx, workerAddr, ownerActor))

		minerActor := th.RequireNewMinerActor(t, vms, minerAddr, workerAddr,
			10000, th.RequireRandomPeerID(t), types.ZeroAttoFIL)
		require.NoError(t, tree.SetActor(ctx, minerAddr, minerActor))
	}
	root, err := tree.Flush(ctx)
	require.NoError(t, err)
	return root
}

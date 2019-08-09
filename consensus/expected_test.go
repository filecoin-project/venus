package consensus_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/proofs/verification"
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
	"github.com/minio/sha256-simd"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExpected(t *testing.T) {
	tf.UnitTest(t)

	t.Run("a new Expected can be created", func(t *testing.T) {
		cst, bstore, verifier := setupCborBlockstoreProofs()
		ptv := th.NewTestPowerTableView(types.NewBytesAmount(1), types.NewBytesAmount(5))
		exp := consensus.NewExpected(cst, bstore, consensus.NewDefaultProcessor(), th.NewFakeBlockValidator(), ptv, types.CidFromString(t, "somecid"), verifier, th.BlockTimeTest)
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

	cistore, bstore, verifier := setupCborBlockstoreProofs()
	genesisBlock, err := consensus.DefaultGenesis(cistore, bstore)
	require.NoError(t, err)

	t.Run("passes the validateMining section when given valid mining blocks", func(t *testing.T) {

		minerPower := types.NewBytesAmount(1)
		totalPower := types.NewBytesAmount(1)

		ptv := th.NewTestPowerTableView(minerPower, totalPower)
		exp := consensus.NewExpected(cistore, bstore, th.NewTestProcessor(), th.NewFakeBlockValidator(), ptv, genesisBlock.Cid(), verifier, th.BlockTimeTest)

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

	t.Run("returns nil + mining error when IsWinningTicket fails due to miner power error", func(t *testing.T) {

		ptv := NewFailingMinerTestPowerTableView(types.NewBytesAmount(1), types.NewBytesAmount(5))
		exp := consensus.NewExpected(cistore, bstore, consensus.NewDefaultProcessor(), th.NewFakeBlockValidator(), ptv, types.CidFromString(t, "somecid"), verifier, th.BlockTimeTest)

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
		assert.EqualError(t, err, "can't check for winning ticket: Couldn't get minerPower: something went wrong with the miner power")
	})
}

func TestIsWinningTicket(t *testing.T) {
	tf.UnitTest(t)

	minerAddress := address.NewForTestGetter()()
	ctx := context.Background()
	d := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(d)
	var st state.Tree

	// Just do a sanity check here and do the rest of the checks by tesing CompareTicketPower directly.
	t.Run("IsWinningTicket returns expected boolean + nil in non-error case", func(t *testing.T) {
		cases := []struct {
			ticket     byte
			myPower    uint64
			totalPower uint64
			wins       bool
		}{
			{0x00, 1, 5, true},
			{0xF0, 0, 5, false},
		}

		minerAddress := address.NewForTestGetter()()
		ctx := context.Background()
		d := datastore.NewMapDatastore()
		bs := blockstore.NewBlockstore(d)
		var st state.Tree

		for _, c := range cases {
			ptv := th.NewTestPowerTableView(types.NewBytesAmount(c.myPower), types.NewBytesAmount(c.totalPower))
			ticket := [65]byte{}
			ticket[0] = c.ticket
			r, err := consensus.IsWinningTicket(ctx, bs, ptv, st, ticket[:], minerAddress)
			assert.NoError(t, err)
			assert.Equal(t, c.wins, r, "%+v", c)
		}
	})

	testCase := struct {
		ticket     byte
		myPower    uint64
		totalPower uint64
		wins       bool
	}{0x00, 1, 5, true}

	t.Run("IsWinningTicket returns false + error when we fail to get total power", func(t *testing.T) {
		ptv1 := NewFailingTestPowerTableView(types.NewBytesAmount(testCase.myPower), types.NewBytesAmount(testCase.totalPower))
		ticket := [65]byte{}
		ticket[0] = testCase.ticket
		r, err := consensus.IsWinningTicket(ctx, bs, ptv1, st, ticket[:], minerAddress)
		assert.False(t, r)
		assert.Equal(t, err.Error(), "Couldn't get totalPower: something went wrong with the total power")

	})

	t.Run("IsWinningTicket returns false + error when we fail to get miner power", func(t *testing.T) {
		ptv2 := NewFailingMinerTestPowerTableView(types.NewBytesAmount(testCase.myPower), types.NewBytesAmount(testCase.totalPower))
		ticket := [sha256.Size]byte{}
		ticket[0] = testCase.ticket
		r, err := consensus.IsWinningTicket(ctx, bs, ptv2, st, ticket[:], minerAddress)
		assert.False(t, r)
		assert.Equal(t, err.Error(), "Couldn't get minerPower: something went wrong with the miner power")

	})
}

func TestCompareTicketPower(t *testing.T) {
	tf.UnitTest(t)

	cases := []struct {
		ticket     byte
		myPower    uint64
		totalPower uint64
		wins       bool
	}{
		{0x00, 1, 5, true},
		{0x30, 1, 5, true},
		{0x40, 1, 5, false},
		{0xF0, 1, 5, false},
		{0x00, 5, 5, true},
		{0x33, 5, 5, true},
		{0x44, 5, 5, true},
		{0xFF, 5, 5, true},
		{0x00, 0, 5, false},
		{0x33, 0, 5, false},
		{0x44, 0, 5, false},
		{0xFF, 0, 5, false},
	}
	for _, c := range cases {
		ticket := [65]byte{}
		ticket[0] = c.ticket
		res := consensus.CompareTicketPower(ticket[:], types.NewBytesAmount(c.myPower), types.NewBytesAmount(c.totalPower))
		assert.Equal(t, c.wins, res, "%+v", c)
	}
}

func TestCreateChallenge(t *testing.T) {
	tf.UnitTest(t)

	cases := []struct {
		parentTickets  [][]byte
		nullBlockCount uint64
		challenge      string
	}{
		// From https://www.di-mgt.com.au/sha_testvectors.html
		{[][]byte{[]byte("ac"), []byte("ab"), []byte("xx")},
			uint64('c'),
			"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},

		{[][]byte{[]byte("z"), []byte("x"), []byte("abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnop")},
			uint64('q'),
			"248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1"},

		{[][]byte{[]byte("abcdefghbcdefghicdefghijdefghijkefghijklfghijklmghijklmnhijklmnoijklmnopjklmnopqklmnopqrlmnopqrsmnopqrstnopqrst"), []byte("z"), []byte("x")},
			uint64('u'),
			"cf5b16a778af8380036ce59e7b0492370b249b11e8f07a51afac45037afee9d1"},
	}

	for _, c := range cases {
		decoded, err := hex.DecodeString(c.challenge)
		assert.NoError(t, err)

		var parents []*types.Block
		for _, ticket := range c.parentTickets {
			b := types.Block{Ticket: ticket}
			parents = append(parents, &b)
		}
		parentTs := types.RequireNewTipSet(t, parents...)

		r, err := consensus.CreateChallengeSeed(parentTs, c.nullBlockCount)
		assert.NoError(t, err)
		assert.Equal(t, decoded, r[:])
	}
}

func setupCborBlockstoreProofs() (*hamt.CborIpldStore, blockstore.Blockstore, verification.Verifier) {
	mds := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(mds)
	offl := offline.Exchange(bs)
	blkserv := blockservice.New(bs, offl)
	cis := &hamt.CborIpldStore{Blocks: blkserv}
	pv := &verification.FakeVerifier{
		VerifyPoStValid: true,
	}
	return cis, bs, pv
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

func (tv *FailingMinerTestPowerTableView) HasPower(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) bool {
	return true
}

package consensus_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/testhelpers"
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
		ptv := testhelpers.NewTestPowerTableView(types.NewBytesAmount(1), types.NewBytesAmount(5))
		exp := consensus.NewExpected(cst, bstore, consensus.NewDefaultProcessor(), ptv, types.SomeCid(), verifier)
		assert.NotNil(t, exp)
	})
}

// TestExpected_NewValidTipSet also tests validateBlockStructure.
func TestExpected_NewValidTipSet(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	cistore, bstore, verifier := setupCborBlockstoreProofs()
	ptv := testhelpers.NewTestPowerTableView(types.NewBytesAmount(1), types.NewBytesAmount(5))

	t.Run("NewValidTipSet returns a tipset + nil (no errors) when valid blocks", func(t *testing.T) {

		genesisBlock, err := consensus.DefaultGenesis(cistore, bstore)
		require.NoError(t, err)

		exp := consensus.NewExpected(cistore, bstore, consensus.NewDefaultProcessor(), ptv, genesisBlock.Cid(), verifier)

		pTipSet, err := exp.NewValidTipSet(ctx, []*types.Block{genesisBlock})
		require.NoError(t, err)

		stateTree, err := state.LoadStateTree(ctx, cistore, genesisBlock.StateRoot, builtin.Actors)
		require.NoError(t, err)

		vms := vm.NewStorageMap(bstore)

		blocks := requireMakeBlocks(ctx, t, pTipSet, stateTree, vms)

		tipSet, err := exp.NewValidTipSet(ctx, blocks)
		assert.NoError(t, err)
		assert.NotNil(t, tipSet)
	})

	t.Run("NewValidTipSet returns nil + error when invalid blocks", func(t *testing.T) {

		parentBlock := types.NewBlockForTest(nil, 0)

		blocks := []*types.Block{
			types.NewBlockForTest(parentBlock, 1),
		}
		ki := types.MustGenerateKeyInfo(1, types.GenerateKeyInfoSeed())
		mockSigner := types.NewMockSigner(ki)
		blocks[0].Messages = types.NewSignedMsgs(1, mockSigner)
		retVal := []byte{1, 2, 3}

		receipt := &types.MessageReceipt{
			ExitCode: 123,
			Return:   [][]byte{retVal},
		}
		blocks[0].MessageReceipts = []*types.MessageReceipt{receipt}

		exp := consensus.NewExpected(cistore, bstore, consensus.NewDefaultProcessor(), ptv, types.SomeCid(), verifier)

		tipSet, err := exp.NewValidTipSet(ctx, blocks)
		assert.Error(t, err, "Foo")
		assert.Nil(t, tipSet)
	})
}

// requireMakeBlocks sets up 3 blocks with 3 owner actors and 3 miner actors and puts them in the state tree.
// the owner actors have associated mockSigners for signing blocks (not implemented yet) and tickets.
func requireMakeBlocks(ctx context.Context, t *testing.T, pTipSet types.TipSet, tree state.Tree, vms vm.StorageMap) []*types.Block {
	// make  a set of owner keypairs so they can sign blocks
	mockSigner, kis := types.NewMockSignersAndKeyInfo(3)

	// iterate over the keypairs and set up owner actors and miner actors with their own addresses
	// and add them to the state tree
	ownerPubKeys := make([][]byte, 3)
	minerAddrs := make([]address.Address, 3)
	for i, name := range kis {
		addr, err := kis[i].Address()
		require.NoError(t, err)

		ownerPubKeys[i] = kis[i].PublicKey()

		ownerActor := testhelpers.RequireNewAccountActor(t, types.NewZeroAttoFIL())
		require.NoError(t, tree.SetActor(ctx, addr, ownerActor))

		minerAddrs[i], err = address.NewActorAddress([]byte(fmt.Sprintf("%s%s", name, "Miner")))
		require.NoError(t, err)
		minerActor := testhelpers.RequireNewMinerActor(t, vms, minerAddrs[i], addr,
			ownerPubKeys[i], 10000, testhelpers.RequireRandomPeerID(t), types.NewZeroAttoFIL())
		require.NoError(t, tree.SetActor(ctx, minerAddrs[i], minerActor))
	}
	stateRoot, err := tree.Flush(ctx)
	require.NoError(t, err)

	blocks := []*types.Block{
		testhelpers.NewValidTestBlockFromTipSet(pTipSet, stateRoot, 1, minerAddrs[0], ownerPubKeys[0], mockSigner),
		testhelpers.NewValidTestBlockFromTipSet(pTipSet, stateRoot, 1, minerAddrs[1], ownerPubKeys[1], mockSigner),
		testhelpers.NewValidTestBlockFromTipSet(pTipSet, stateRoot, 1, minerAddrs[2], ownerPubKeys[2], mockSigner),
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

		ptv := testhelpers.NewTestPowerTableView(minerPower, totalPower)
		exp := consensus.NewExpected(cistore, bstore, testhelpers.NewTestProcessor(), ptv, genesisBlock.Cid(), verifier)

		pTipSet, err := exp.NewValidTipSet(ctx, []*types.Block{genesisBlock})
		require.NoError(t, err)

		stateTree, err := state.LoadStateTree(ctx, cistore, genesisBlock.StateRoot, builtin.Actors)
		require.NoError(t, err)
		vms := vm.NewStorageMap(bstore)

		blocks := requireMakeBlocks(ctx, t, pTipSet, stateTree, vms)

		tipSet, err := exp.NewValidTipSet(ctx, blocks)
		require.NoError(t, err)

		_, err = exp.RunStateTransition(ctx, tipSet, []types.TipSet{pTipSet}, stateTree)
		assert.NoError(t, err)
	})

	t.Run("returns nil + mining error when IsWinningTicket fails due to miner power error", func(t *testing.T) {

		ptv := NewFailingMinerTestPowerTableView(types.NewBytesAmount(1), types.NewBytesAmount(5))
		exp := consensus.NewExpected(cistore, bstore, consensus.NewDefaultProcessor(), ptv, types.SomeCid(), verifier)

		pTipSet, err := exp.NewValidTipSet(ctx, []*types.Block{genesisBlock})
		require.NoError(t, err)

		stateTree, err := state.LoadStateTree(ctx, cistore, genesisBlock.StateRoot, builtin.Actors)
		require.NoError(t, err)

		vms := vm.NewStorageMap(bstore)

		blocks := requireMakeBlocks(ctx, t, pTipSet, stateTree, vms)

		tipSet, err := exp.NewValidTipSet(ctx, blocks)
		require.NoError(t, err)

		_, err = exp.RunStateTransition(ctx, tipSet, []types.TipSet{pTipSet}, stateTree)
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
			ptv := testhelpers.NewTestPowerTableView(types.NewBytesAmount(c.myPower), types.NewBytesAmount(c.totalPower))
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

		parents := types.TipSet{}
		for _, ticket := range c.parentTickets {
			b := types.Block{Ticket: ticket}
			err = parents.AddBlock(&b)
			assert.NoError(t, err)
		}
		r, err := consensus.CreateChallengeSeed(parents, c.nullBlockCount)
		assert.NoError(t, err)
		assert.Equal(t, decoded, r[:])
	}
}

func setupCborBlockstoreProofs() (*hamt.CborIpldStore, blockstore.Blockstore, proofs.Verifier) {
	mds := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(mds)
	offl := offline.Exchange(bs)
	blkserv := blockservice.New(bs, offl)
	cis := &hamt.CborIpldStore{Blocks: blkserv}
	pv := proofs.NewFakeVerifier(true, nil)
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

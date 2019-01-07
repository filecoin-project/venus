package chain

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	mrand "math/rand"
	"testing"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	bstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	mh "gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/stretchr/testify/require"
)

// FakeChildParams is a wrapper for all the params needed to create fake child blocks.
type FakeChildParams struct {
	Consensus      consensus.Protocol
	GenesisCid     cid.Cid
	MinerAddr      address.Address
	Nonce          uint64
	NullBlockCount uint64
	Parent         consensus.TipSet
	StateRoot      cid.Cid
}

// MkFakeChild creates a mock child block of a genesis block. If a
// stateRootCid is non-nil it will be added to the block, otherwise
// MkFakeChild will use the stateRoot of the parent tipset.  State roots
// in blocks constructed with MkFakeChild are invalid with respect to
// any messages in parent tipsets.
//
// MkFakeChild does not mine the block. The parent set does not have a min
// ticket that would validate that the child's miner is elected by consensus.
// In fact MkFakeChild does not assign a miner address to the block at all.
//
// MkFakeChild assigns blocks correct parent weight, height, and parent headers.
// Chains created with this function are useful for validating chain syncing
// and chain storing behavior, and the weight related methods of the consensus
// interface.  They are not useful for testing the full range of consensus
// validation, particularly message processing and mining edge cases.
func MkFakeChild(params FakeChildParams) (*types.Block, error) {

	// Create consensus for reading the valid weight
	bs := bstore.NewBlockstore(repo.NewInMemoryRepo().Datastore())
	cst := hamt.NewCborStore()
	powerTableView := &th.TestView{}
	con := consensus.NewExpected(cst,
		bs,
		th.NewTestProcessor(),
		powerTableView,
		params.GenesisCid,
		proofs.NewFakeProver(true, nil))
	params.Consensus = con
	return MkFakeChildWithCon(params)
}

// MkFakeChildWithCon creates a chain with the given consensus weight function.
func MkFakeChildWithCon(params FakeChildParams) (*types.Block, error) {
	wFun := func(ts consensus.TipSet) (uint64, error) {
		return params.Consensus.Weight(context.Background(), params.Parent, nil)
	}
	return MkFakeChildCore(params.Parent,
		params.GenesisCid,
		params.StateRoot,
		params.Nonce,
		params.NullBlockCount,
		params.MinerAddr,
		wFun)
}

// MkFakeChildCore houses shared functionality between MkFakeChildWithCon and MkFakeChild.
func MkFakeChildCore(parent consensus.TipSet,
	genCid cid.Cid,
	stateRoot cid.Cid,
	nonce uint64,
	nullBlockCount uint64,
	minerAddress address.Address,
	wFun func(consensus.TipSet) (uint64, error)) (*types.Block, error) {
	// State can be nil because it doesn't it is assumed consensus uses a
	// power table view that does not access the state.
	w, err := wFun(parent)
	if err != nil {
		return nil, err
	}

	// Height is parent height plus null block count plus one
	pHeight, err := parent.Height()
	if err != nil {
		return nil, err
	}
	height := pHeight + uint64(1) + nullBlockCount

	pIDs := parent.ToSortedCidSet()

	newBlock := th.NewValidTestBlockFromTipSet(parent, height, minerAddress)

	// Override fake values with our values
	newBlock.Parents = pIDs
	newBlock.ParentWeight = types.Uint64(w)
	newBlock.Nonce = types.Uint64(nonce)
	newBlock.StateRoot = stateRoot

	return newBlock, nil
}

// RequireMkFakeChild wraps MkFakeChild with a testify requirement that it does not error
func RequireMkFakeChild(require *require.Assertions, params FakeChildParams) *types.Block {
	child, err := MkFakeChild(params)
	require.NoError(err)
	return child
}

// RequireMkFakeChildWithCon wraps MkFakeChildWithCon with a requirement that
// it does not error.
func RequireMkFakeChildWithCon(require *require.Assertions, params FakeChildParams) *types.Block {
	child, err := MkFakeChildWithCon(params)
	require.NoError(err)
	return child
}

// RequireMkFakeChildCore wraps MkFakeChildCore with a requirement that
// it does not errror.
func RequireMkFakeChildCore(require *require.Assertions,
	params FakeChildParams,
	wFun func(consensus.TipSet) (uint64, error)) *types.Block {
	child, err := MkFakeChildCore(params.Parent, params.GenesisCid, params.StateRoot, params.Nonce, params.NullBlockCount, params.MinerAddr, wFun)
	require.NoError(err)
	return child
}

// MustNewTipSet makes a new tipset or panics trying.
func MustNewTipSet(blks ...*types.Block) consensus.TipSet {
	ts, err := consensus.NewTipSet(blks...)
	if err != nil {
		panic(err)
	}
	return ts
}

// RequirePutTsas ensures that the provided tipset and state is placed in the
// input store.
func RequirePutTsas(ctx context.Context, require *require.Assertions, chain Store, tsas *TipSetAndState) {
	err := chain.PutTipSetAndState(ctx, tsas)
	require.NoError(err)
}

// CreateMinerWithPower uses storage market functionality to mine the messages needed to create a miner, ask, bid, and deal, and then commit that deal to give the miner power.
// If the power is nil, this method will just create the miner.
// The returned block and nonce should be used in subsequent calls to this method.
func CreateMinerWithPower(ctx context.Context,
	t *testing.T,
	syncer Syncer,
	lastBlock *types.Block,
	sn types.MockSigner,
	nonce uint64,
	rewardAddress address.Address,
	power uint64,
	cst *hamt.CborIpldStore,
	bs bstore.Blockstore,
	genCid cid.Cid) (address.Address, *types.Block, uint64, error) {
	require := require.New(t)

	pledge := power
	if pledge == uint64(0) {
		pledge = uint64(10)
	}
	var bigIntPledge big.Int
	bigIntPledge.SetUint64(pledge)

	// create miner
	msg, err := th.CreateMinerMessage(sn.Addresses[0], nonce, pledge, RequireRandomPeerID(), storagemarket.MinimumCollateral(&bigIntPledge))
	require.NoError(err)

	ptv := th.NewTestPowerTableView(power, 1000)

	b := RequireMineOnce(ctx, t, syncer, cst, bs, lastBlock, rewardAddress, []*types.SignedMessage{mockSign(sn, msg)}, ptv, genCid)
	nonce++

	require.Equal(uint8(0), b.MessageReceipts[0].ExitCode)
	minerAddr, err := address.NewFromBytes(b.MessageReceipts[0].Return[0])
	require.NoError(err)

	if power == uint64(0) {
		return minerAddr, b, nonce, nil
	}

	// TODO: We should obtain the SectorID from the SectorBuilder instead of
	// hard-coding a value here.
	sectorID := uint64(0)

	// commit sector (thus adding power to miner and recording in storage market).
	msgs := make([]*types.SignedMessage, power)
	for i := 0; uint64(i) < power; i++ {
		msg, err = th.CommitSectorMessage(minerAddr, sn.Addresses[0], nonce, sectorID, th.MakeCommitment(), th.MakeCommitment(), th.MakeCommitment())
		require.NoError(err)
		msgs[i] = mockSign(sn, msg)
		sectorID++
		nonce++
	}

	b = RequireMineOnce(ctx, t, syncer, cst, bs, b, rewardAddress, msgs, ptv, genCid)
	for _, r := range b.MessageReceipts {
		require.Equal(uint8(0), r.ExitCode)
	}

	return minerAddr, b, nonce, nil
}

// RequireMineOnce process one block and panic on error.
// TODO ideally this should be wired up to the block generation functionality in the mining sub-package.
func RequireMineOnce(ctx context.Context,
	t *testing.T,
	syncer Syncer,
	cst *hamt.CborIpldStore,
	bs bstore.Blockstore,
	lastBlock *types.Block,
	rewardAddress address.Address,
	msgs []*types.SignedMessage,
	ptv consensus.PowerTableView,
	genCid cid.Cid) *types.Block {
	require := require.New(t)

	// Make a block for processing.
	baseTipSet := th.RequireNewTipSet(require, lastBlock)

	// WARNING this assumes a test power table view, not a real one!!!
	totalPower, err := ptv.Total(ctx, nil, bs)
	require.NoError(err)
	minerPower, err := ptv.Miner(ctx, nil, bs, rewardAddress)
	require.NoError(err)
	// WARNING

	b, err := MkFakeChild(FakeChildParams{
		GenesisCid: genCid,
		StateRoot:  lastBlock.StateRoot,
		Parent:     baseTipSet,
		MinerAddr:  rewardAddress,
	})
	require.NoError(err)

	// proofs & tickets for minerPower = 0 aren't needed
	if minerPower > 0 {
		b.Proof, b.Ticket, err = MakeProofAndWinningTicket(rewardAddress, minerPower, totalPower)
		require.NoError(err)
	}

	// Get the updated state root after applying messages.
	st, err := state.LoadStateTree(ctx, cst, lastBlock.StateRoot, builtin.Actors)
	require.NoError(err)

	vms := vm.NewStorageMap(bs)
	require.NoError(err)
	b.Messages = append(b.Messages, msgs...)

	results, err := th.NewTestProcessor().ProcessBlock(ctx, st, vms, b)
	require.NoError(err)
	err = vms.Flush()
	require.NoError(err)
	newStateRoot, err := st.Flush(ctx)
	require.NoError(err)

	// Update block with new state root and message receipts.
	for _, r := range results {
		b.MessageReceipts = append(b.MessageReceipts, r.Receipt)
	}
	b.StateRoot = newStateRoot

	// Sync the block.
	c, err := cst.Put(ctx, b)
	require.NoError(err)
	err = syncer.HandleNewBlocks(ctx, []cid.Cid{c})
	require.NoError(err)

	return b
}

// MakeProofAndWinningTicket generates a proof and ticket that will pass validateMining.
func MakeProofAndWinningTicket(minerAddr address.Address, minerPower uint64, totalPower uint64) (proofs.PoStProof, types.Signature, error) {
	var postProof proofs.PoStProof
	var ticket types.Signature

	for {
		postProof = th.MakeRandomPoSTProofForTest()
		ticket = consensus.CreateTicket(postProof, minerAddr)
		if consensus.CompareTicketPower(ticket, minerPower, totalPower) {
			return postProof, ticket, nil
		}
	}

	return postProof, nil, errors.New("could not calculate a proof")
}

// These peer.ID generators were copied from libp2p/go-testutil. We didn't bring in the
// whole repo as a dependency because we only need this small bit. However if we find
// ourselves using more and more pieces we should just take a dependency on it.
func randPeerID() (peer.ID, error) {
	buf := make([]byte, 16)
	if n, err := rand.Read(buf); n != 16 || err != nil {
		if n != 16 && err == nil {
			err = errors.New("couldnt read 16 random bytes")
		}
		panic(err)
	}
	h, _ := mh.Sum(buf, mh.SHA2_256, -1)
	return peer.ID(h), nil
}

// RequireRandomPeerID returns a new libp2p peer ID or panics.
func RequireRandomPeerID() peer.ID {
	pid, err := randPeerID()
	if err != nil {
		panic(err)
	}

	return pid
}

func mockSign(sn types.MockSigner, msg *types.Message) *types.SignedMessage {
	return th.MustSign(sn, msg)[0]
}

// AddChain creates a new chain of length, beginning from blks, and adds to
// the input chain store.  Blocks of the chain do not contain messages.
// Precondition: the starting tipset must be in the store.
func AddChain(ctx context.Context, chain Store, start []*types.Block, length int) (*types.Block, error) {
	// look up starting state in the store
	var cids types.SortedCidSet
	for _, blk := range start {
		(&cids).Add(blk.Cid())
	}
	id := cids.String()
	tsas, err := chain.GetTipSetAndState(ctx, id)
	if err != nil {
		return nil, err
	}
	ts := tsas.TipSet
	stateRoot := tsas.TipSetStateRoot
	l := uint64(length)
	var blk *types.Block
	for i := uint64(0); i < l; i++ {
		blk, err = MkFakeChild(FakeChildParams{
			GenesisCid:     chain.GenesisCid(),
			StateRoot:      stateRoot,
			Parent:         ts,
			Nonce:          i,
			NullBlockCount: uint64(0),
		})
		if err != nil {
			return nil, err
		}
		ts, err = consensus.NewTipSet(blk)
		if err != nil {
			return nil, err
		}
		err = chain.PutTipSetAndState(ctx, &TipSetAndState{
			TipSet:          ts,
			TipSetStateRoot: stateRoot,
		})
		if err != nil {
			return nil, err
		}
		if err = chain.SetHead(ctx, ts); err != nil {
			return nil, err
		}

	}
	return blk, nil
}

func getWinningMinerCount(n int, p float64) int {
	wins := 0
	for i := 0; i < n; i++ {
		if mrand.Float64() < p {
			wins++
		}
	}
	return wins
}

// AddChainBinomBlocksPerEpoch creates a new chain of length, beginning from blks,
// and adds to the input chain store.  Blocks of the chain do not contain messages.
// The number of blocks mined in each epoch is drawn from the binomial distribution
// where n = num_miners and p = 1/n.  Concretely this distribution corresponds to
// the configuration where all miners havwe the same storage power.
// Precondition: the starting tipset must be in the store.
// FIXME: Unused - https://github.com/filecoin-project/go-filecoin/issues/1541
// this function is called only in a commented-out test -- of itself -- which fails anyway.
func AddChainBinomBlocksPerEpoch(ctx context.Context, chain Store, start []*types.Block, numMiners, length int) (consensus.TipSet, error) {
	var cids types.SortedCidSet
	for _, blk := range start {
		(&cids).Add(blk.Cid())
	}
	id := cids.String()
	tsas, err := chain.GetTipSetAndState(ctx, id)
	if err != nil {
		return nil, err
	}
	ts := tsas.TipSet
	stateRoot := tsas.TipSetStateRoot

	// Initialize epoch traversal.
	l := uint64(length)
	var lastNull uint64
	var head consensus.TipSet
	p := float64(1) / float64(numMiners)
	// Construct a tipset for each epoch.
	for i := uint64(0); i < l; i++ {
		head = consensus.TipSet{}
		// Draw number of blocks per TS from binom distribution.
		nBlks := getWinningMinerCount(numMiners, p)
		if nBlks == 0 {
			lastNull += uint64(1)
		}

		// Construct each block.
		for j := 0; j < nBlks; j++ {
			blk, err := MkFakeChild(FakeChildParams{
				GenesisCid:     chain.GenesisCid(),
				StateRoot:      stateRoot,
				Parent:         ts,
				Nonce:          i,
				NullBlockCount: lastNull,
			})
			if err != nil {
				return nil, err
			}
			err = head.AddBlock(blk)
			if err != nil {
				return nil, err
			}
		}

		// Update chain, chain head and null block count.
		if nBlks > 0 {
			lastNull = 0
			ts = head
			err = chain.PutTipSetAndState(ctx, &TipSetAndState{
				TipSet:          ts,
				TipSetStateRoot: stateRoot,
			})
			if err != nil {
				return nil, err
			}
			if err = chain.SetHead(ctx, ts); err != nil {
				return nil, err
			}
		}
	}
	return head, nil
}

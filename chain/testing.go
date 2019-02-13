package chain

import (
	"context"
	"errors"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	bstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/repo"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/require"
)

// FakeChildParams is a wrapper for all the params needed to create fake child blocks.
type FakeChildParams struct {
	Consensus      consensus.Protocol
	GenesisCid     cid.Cid
	MinerAddr      address.Address
	Nonce          uint64
	NullBlockCount uint64
	Parent         types.TipSet
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
		proofs.NewFakeVerifier(true, nil))
	params.Consensus = con
	return MkFakeChildWithCon(params)
}

// MkFakeChildWithCon creates a chain with the given consensus weight function.
func MkFakeChildWithCon(params FakeChildParams) (*types.Block, error) {
	wFun := func(ts types.TipSet) (uint64, error) {
		return params.Consensus.Weight(context.Background(), params.Parent, nil)
	}
	return MkFakeChildCore(params.Parent,
		params.StateRoot,
		params.Nonce,
		params.NullBlockCount,
		params.MinerAddr,
		wFun)
}

// MkFakeChildCore houses shared functionality between MkFakeChildWithCon and MkFakeChild.
func MkFakeChildCore(parent types.TipSet,
	stateRoot cid.Cid,
	nonce uint64,
	nullBlockCount uint64,
	minerAddress address.Address,
	wFun func(types.TipSet) (uint64, error)) (*types.Block, error) {
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
	wFun func(types.TipSet) (uint64, error)) *types.Block {
	child, err := MkFakeChildCore(params.Parent, params.StateRoot, params.Nonce, params.NullBlockCount, params.MinerAddr, wFun)
	require.NoError(err)
	return child
}

// MustNewTipSet makes a new tipset or panics trying.
func MustNewTipSet(blks ...*types.Block) types.TipSet {
	ts, err := types.NewTipSet(blks...)
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

// MakeProofAndWinningTicket generates a proof and ticket that will pass validateMining.
func MakeProofAndWinningTicket(minerAddr address.Address, minerPower uint64, totalPower uint64) (proofs.PoStProof, types.Signature, error) {
	var postProof proofs.PoStProof
	var ticket types.Signature

	if totalPower/minerPower > 100000 {
		return postProof, ticket, errors.New("MakeProofAndWinningTicket: minerPower is too small for totalPower to generate a winning ticket")
	}

	for {
		postProof = th.MakeRandomPoSTProofForTest()
		ticket = consensus.CreateTicket(postProof, minerAddr)
		if consensus.CompareTicketPower(ticket, minerPower, totalPower) {
			return postProof, ticket, nil
		}
	}
}

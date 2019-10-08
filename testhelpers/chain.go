package testhelpers

import (
	"context"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
)

type chainWeighter interface {
	Weight(context.Context, types.TipSet, cid.Cid) (uint64, error)
}

// FakeChildParams is a wrapper for all the params needed to create fake child blocks.
type FakeChildParams struct {
	ConsensusChainSelection chainWeighter
	GenesisCid              cid.Cid
	MinerAddr               address.Address
	NullBlockCount          uint64
	Parent                  types.TipSet
	StateRoot               cid.Cid
	Signer                  types.Signer
	MinerWorker             address.Address
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
	processor := consensus.NewDefaultProcessor()
	actorState := consensus.NewActorStateStore(nil, cst, bs, processor)
	selector := consensus.NewChainSelector(cst,
		actorState,
		params.GenesisCid,
	)
	params.ConsensusChainSelection = selector
	return MkFakeChildWithCon(params)
}

// MkFakeChildWithCon creates a chain with the given consensus weight function.
func MkFakeChildWithCon(params FakeChildParams) (*types.Block, error) {
	wFun := func(ts types.TipSet) (uint64, error) {
		return params.ConsensusChainSelection.Weight(context.Background(), params.Parent, cid.Undef)
	}
	return MkFakeChildCore(params.Parent,
		params.StateRoot,
		params.NullBlockCount,
		params.MinerAddr,
		params.MinerWorker,
		params.Signer,
		wFun)
}

// MkFakeChildCore houses shared functionality between MkFakeChildWithCon and MkFakeChild.
// NOTE: This is NOT deterministic because it generates a random value for the Proof field.
func MkFakeChildCore(parent types.TipSet,
	stateRoot cid.Cid,
	nullBlockCount uint64,
	minerAddr address.Address,
	minerWorker address.Address,
	signer types.Signer,
	wFun func(types.TipSet) (uint64, error)) (*types.Block, error) {
	// State can be nil because it is assumed consensus uses a
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

	pIDs := parent.Key()

	newBlock, err := NewValidTestBlockFromTipSet(parent, stateRoot, height, minerAddr, minerWorker, signer)
	if err != nil {
		return nil, err
	}

	// Override fake values with our values
	newBlock.Parents = pIDs
	newBlock.ParentWeight = types.Uint64(w)
	newBlock.StateRoot = stateRoot

	return newBlock, nil
}

// RequireMkFakeChild wraps MkFakeChild with a testify requirement that it does not error
func RequireMkFakeChild(t *testing.T, params FakeChildParams) *types.Block {
	child, err := MkFakeChild(params)
	require.NoError(t, err)
	return child
}

// RequireMkFakeChain returns a chain of num successive tipsets (no null blocks)
// created with MkFakeChild and starting off of base.  Nonce, genCid and
// stateRoot parameters for the whole chain are passed in with params.
func RequireMkFakeChain(t *testing.T, base types.TipSet, num int, params FakeChildParams) []types.TipSet {
	var ret []types.TipSet
	params.Parent = base
	for i := 0; i < num; i++ {
		block := RequireMkFakeChild(t, params)
		ts := RequireNewTipSet(t, block)
		ret = append(ret, ts)
		params.Parent = ts
	}
	return ret
}

// RequireMkFakeChildCore wraps MkFakeChildCore with a requirement that
// it does not errror.
func RequireMkFakeChildCore(t *testing.T,
	params FakeChildParams,
	wFun func(types.TipSet) (uint64, error)) *types.Block {
	child, err := MkFakeChildCore(params.Parent,
		params.StateRoot,
		params.NullBlockCount,
		params.MinerAddr,
		params.MinerWorker,
		params.Signer,
		wFun)
	require.NoError(t, err)
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

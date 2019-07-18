package testhelpers

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/proofs/verification"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
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
	Signer         consensus.TicketSigner
	MinerWorker    address.Address
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
	powerTableView := &TestView{}
	con := consensus.NewExpected(cst,
		bs,
		NewTestProcessor(),
		NewFakeBlockValidator(),
		powerTableView,
		params.GenesisCid,
		&verification.FakeVerifier{
			VerifyPoStValid:                true,
			VerifyPieceInclusionProofValid: true,
			VerifySealValid:                true,
		},
		BlockTimeTest)
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
		params.MinerWorker,
		params.Signer,
		wFun)
}

// MkFakeChildCore houses shared functionality between MkFakeChildWithCon and MkFakeChild.
// NOTE: This is NOT deterministic because it generates a random value for the Proof field.
func MkFakeChildCore(parent types.TipSet,
	stateRoot cid.Cid,
	nonce uint64,
	nullBlockCount uint64,
	minerAddr address.Address,
	minerWorker address.Address,
	signer consensus.TicketSigner,
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

	newBlock := NewValidTestBlockFromTipSet(parent, stateRoot, height, minerAddr, minerWorker, signer)

	// Override fake values with our values
	newBlock.Parents = pIDs
	newBlock.ParentWeight = types.Uint64(w)
	newBlock.Nonce = types.Uint64(nonce)
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

// RequireMkFakeChildWithCon wraps MkFakeChildWithCon with a requirement that
// it does not error.
func RequireMkFakeChildWithCon(t *testing.T, params FakeChildParams) *types.Block {
	child, err := MkFakeChildWithCon(params)
	require.NoError(t, err)
	return child
}

// RequireMkFakeChildCore wraps MkFakeChildCore with a requirement that
// it does not errror.
func RequireMkFakeChildCore(t *testing.T,
	params FakeChildParams,
	wFun func(types.TipSet) (uint64, error)) *types.Block {
	child, err := MkFakeChildCore(params.Parent,
		params.StateRoot,
		params.Nonce,
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

// MakeProofAndWinningTicket generates a proof and ticket that will pass validateMining.
func MakeProofAndWinningTicket(signerAddr address.Address, minerPower *types.BytesAmount, totalPower *types.BytesAmount, signer consensus.TicketSigner) (types.PoStProof, types.Signature, error) {
	poStProof := make([]byte, types.OnePoStProofPartition.ProofLen())
	var ticket types.Signature

	quot := totalPower.Quo(minerPower)
	threshold := types.NewBytesAmount(100000).Mul(types.OneKiBSectorSize)

	if quot.GreaterThan(threshold) {
		return poStProof, ticket, errors.New("MakeProofAndWinningTicket: minerPower is too small for totalPower to generate a winning ticket")
	}

	for {
		poStProof = MakeRandomPoStProofForTest()
		ticket, err := consensus.CreateTicket(poStProof, signerAddr, signer)
		if err != nil {
			errStr := fmt.Sprintf("error creating ticket: %s", err)
			panic(errStr)
		}
		if consensus.CompareTicketPower(ticket, minerPower, totalPower) {
			return poStProof, ticket, nil
		}
	}
}

package consensus

// This is to implement Expected Consensus protocol
// See: https://github.com/filecoin-project/specs/blob/master/expected-consensus.md

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"
	logging "github.com/ipfs/go-log"
	"github.com/minio/sha256-simd"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/metrics/tracing"
	"github.com/filecoin-project/go-filecoin/proofs/verification"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

var (
	ticketDomain *big.Int
	log          = logging.Logger("consensus.expected")
)

func init() {
	ticketDomain = &big.Int{}
	// The size of the ticket domain must equal the size of the Signature (ticket) generated.
	// Currently this is a secp256k1.Sign signature, which is 65 bytes.
	ticketDomain.Exp(big.NewInt(2), big.NewInt(65*8), nil)
	ticketDomain.Sub(ticketDomain, big.NewInt(1))
}

var (
	// ErrStateRootMismatch is returned when the computed state root doesn't match the expected result.
	ErrStateRootMismatch = errors.New("blocks state root does not match computed result")
	// ErrInvalidBase is returned when the chain doesn't connect back to a known good block.
	ErrInvalidBase = errors.New("block does not connect to a known good chain")
	// ErrUnorderedTipSets is returned when weight and minticket are the same between two tipsets.
	ErrUnorderedTipSets = errors.New("trying to order two identical tipsets")
)

// TicketSigner is an interface for a test signer that can create tickets.
type TicketSigner interface {
	GetAddressForPubKey(pk []byte) (address.Address, error)
	SignBytes(data []byte, signerAddr address.Address) (types.Signature, error)
}

// DefaultBlockTime is the estimated proving period time.
// We define this so that we can fake mining in the current incomplete system.
// We also use this to enforce a soft block validation.
const DefaultBlockTime = 30 * time.Second

// TODO none of these parameters are chosen correctly
// with respect to analysis under a security model:
// https://github.com/filecoin-project/go-filecoin/issues/1846

// ECV is the constant V defined in the EC spec.
const ECV uint64 = 10

// ECPrM is the power ratio magnitude defined in the EC spec.
const ECPrM uint64 = 100

// AncestorRoundsNeeded is the number of rounds of the ancestor chain needed
// to process all state transitions.
//
// TODO: If the following PR is merged - and the network doesn't define a
// largest sector size - this constant will need to be reconsidered.
// https://github.com/filecoin-project/specs/pull/318
const AncestorRoundsNeeded = miner.LargestSectorSizeProvingPeriodBlocks + miner.LargestSectorGenerationAttackThresholdBlocks

// A Processor processes all the messages in a block or tip set.
type Processor interface {
	// ProcessBlock processes all messages in a block.
	ProcessBlock(context.Context, state.Tree, vm.StorageMap, *types.Block, []*types.SignedMessage, []types.TipSet) ([]*ApplicationResult, error)

	// ProcessTipSet processes all messages in a tip set.
	ProcessTipSet(context.Context, state.Tree, vm.StorageMap, types.TipSet, [][]*types.SignedMessage, []types.TipSet) (*ProcessTipSetResponse, error)
}

// Expected implements expected consensus.
type Expected struct {
	// PwrTableView provides miner and total power for the EC chain weight
	// computation.
	PwrTableView PowerTableView

	// validator provides a set of methods used to validate a block.
	BlockValidator

	// cstore is used for loading state trees during message running.
	cstore *hamt.CborIpldStore

	// bstore contains data referenced by actors within the state
	// during message running.  Additionally bstore is used for
	// accessing the power table.
	bstore blockstore.Blockstore

	// processor is what we use to process messages and pay rewards
	processor Processor

	genesisCid cid.Cid

	verifier verification.Verifier

	blockTime time.Duration
}

// Ensure Expected satisfies the Protocol interface at compile time.
var _ Protocol = (*Expected)(nil)

// NewExpected is the constructor for the Expected consenus.Protocol module.
func NewExpected(cs *hamt.CborIpldStore, bs blockstore.Blockstore, processor Processor, v BlockValidator, pt PowerTableView, gCid cid.Cid, verifier verification.Verifier, bt time.Duration) *Expected {
	return &Expected{
		cstore:         cs,
		blockTime:      bt,
		bstore:         bs,
		processor:      processor,
		PwrTableView:   pt,
		genesisCid:     gCid,
		verifier:       verifier,
		BlockValidator: v,
	}
}

// BlockTime returns the block time used by the consensus protocol.
func (c *Expected) BlockTime() time.Duration {
	return c.blockTime
}

// Weight returns the EC weight of this TipSet in uint64 encoded fixed point
// representation.
func (c *Expected) Weight(ctx context.Context, ts types.TipSet, pSt state.Tree) (uint64, error) {
	ctx = log.Start(ctx, "Expected.Weight")
	log.LogKV(ctx, "Weight", ts.String())
	if ts.Len() == 1 && ts.At(0).Cid().Equals(c.genesisCid) {
		return uint64(0), nil
	}
	// Compute parent weight.
	parentW, err := ts.ParentWeight()
	if err != nil {
		return uint64(0), err
	}

	w, err := types.FixedToBig(parentW)
	if err != nil {
		return uint64(0), err
	}
	// Each block in the tipset adds ECV + ECPrm * miner_power to parent weight.
	totalBytes, err := c.PwrTableView.Total(ctx, pSt, c.bstore)
	if err != nil {
		return uint64(0), err
	}
	floatTotalBytes := new(big.Float).SetInt(totalBytes.BigInt())
	floatECV := new(big.Float).SetInt64(int64(ECV))
	floatECPrM := new(big.Float).SetInt64(int64(ECPrM))
	for _, blk := range ts.ToSlice() {
		minerBytes, err := c.PwrTableView.Miner(ctx, pSt, c.bstore, blk.Miner)
		if err != nil {
			return uint64(0), err
		}
		floatOwnBytes := new(big.Float).SetInt(minerBytes.BigInt())
		wBlk := new(big.Float)
		wBlk.Quo(floatOwnBytes, floatTotalBytes)
		wBlk.Mul(wBlk, floatECPrM) // Power addition
		wBlk.Add(wBlk, floatECV)   // Constant addition
		w.Add(w, wBlk)
	}
	return types.BigToFixed(w)
}

// IsHeavier returns true if tipset a is heavier than tipset b, and false
// vice versa.  In the rare case where two tipsets have the same weight ties
// are broken by taking the tipset with the smallest ticket.  In the event that
// tickets are the same, IsHeavier will break ties by comparing the
// concatenation of block cids in the tipset.
// TODO BLOCK CID CONCAT TIE BREAKER IS NOT IN THE SPEC AND SHOULD BE
// EVALUATED BEFORE GETTING TO PRODUCTION.
func (c *Expected) IsHeavier(ctx context.Context, a, b types.TipSet, aStateID, bStateID cid.Cid) (bool, error) {
	var aSt, bSt state.Tree
	var err error
	if aStateID.Defined() {
		aSt, err = c.loadStateTree(ctx, aStateID)
		if err != nil {
			return false, err
		}
	}
	if bStateID.Defined() {
		bSt, err = c.loadStateTree(ctx, bStateID)
		if err != nil {
			return false, err
		}
	}

	aW, err := c.Weight(ctx, a, aSt)
	if err != nil {
		return false, err
	}
	bW, err := c.Weight(ctx, b, bSt)
	if err != nil {
		return false, err
	}

	// Without ties pass along the comparison.
	if aW != bW {
		return aW > bW, nil
	}

	// To break ties compare the min tickets.
	aTicket, err := a.MinTicket()
	if err != nil {
		return false, err
	}
	bTicket, err := b.MinTicket()
	if err != nil {
		return false, err
	}

	cmp := bytes.Compare(bTicket, aTicket)
	if cmp != 0 {
		// a is heavier if b's ticket is greater than a's ticket.
		return cmp == 1, nil
	}

	// Tie break on cid ids.
	// TODO: I think this is drastically impacted by number of blocks in tipset
	// i.e. bigger tipset is always heavier.  Not sure if this is ok, need to revist.
	cmp = strings.Compare(a.String(), b.String())
	if cmp == 0 {
		// Caller is mistakenly calling on two identical tipsets.
		return false, ErrUnorderedTipSets
	}
	return cmp == 1, nil
}

// RunStateTransition applies the messages in a tipset to a state, and persists that new state.
// It errors if the tipset was not mined according to the EC rules, or if any of the messages
// in the tipset results in an error.
func (c *Expected) RunStateTransition(ctx context.Context, ts types.TipSet, tsMessages [][]*types.SignedMessage, tsReceipts [][]*types.MessageReceipt, ancestors []types.TipSet, priorStateID cid.Cid) (root cid.Cid, err error) {
	ctx, span := trace.StartSpan(ctx, "Expected.RunStateTransition")
	span.AddAttributes(trace.StringAttribute("tipset", ts.String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	for i := 0; i < ts.Len(); i++ {
		if err := c.BlockValidator.ValidateSemantic(ctx, ts.At(i), &ancestors[0]); err != nil {
			return cid.Undef, err
		}
	}

	priorState, err := c.loadStateTree(ctx, priorStateID)
	if err != nil {
		return cid.Undef, err
	}

	if err := c.validateMining(ctx, priorState, ts, ancestors[0]); err != nil {
		return cid.Undef, err
	}

	vms := vm.NewStorageMap(c.bstore)
	st, err := c.runMessages(ctx, priorState, vms, ts, tsMessages, tsReceipts, ancestors)
	if err != nil {
		return cid.Undef, err
	}
	err = vms.Flush()
	if err != nil {
		return cid.Undef, err
	}

	return st.Flush(ctx)
}

// validateMining checks validity of the block ticket, proof, and miner address.
//    Returns an error if:
//    	* any tipset's block was mined by an invalid miner address.
//      * the block proof is invalid for the challenge
//      * the block ticket fails the power check, i.e. is not a winning ticket
//    Returns nil if all the above checks pass.
// See https://github.com/filecoin-project/specs/blob/master/mining.md#chain-validation
func (c *Expected) validateMining(ctx context.Context, st state.Tree, ts types.TipSet, parentTs types.TipSet) error {
	for i := 0; i < ts.Len(); i++ {
		blk := ts.At(i)
		// TODO: Also need to validate BlockSig

		// TODO: Once we've picked a delay function (see #2119), we need to
		// verify its proof here. The proof will likely be written to a field on
		// the mined block.

		// See https://github.com/filecoin-project/specs/blob/master/mining.md#ticket-checking
		result, err := IsWinningTicket(ctx, c.bstore, c.PwrTableView, st, blk.Ticket, blk.Miner)
		if err != nil {
			return errors.Wrap(err, "can't check for winning ticket")
		}

		if !result {
			return errors.New("not a winning ticket")
		}
	}
	return nil
}

// IsWinningTicket fetches miner power & total power, returns true if it's a winning ticket, false if not,
//    errors out if minerPower or totalPower can't be found.
//    See https://github.com/filecoin-project/specs/blob/master/expected-consensus.md
//    for an explanation of the math here.
func IsWinningTicket(ctx context.Context, bs blockstore.Blockstore, ptv PowerTableView, st state.Tree,
	ticket types.Signature, miner address.Address) (bool, error) {

	totalPower, err := ptv.Total(ctx, st, bs)
	if err != nil {
		return false, errors.Wrap(err, "Couldn't get totalPower")
	}

	minerPower, err := ptv.Miner(ctx, st, bs, miner)
	if err != nil {
		return false, errors.Wrap(err, "Couldn't get minerPower")
	}

	return CompareTicketPower(ticket, minerPower, totalPower), nil
}

// CompareTicketPower abstracts the actual comparison logic so it can be used by some test
// helpers
func CompareTicketPower(ticket types.Signature, minerPower *types.BytesAmount, totalPower *types.BytesAmount) bool {
	lhs := &big.Int{}
	lhs.SetBytes(ticket)
	lhs.Mul(lhs, totalPower.BigInt())
	rhs := &big.Int{}
	rhs.Mul(minerPower.BigInt(), ticketDomain)
	return lhs.Cmp(rhs) < 0
}

// CreateChallengeSeed creates/recreates the block challenge for purposes of validation.
//   TODO -- in general this won't work with only the base tipset.
//     We'll potentially need some chain manager utils, similar to
//     the State function, to sample further back in the chain.
func CreateChallengeSeed(parents types.TipSet, nullBlkCount uint64) (types.PoStChallengeSeed, error) {
	smallest, err := parents.MinTicket()
	if err != nil {
		return types.PoStChallengeSeed{}, err
	}

	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, nullBlkCount)
	buf = append(smallest, buf[:n]...)

	h := sha256.Sum256(buf)
	return h, nil
}

// runMessages applies the messages of all blocks within the input
// tipset to the input base state.  Messages are applied block by
// block with blocks sorted by their ticket bytes.  The output state must be
// flushed after calling to guarantee that the state transitions propagate.
//
// An error is returned if individual blocks contain messages that do not
// lead to successful state transitions.  An error is also returned if the node
// faults while running aggregate state computation.
func (c *Expected) runMessages(ctx context.Context, st state.Tree, vms vm.StorageMap, ts types.TipSet, tsMessages [][]*types.SignedMessage, tsReceipts [][]*types.MessageReceipt, ancestors []types.TipSet) (state.Tree, error) {
	var cpySt state.Tree

	// TODO: don't process messages twice
	for i := 0; i < ts.Len(); i++ {
		blk := ts.At(i)
		cpyCid, err := st.Flush(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "error validating block state")
		}
		// state copied so changes don't propagate between block validations
		cpySt, err = c.loadStateTree(ctx, cpyCid)
		if err != nil {
			return nil, errors.Wrap(err, "error validating block state")
		}

		receipts, err := c.processor.ProcessBlock(ctx, cpySt, vms, blk, tsMessages[i], ancestors)
		if err != nil {
			return nil, errors.Wrap(err, "error validating block state")
		}
		// TODO: check that receipts actually match
		if len(receipts) != len(tsReceipts[i]) {
			return nil, fmt.Errorf("found invalid message receipts: %v %v", receipts, blk.MessageReceipts)
		}

		outCid, err := cpySt.Flush(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "error validating block state")
		}
		if !outCid.Equals(blk.StateRoot) {
			return nil, ErrStateRootMismatch
		}
	}
	if ts.Len() <= 1 { // block validation state == aggregate parent state
		return cpySt, nil
	}
	// multiblock tipsets require reapplying messages to get aggregate state
	// NOTE: It is possible to optimize further by applying block validation
	// in sorted order to reuse first block transitions as the starting state
	// for the tipSetProcessor.
	_, err := c.processor.ProcessTipSet(ctx, st, vms, ts, tsMessages, ancestors)
	if err != nil {
		return nil, errors.Wrap(err, "error validating tipset")
	}
	return st, nil
}

func (c *Expected) loadStateTree(ctx context.Context, id cid.Cid) (state.Tree, error) {
	return state.LoadStateTree(ctx, c.cstore, id, builtin.Actors)
}

// CreateTicket computes a valid ticket.
// 	params:  proof  []byte, the proof to sign
// 			 signerAddr address.Address, the the signer's address. Must exist in the wallet
//      	 signer, implements TicketSigner interface. Must have signerPubKey in its keyinfo.
//  returns:  types.Signature ( []byte ), error
func CreateTicket(proof types.PoStProof, signerAddr address.Address, signer TicketSigner) (types.Signature, error) {

	buf := append(proof[:], signerAddr.Bytes()...)
	// Don't hash it here; it gets hashed in walletutil.Sign
	return signer.SignBytes(buf[:], signerAddr)
}

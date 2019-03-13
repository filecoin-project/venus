package consensus

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"
	"gx/ipfs/QmcTzQXRcU2vf8yX5EEboz1BSvWC7wWmeYAKVQmhp8WZYU/sha256-simd"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs"
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
	// Currently this is a secp256.Sign signature, which is 65 bytes.
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

// TODO none of these parameters are chosen correctly
// with respect to analysis under a security model:
// https://github.com/filecoin-project/go-filecoin/issues/1846

// ECV is the constant V defined in the EC spec.
const ECV uint64 = 10

// ECPrM is the power ratio magnitude defined in the EC spec.
const ECPrM uint64 = 100

// AncestorRoundsNeeded is the number of rounds of the ancestor chain needed
// to process all state transitions.
var AncestorRoundsNeeded = miner.ProvingPeriodBlocks.Add(miner.GracePeriodBlocks)

// A Processor processes all the messages in a block or tip set.
type Processor interface {
	// ProcessBlock processes all messages in a block.
	ProcessBlock(ctx context.Context, st state.Tree, vms vm.StorageMap, blk *types.Block, ancestors []types.TipSet) ([]*ApplicationResult, error)

	// ProcessTipSet processes all messages in a tip set.
	ProcessTipSet(ctx context.Context, st state.Tree, vms vm.StorageMap, ts types.TipSet, ancestors []types.TipSet) (*ProcessTipSetResponse, error)
}

// Expected implements expected consensus.
type Expected struct {
	// PwrTableView provides miner and total power for the EC chain weight
	// computation.
	PwrTableView PowerTableView

	// cstore is used for loading state trees during message running.
	cstore *hamt.CborIpldStore

	// bstore contains data referenced by actors within the state
	// during message running.  Additionally bstore is used for
	// accessing the power table.
	bstore blockstore.Blockstore

	// processor is what we use to process messages and pay rewards
	processor Processor

	genesisCid cid.Cid

	verifier proofs.Verifier
}

// Ensure Expected satisfies the Protocol interface at compile time.
var _ Protocol = (*Expected)(nil)

// NewExpected is the constructor for the Expected consenus.Protocol module.
func NewExpected(cs *hamt.CborIpldStore, bs blockstore.Blockstore, processor Processor, pt PowerTableView, gCid cid.Cid, verifier proofs.Verifier) Protocol {
	return &Expected{
		cstore:       cs,
		bstore:       bs,
		processor:    processor,
		PwrTableView: pt,
		genesisCid:   gCid,
		verifier:     verifier,
	}
}

// NewValidTipSet creates a new tipset from the input blocks that is guaranteed
// to be valid. It operates by validating each block and further checking that
// this tipset contains only blocks with the same heights, parent weights,
// and parent sets.
func (c *Expected) NewValidTipSet(ctx context.Context, blks []*types.Block) (types.TipSet, error) {
	for _, blk := range blks {
		if err := c.validateBlockStructure(ctx, blk); err != nil {
			return nil, err
		}
	}
	return types.NewTipSet(blks...)
}

// ValidateBlockStructure verifies that this block, on its own, is structurally and
// cryptographically valid. This means checking that all of its fields are
// properly filled out and its signatures are correct. Checking the validity of
// state changes must be done separately and only once the state of the
// previous block has been validated. TODO: not yet signature checking
func (c *Expected) validateBlockStructure(ctx context.Context, b *types.Block) error {
	// TODO: validate signature on block
	ctx = log.Start(ctx, "Expected.validateBlockStructure")
	log.LogKV(ctx, "ValidateBlockStructure", b.Cid().String())
	if !b.StateRoot.Defined() {
		return fmt.Errorf("block has nil StateRoot")
	}

	return nil
}

// Weight returns the EC weight of this TipSet in uint64 encoded fixed point
// representation.
func (c *Expected) Weight(ctx context.Context, ts types.TipSet, pSt state.Tree) (uint64, error) {
	ctx = log.Start(ctx, "Expected.Weight")
	log.LogKV(ctx, "Weight", ts.String())
	if len(ts) == 1 && ts.ToSlice()[0].Cid().Equals(c.genesisCid) {
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
	floatTotalBytes := new(big.Float).SetInt64(int64(totalBytes))
	floatECV := new(big.Float).SetInt64(int64(ECV))
	floatECPrM := new(big.Float).SetInt64(int64(ECPrM))
	for _, blk := range ts.ToSlice() {
		minerBytes, err := c.PwrTableView.Miner(ctx, pSt, c.bstore, blk.Miner)
		if err != nil {
			return uint64(0), err
		}
		floatOwnBytes := new(big.Float).SetInt64(int64(minerBytes))
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
func (c *Expected) IsHeavier(ctx context.Context, a, b types.TipSet, aSt, bSt state.Tree) (bool, error) {
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

// RunStateTransition is the chain transition function that goes from a
// starting state and a tipset to a new state.  It errors if the tipset was not
// mined according to the EC rules, or if running the messages in the tipset
// results in an error.
func (c *Expected) RunStateTransition(ctx context.Context, ts types.TipSet, ancestors []types.TipSet, pSt state.Tree) (state.Tree, error) {
	err := c.validateMining(ctx, pSt, ts, ancestors[0])
	if err != nil {
		return nil, err
	}

	sl := ts.ToSlice()
	one := sl[0]
	for _, blk := range sl[1:] {
		if blk.Parents.String() != one.Parents.String() {
			log.Error("invalid parents", blk.Parents.String(), one.Parents.String(), blk)
			panic("invalid parents")
		}
		if blk.Height != one.Height {
			log.Error("invalid height", blk.Height, one.Height, blk)
			panic("invalid height")
		}
	}

	vms := vm.NewStorageMap(c.bstore)
	st, err := c.runMessages(ctx, pSt, vms, ts, ancestors)
	if err != nil {
		return nil, err
	}
	err = vms.Flush()
	if err != nil {
		return nil, err
	}
	return st, nil
}

// validateMining checks validity of the block ticket, proof, and miner address.
//    Returns an error if:
//    	* any tipset's block was mined by an invalid miner address.
//      * the block proof is invalid for the challenge
//      * the block ticket fails the power check, i.e. is not a winning ticket
//    Returns nil if all the above checks pass.
// See https://github.com/filecoin-project/specs/blob/master/mining.md#chain-validation
func (c *Expected) validateMining(ctx context.Context, st state.Tree, ts types.TipSet, parentTs types.TipSet) error {
	for _, blk := range ts.ToSlice() {
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
//    See https://github.com/filecoin-project/aq/issues/70 for an explanation of the math here.
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
func CompareTicketPower(ticket types.Signature, minerPower uint64, totalPower uint64) bool {
	lhs := &big.Int{}
	lhs.SetBytes(ticket)
	lhs.Mul(lhs, big.NewInt(int64(totalPower)))
	rhs := &big.Int{}
	rhs.Mul(big.NewInt(int64(minerPower)), ticketDomain)
	return lhs.Cmp(rhs) < 0
}

// CreateChallengeSeed creates/recreates the block challenge for purposes of validation.
//   TODO -- in general this won't work with only the base tipset.
//     We'll potentially need some chain manager utils, similar to
//     the State function, to sample further back in the chain.
func CreateChallengeSeed(parents types.TipSet, nullBlkCount uint64) (proofs.PoStChallengeSeed, error) {
	smallest, err := parents.MinTicket()
	if err != nil {
		return proofs.PoStChallengeSeed{}, err
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
func (c *Expected) runMessages(ctx context.Context, st state.Tree, vms vm.StorageMap, ts types.TipSet, ancestors []types.TipSet) (state.Tree, error) {
	var cpySt state.Tree

	// TODO: order blocks in the tipset by ticket
	// TODO: don't process messages twice
	for _, blk := range ts.ToSlice() {
		cpyCid, err := st.Flush(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "error validating block state")
		}
		// state copied so changes don't propagate between block validations
		cpySt, err = state.LoadStateTree(ctx, c.cstore, cpyCid, builtin.Actors)
		if err != nil {
			return nil, errors.Wrap(err, "error validating block state")
		}

		receipts, err := c.processor.ProcessBlock(ctx, cpySt, vms, blk, ancestors)
		if err != nil {
			return nil, errors.Wrap(err, "error validating block state")
		}
		// TODO: check that receipts actually match
		if len(receipts) != len(blk.MessageReceipts) {
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
	if len(ts) == 1 { // block validation state == aggregate parent state
		return cpySt, nil
	}
	// multiblock tipsets require reapplying messages to get aggregate state
	// NOTE: It is possible to optimize further by applying block validation
	// in sorted order to reuse first block transitions as the starting state
	// for the tipSetProcessor.
	_, err := c.processor.ProcessTipSet(ctx, st, vms, ts, ancestors)
	if err != nil {
		return nil, errors.Wrap(err, "error validating tipset")
	}
	return st, nil
}

// CreateTicket computes a valid ticket.
// 	params:  proof  []byte, the proof to sign
// 			 signerPubKey []byte, the public key for the signer. Must exist in the signer
//      	 signer, implements TicketSigner interface. Must have signerPubKey in its keyinfo.
//  returns:  types.Signature ( []byte ), error
func CreateTicket(proof proofs.PoStProof, signerPubKey []byte, signer TicketSigner) (types.Signature, error) {

	var ticket types.Signature

	signerAddr, err := signer.GetAddressForPubKey(signerPubKey)
	if err != nil {
		return ticket, errors.Wrap(err, "could not get address for signerPubKey")
	}
	buf := append(proof[:], signerAddr.Bytes()...)
	// Don't hash it here; it gets hashed in walletutil.Sign
	return signer.SignBytes(buf[:], signerAddr)
}

package consensus

// This is to implement Expected Consensus protocol
// See: https://github.com/filecoin-project/specs/blob/master/expected-consensus.md

import (
	"context"
	"math/big"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/crypto"
	"github.com/filecoin-project/go-filecoin/metrics/tracing"
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

// DefaultBlockTime is the estimated proving period time.
// We define this so that we can fake mining in the current incomplete system.
// We also use this to enforce a soft block validation.
const DefaultBlockTime = 30 * time.Second

// AncestorRoundsNeeded is the number of rounds of the ancestor chain needed
// to process all state transitions.
//
// TODO: If the following PR is merged - and the network doesn't define a
// largest sector size - this constant will need to be reconsidered.
// https://github.com/filecoin-project/specs/pull/318
// NOTE(anorth): This height is excessive, but safe, with the Rational PoSt construction.
const AncestorRoundsNeeded = miner.LargestSectorSizeProvingPeriodBlocks + miner.PoStChallengeWindowBlocks

// A Processor processes all the messages in a block or tip set.
type Processor interface {
	// ProcessBlock processes all messages in a block.
	ProcessBlock(context.Context, state.Tree, vm.StorageMap, *types.Block, []*types.SignedMessage, []types.TipSet) ([]*ApplicationResult, error)

	// ProcessTipSet processes all messages in a tip set.
	ProcessTipSet(context.Context, state.Tree, vm.StorageMap, types.TipSet, [][]*types.SignedMessage, []types.TipSet) (*ProcessTipSetResponse, error)
}

// TicketValidator validates that an input ticket is valid.
type TicketValidator interface {
	IsValidTicket(parent types.Ticket, ticket types.Ticket, signerAddr address.Address) bool
}

// ElectionValidator validates that an election fairly produced a winner.
type ElectionValidator interface {
	IsElectionWinner(context.Context, PowerTableView, types.Ticket, types.VRFPi, address.Address, address.Address) (bool, error)
}

// SnapshotGenerator produces snapshots to examine actor state
type SnapshotGenerator interface {
	StateTreeSnapshot(st state.Tree, bh *types.BlockHeight) ActorStateSnapshot
}

// Expected implements expected consensus.
type Expected struct {
	// validator provides a set of methods used to validate a block.
	BlockValidator

	// ElectionValidator validates election proofs.
	ElectionValidator

	// TicketValidator validates ticket generation
	TicketValidator

	// cstore is used for loading state trees during message running.
	cstore *hamt.CborIpldStore

	// bstore contains data referenced by actors within the state
	// during message running.  Additionally bstore is used for
	// accessing the power table.
	bstore blockstore.Blockstore

	// processor is what we use to process messages and pay rewards
	processor Processor

	genesisCid cid.Cid

	// actorState provides produces snapshots
	actorState SnapshotGenerator

	blockTime time.Duration
}

// Ensure Expected satisfies the Protocol interface at compile time.
var _ Protocol = (*Expected)(nil)

// NewExpected is the constructor for the Expected consenus.Protocol module.
func NewExpected(cs *hamt.CborIpldStore, bs blockstore.Blockstore, processor Processor, v BlockValidator, actorState SnapshotGenerator, gCid cid.Cid, bt time.Duration, ev ElectionValidator, tv TicketValidator) *Expected {
	return &Expected{
		cstore:            cs,
		blockTime:         bt,
		bstore:            bs,
		processor:         processor,
		actorState:        actorState,
		genesisCid:        gCid,
		BlockValidator:    v,
		ElectionValidator: ev,
		TicketValidator:   tv,
	}
}

// BlockTime returns the block time used by the consensus protocol.
func (c *Expected) BlockTime() time.Duration {
	return c.blockTime
}

// RunStateTransition applies the messages in a tipset to a state, and persists that new state.
// It errors if the tipset was not mined according to the EC rules, or if any of the messages
// in the tipset results in an error.
func (c *Expected) RunStateTransition(ctx context.Context, ts types.TipSet, blsMessages [][]*types.MeteredMessage, secpMessages [][]*types.SignedMessage, tsReceipts [][]*types.MessageReceipt, ancestors []types.TipSet, parentWeight uint64, priorStateID cid.Cid) (root cid.Cid, err error) {
	ctx, span := trace.StartSpan(ctx, "Expected.RunStateTransition")
	span.AddAttributes(trace.StringAttribute("tipset", ts.String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	for i := 0; i < ts.Len(); i++ {
		if err := c.BlockValidator.ValidateSemantic(ctx, ts.At(i), &ancestors[0], parentWeight); err != nil {
			return cid.Undef, err
		}
	}

	priorState, err := c.loadStateTree(ctx, priorStateID)
	if err != nil {
		return cid.Undef, err
	}

	if err := c.validateMining(ctx, priorState, ts, ancestors[0], blsMessages); err != nil {
		return cid.Undef, err
	}

	vms := vm.NewStorageMap(c.bstore)
	st, err := c.runMessages(ctx, priorState, vms, ts, blsMessages, secpMessages, tsReceipts, ancestors)
	if err != nil {
		return cid.Undef, err
	}
	err = vms.Flush()
	if err != nil {
		return cid.Undef, err
	}

	return st.Flush(ctx)
}

// validateMining checks validity of the ticket, proof, signature and miner
// address of every block in the tipset.
//    Returns an error if any block:
//    	* is mined by a miner not in the power table
//      * is not validly signed by the miner's worker key
//      * has an invalid election proof
//      * has an invalid ticket
//      * has a losing election proof
//    Returns nil if all the above checks pass.
// See https://github.com/filecoin-project/specs/blob/master/mining.md#chain-validation
func (c *Expected) validateMining(ctx context.Context, st state.Tree, ts types.TipSet, parentTs types.TipSet, blsMsgs [][]*types.MeteredMessage) error {
	prevTicket, err := parentTs.MinTicket()
	if err != nil {
		return errors.Wrap(err, "failed to read parent min ticket")
	}
	prevHeight, err := parentTs.Height()
	if err != nil {
		return errors.Wrap(err, "failed to read parent height")
	}

	pwrTableView := c.createPowerTableView(st)

	for i := 0; i < ts.Len(); i++ {
		blk := ts.At(i)

		workerAddr, err := pwrTableView.WorkerAddr(ctx, blk.Miner)
		if err != nil {
			return errors.Wrap(err, "failed to read worker address of block miner")
		}
		// Validate block signature
		if valid := types.IsValidSignature(blk.SignatureData(), workerAddr, blk.BlockSig); !valid {
			return errors.New("block signature invalid")
		}

		// Verify that the BLS signature is correct
		if err := verifyBLSMessageAggregate(blk.BLSAggregateSig, blsMsgs[i]); err != nil {
			return errors.Wrapf(err, "bls message verification failed for block %s", blk.Cid())
		}

		// Validate ElectionProof
		numTickets := len(blk.Tickets)
		result, err := c.IsElectionWinner(ctx, pwrTableView, blk.Tickets[numTickets-1], blk.ElectionProof, workerAddr, blk.Miner)
		if err != nil {
			return errors.Wrap(err, "failed checking election proof")
		}
		if !result {
			return errors.New("block author did not win election")
		}

		// Validate ticket array
		// Block has same number of tickets as increase in height
		if uint64(len(blk.Tickets)) != uint64(blk.Height)-prevHeight {
			return errors.Errorf("invalid ticket array length. Expected: %d, actual %d", uint64(blk.Height)-prevHeight, len(blk.Tickets))
		}

		// All tickets were correctly generated by miner
		prevTickets := append([]types.Ticket{prevTicket}, blk.Tickets[:len(blk.Tickets)-1]...)
		for i := 0; i < len(blk.Tickets); i++ {
			if !c.IsValidTicket(prevTickets[i], blk.Tickets[i], workerAddr) {
				return errors.Errorf("invalid ticket: %s in position %d in block %s", blk.Tickets[i].String(), i, blk.Cid().String())
			}
		}
	}
	return nil
}

// runMessages applies the messages of all blocks within the input
// tipset to the input base state.  Messages are applied block by
// block with blocks sorted by their ticket bytes.  The output state must be
// flushed after calling to guarantee that the state transitions propagate.
//
// An error is returned if individual blocks contain messages that do not
// lead to successful state transitions.  An error is also returned if the node
// faults while running aggregate state computation.
func (c *Expected) runMessages(ctx context.Context, st state.Tree, vms vm.StorageMap, ts types.TipSet, blsMessages [][]*types.MeteredMessage, secpMessages [][]*types.SignedMessage, tsReceipts [][]*types.MessageReceipt, ancestors []types.TipSet) (state.Tree, error) {
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

		// wrap bls messages and combine to process bls messages first
		msgs := append(wrapMessages(blsMessages[i]), secpMessages[i]...)
		receipts, err := c.processor.ProcessBlock(ctx, cpySt, vms, blk, msgs, ancestors)
		if err != nil {
			return nil, errors.Wrap(err, "error validating block state")
		}
		// TODO: check that receipts actually match
		if len(receipts) != len(tsReceipts[i]) {
			return nil, errors.Errorf("found invalid message receipts: %v %v", receipts, blk.MessageReceipts)
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
	allMessages := combineMessages(blsMessages, secpMessages)
	_, err := c.processor.ProcessTipSet(ctx, st, vms, ts, allMessages, ancestors)
	if err != nil {
		return nil, errors.Wrap(err, "error validating tipset")
	}
	return st, nil
}

func (c *Expected) createPowerTableView(st state.Tree) PowerTableView {
	snapshot := c.actorState.StateTreeSnapshot(st, nil)
	return NewPowerTableView(snapshot)
}

func (c *Expected) loadStateTree(ctx context.Context, id cid.Cid) (state.Tree, error) {
	return state.LoadStateTree(ctx, c.cstore, id)
}

// verifyBLSMessageAggregate errors if the bls signature is not a valid aggregate of message signatures
func verifyBLSMessageAggregate(sig types.Signature, msgs []*types.MeteredMessage) error {
	pubKeys := [][]byte{}
	marshalledMsgs := [][]byte{}
	for _, msg := range msgs {
		pubKeys = append(pubKeys, msg.From.Payload())
		msgBytes, err := msg.Marshal()
		if err != nil {
			return err
		}
		marshalledMsgs = append(marshalledMsgs, msgBytes)
	}
	if !crypto.VerifyBLSAggregate(pubKeys, marshalledMsgs, sig) {
		return errors.New("block BLS signature does not validate against BLS messages")
	}
	return nil
}

func combineMessages(blsMessages [][]*types.MeteredMessage, secpMessages [][]*types.SignedMessage) [][]*types.SignedMessage {
	messages := [][]*types.SignedMessage{}
	for _, msgs := range blsMessages {
		messages = append(messages, wrapMessages(msgs))
	}
	for _, msgs := range secpMessages {
		messages = append(messages, msgs)
	}
	return messages
}

func wrapMessages(blsMessages []*types.MeteredMessage) []*types.SignedMessage {
	signed := []*types.SignedMessage{}
	for _, msg := range blsMessages {
		signed = append(signed, &types.SignedMessage{MeteredMessage: *msg})
	}
	return signed
}

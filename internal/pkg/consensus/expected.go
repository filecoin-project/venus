package consensus

// This is to implement Expected Consensus protocol
// See: https://github.com/filecoin-project/specs/blob/master/expected-consensus.md

import (
	"context"
	"math/big"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics/tracing"
	"github.com/filecoin-project/go-filecoin/internal/pkg/sampling"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
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
	// ErrReceiptRootMismatch is returned when the block's receipt root doesn't match the receipt root computed for the parent tipset.
	ErrReceiptRootMismatch = errors.New("blocks receipt root does not match parent tip set")
)

// DefaultBlockTime is the estimated proving period time.
// We define this so that we can fake mining in the current incomplete system.
// We also use this to enforce a soft block validation.
const DefaultBlockTime = 30 * time.Second

// ElectionLookback is the number of tipsets past the head (inclusive)) that
// must be traversed to sample the election ticket.
const ElectionLookback = 5

// AncestorRoundsNeeded is the number of rounds of the ancestor chain needed
// to process all state transitions.
//
// TODO: If the following PR is merged - and the network doesn't define a
// largest sector size - this constant will need to be reconsidered.
// https://github.com/filecoin-project/specs/pull/318
// NOTE(anorth): This height is excessive, but safe, with the Rational PoSt construction.
var AncestorRoundsNeeded = max(miner.LargestSectorSizeProvingPeriodBlocks+miner.PoStChallengeWindowBlocks, ElectionLookback)

// A Processor processes all the messages in a block or tip set.
type Processor interface {
	// ProcessTipSet processes all messages in a tip set.
	ProcessTipSet(context.Context, state.Tree, vm.StorageMap, block.TipSet, [][]*types.UnsignedMessage, []block.TipSet) ([]*ApplyMessageResult, error)
}

// TicketValidator validates that an input ticket is valid.
type TicketValidator interface {
	IsValidTicket(parent block.Ticket, ticket block.Ticket, signerAddr address.Address) bool
}

// ElectionValidator validates that an election fairly produced a winner.
type ElectionValidator interface {
	IsElectionWinner(context.Context, PowerTableView, block.Ticket, uint64, block.VRFPi, address.Address, address.Address) (bool, error)
}

// SnapshotGenerator produces snapshots to examine actor state
type SnapshotGenerator interface {
	StateTreeSnapshot(st state.Tree, bh *types.BlockHeight) ActorStateSnapshot
}

// Expected implements expected consensus.
type Expected struct {
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

	// actorState provides produces snapshots
	actorState SnapshotGenerator

	blockTime time.Duration
}

// Ensure Expected satisfies the Protocol interface at compile time.
var _ Protocol = (*Expected)(nil)

// NewExpected is the constructor for the Expected consenus.Protocol module.
func NewExpected(cs *hamt.CborIpldStore, bs blockstore.Blockstore, processor Processor, actorState SnapshotGenerator, bt time.Duration, ev ElectionValidator, tv TicketValidator) *Expected {
	return &Expected{
		cstore:            cs,
		blockTime:         bt,
		bstore:            bs,
		processor:         processor,
		actorState:        actorState,
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
func (c *Expected) RunStateTransition(ctx context.Context, ts block.TipSet, blsMessages [][]*types.UnsignedMessage, secpMessages [][]*types.SignedMessage, ancestors []block.TipSet, parentWeight uint64, parentStateRoot cid.Cid, parentReceiptRoot cid.Cid) (root cid.Cid, receipts []*types.MessageReceipt, err error) {
	ctx, span := trace.StartSpan(ctx, "Expected.RunStateTransition")
	span.AddAttributes(trace.StringAttribute("tipset", ts.String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	priorState, err := c.loadStateTree(ctx, parentStateRoot)
	if err != nil {
		return cid.Undef, []*types.MessageReceipt{}, err
	}

	if err := c.validateMining(ctx, priorState, ts, ancestors[0], ancestors, blsMessages, secpMessages, parentWeight, parentStateRoot, parentReceiptRoot); err != nil {
		return cid.Undef, []*types.MessageReceipt{}, err
	}

	vms := vm.NewStorageMap(c.bstore)
	var st state.Tree
	st, receipts, err = c.runMessages(ctx, priorState, vms, ts, blsMessages, unwrap(secpMessages), ancestors)
	if err != nil {
		return cid.Undef, []*types.MessageReceipt{}, err
	}
	err = vms.Flush()
	if err != nil {
		return cid.Undef, []*types.MessageReceipt{}, err
	}

	root, err = st.Flush(ctx)
	if err != nil {
		return cid.Undef, []*types.MessageReceipt{}, err
	}
	return root, receipts, err
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
func (c *Expected) validateMining(
	ctx context.Context,
	st state.Tree,
	ts block.TipSet,
	parentTs block.TipSet,
	ancestors []block.TipSet,
	blsMsgs [][]*types.UnsignedMessage,
	secpMsgs [][]*types.SignedMessage,
	parentWeight uint64,
	parentStateRoot cid.Cid,
	parentReceiptRoot cid.Cid) error {

	electionTicket, err := sampling.SampleNthTicket(ElectionLookback-1, ancestors)
	if err != nil {
		return errors.Wrap(err, "failed to sample election ticket from ancestors")
	}
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

		// confirm block state root matches parent state root
		if !parentStateRoot.Equals(blk.StateRoot) {
			return ErrStateRootMismatch
		}

		// confirm block receipts match parent receipts
		if !parentReceiptRoot.Equals(blk.MessageReceipts) {
			return ErrReceiptRootMismatch
		}

		if uint64(blk.ParentWeight) != parentWeight {
			return errors.Errorf("block %s has invalid parent weight %d", blk.Cid().String(), parentWeight)
		}
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

		// Verify that all secp message signatures are correct
		for i, msg := range secpMsgs[i] {
			if !msg.VerifySignature() {
				return errors.Errorf("secp message signature invalid for message, %d, in block %s", i, blk.Cid())
			}
		}

		// Validate ElectionProof
		nullBlkCount := uint64(blk.Height) - prevHeight - 1
		result, err := c.IsElectionWinner(ctx, pwrTableView, electionTicket, nullBlkCount, blk.ElectionProof, workerAddr, blk.Miner)
		if err != nil {
			return errors.Wrap(err, "failed checking election proof")
		}
		if !result {
			return errors.New("block author did not win election")
		}

		// Ticket was correctly generated by miner
		if !c.IsValidTicket(prevTicket, blk.Ticket, workerAddr) {
			return errors.Errorf("invalid ticket: %s in block %s", blk.Ticket.String(), blk.Cid().String())
		}
	}
	return nil
}

// runMessages applies the messages of all blocks within the input
// tipset to the input base state.  Messages are extracted from tipset
// blocks sorted by their ticket bytes and run as a single state transition
// for the entire tipset. The output state must be flushed after calling to
// guarantee that the state transitions propagate.
// Messages that fail to apply are dropped on the floor (and no receipt is emitted).
func (c *Expected) runMessages(ctx context.Context, st state.Tree, vms vm.StorageMap, ts block.TipSet, blsMessages [][]*types.UnsignedMessage, secpMessages [][]*types.UnsignedMessage, ancestors []block.TipSet) (state.Tree, []*types.MessageReceipt, error) {
	allMessages := combineMessages(blsMessages, secpMessages)

	results, err := c.processor.ProcessTipSet(ctx, st, vms, ts, allMessages, ancestors)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error validating tipset")
	}

	var receipts []*types.MessageReceipt
	for _, res := range results {
		if res.Failure == nil {
			receipts = append(receipts, res.Receipt)
		}
		// Else drop the error on the floor.
	}

	return st, receipts, nil
}

func (c *Expected) createPowerTableView(st state.Tree) PowerTableView {
	snapshot := c.actorState.StateTreeSnapshot(st, nil)
	return NewPowerTableView(snapshot)
}

func (c *Expected) loadStateTree(ctx context.Context, id cid.Cid) (state.Tree, error) {
	return state.NewTreeLoader().LoadStateTree(ctx, c.cstore, id)
}

// verifyBLSMessageAggregate errors if the bls signature is not a valid aggregate of message signatures
func verifyBLSMessageAggregate(sig types.Signature, msgs []*types.UnsignedMessage) error {
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

// combineMessages takes lists of bls and secp messages grouped by block and combines them so
// that they are still grouped by block and the bls messages come before secp messages in the same block.
func combineMessages(blsMsgs [][]*types.UnsignedMessage, secpMsgs [][]*types.UnsignedMessage) [][]*types.UnsignedMessage {
	combined := make([][]*types.UnsignedMessage, len(blsMsgs))
	for blkIndex := 0; blkIndex < len(blsMsgs); blkIndex++ {
		combined[blkIndex] = append(blsMsgs[blkIndex], secpMsgs[blkIndex]...)
	}
	return combined
}

// Unwraps nested slices of signed messages.
// Much better than this function would be a type encapsulating a tipset's messages, along
// with deduplication and unwrapping logic.
func unwrap(smsgs [][]*types.SignedMessage) [][]*types.UnsignedMessage {
	unsigned := make([][]*types.UnsignedMessage, len(smsgs))
	for i, inner := range smsgs {
		unsigned[i] = types.UnwrapSigned(inner)
	}
	return unsigned
}

func max(a, b int) int {
	if a >= b {
		return a
	}
	return b
}

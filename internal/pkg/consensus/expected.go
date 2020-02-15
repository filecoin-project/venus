package consensus

import (
	"context"
	"math/big"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	fbig "github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/actors/builtin/power"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	ffi "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics/tracing"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	"github.com/filecoin-project/go-filecoin/internal/pkg/sampling"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/util/hasher"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

var (
	ticketDomain *big.Int
	log          = logging.Logger("consensus.expected")
)

func init() {
	ticketDomain = &big.Int{}
	// DEPRECATED: The size of the ticket domain must equal the size of the Signature (ticket) generated.
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

// challengeBits is the number of bits in the challenge ticket's domain
const challengeBits = 256

// expectedLeadersPerEpoch is the mean number of leaders per epoch
const expectedLeadersPerEpoch = 5

// AncestorRoundsNeeded is the number of rounds of the ancestor chain needed
// to process all state transitions.
//
// TODO: If the following PR is merged - and the network doesn't define a
// largest sector size - this constant will need to be reconsidered.
// https://github.com/filecoin-project/specs/pull/318
// NOTE(anorth): This height is excessive, but safe, with the Rational PoSt construction.
var AncestorRoundsNeeded = max(miner.ProvingPeriod+power.WindowedPostChallengeDuration, miner.ElectionLookback)

// A Processor processes all the messages in a block or tip set.
type Processor interface {
	// ProcessTipSet processes all messages in a tip set.
	ProcessTipSet(context.Context, state.Tree, vm.Storage, block.TipSet, []vm.BlockMessagesInfo) ([]vm.MessageReceipt, error)
}

// TicketValidator validates that an input ticket is valid.
type TicketValidator interface {
	IsValidTicket(parent block.Ticket, ticket block.Ticket, signerAddr address.Address) bool
}

// ElectionValidator validates that an election fairly produced a winner.
type ElectionValidator interface {
	VerifyPoSt(ep verification.PoStVerifier, allSectorInfos ffi.SortedPublicSectorInfo, sectorSize uint64, challengeSeed []byte, proof []byte, candidates []block.EPoStCandidate, proverID address.Address) (bool, error)
	CandidateWins(challengeTicket []byte, sectorNum, faultNum, networkPower, sectorSize uint64) bool
	VerifyPoStRandomness(rand block.VRFPi, ticket block.Ticket, candidateAddr address.Address, nullBlockCount uint64) bool
}

// StateViewer provides views into the chain state.
type StateViewer interface {
	StateView(root cid.Cid) PowerStateView
}

// Expected implements expected consensus.
type Expected struct {
	// ElectionValidator validates election proofs.
	ElectionValidator

	// TicketValidator validates ticket generation
	TicketValidator

	// cstore is used for loading state trees during message running.
	cstore cbor.IpldStore

	// bstore contains data referenced by actors within the state
	// during message running.  Additionally bstore is used for
	// accessing the power table.
	bstore blockstore.Blockstore

	// processor is what we use to process messages and pay rewards
	processor Processor

	// state provides produces snapshots
	state StateViewer

	blockTime time.Duration

	// postVerifier verifies PoSt proofs and associated data
	postVerifier verification.PoStVerifier
}

// Ensure Expected satisfies the Protocol interface at compile time.
var _ Protocol = (*Expected)(nil)

// NewExpected is the constructor for the Expected consenus.Protocol module.
func NewExpected(cs cbor.IpldStore, bs blockstore.Blockstore, processor Processor, state StateViewer, bt time.Duration, ev ElectionValidator, tv TicketValidator, pv verification.PoStVerifier) *Expected {
	return &Expected{
		cstore:            cs,
		blockTime:         bt,
		bstore:            bs,
		processor:         processor,
		state:             state,
		ElectionValidator: ev,
		TicketValidator:   tv,
		postVerifier:      pv,
	}
}

// BlockTime returns the block time used by the consensus protocol.
func (c *Expected) BlockTime() time.Duration {
	return c.blockTime
}

// RunStateTransition applies the messages in a tipset to a state, and persists that new state.
// It errors if the tipset was not mined according to the EC rules, or if any of the messages
// in the tipset results in an error.
func (c *Expected) RunStateTransition(ctx context.Context, ts block.TipSet, blsMessages [][]*types.UnsignedMessage, secpMessages [][]*types.SignedMessage, ancestors []block.TipSet, parentWeight fbig.Int, parentStateRoot cid.Cid, parentReceiptRoot cid.Cid) (root cid.Cid, receipts []vm.MessageReceipt, err error) {
	ctx, span := trace.StartSpan(ctx, "Expected.RunStateTransition")
	span.AddAttributes(trace.StringAttribute("tipset", ts.String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	if err := c.validateMining(ctx, ts, parentStateRoot, ancestors[0], ancestors, blsMessages, secpMessages, parentWeight, parentReceiptRoot); err != nil {
		return cid.Undef, []vm.MessageReceipt{}, err
	}

	priorState, err := c.loadStateTree(ctx, parentStateRoot)
	if err != nil {
		return cid.Undef, []vm.MessageReceipt{}, err
	}
	vms := vm.NewStorage(c.bstore)
	var newState state.Tree
	newState, receipts, err = c.runMessages(ctx, priorState, vms, ts, blsMessages, secpMessages, ancestors)
	if err != nil {
		return cid.Undef, []vm.MessageReceipt{}, err
	}
	err = vms.Flush()
	if err != nil {
		return cid.Undef, []vm.MessageReceipt{}, err
	}

	root, err = newState.Flush(ctx)
	if err != nil {
		return cid.Undef, []vm.MessageReceipt{}, err
	}
	return root, receipts, err
}

// validateMining checks validity of the ticket, proof, signature and miner
// address of every block in the tipset.
func (c *Expected) validateMining(
	ctx context.Context,
	ts block.TipSet,
	parentStateRoot cid.Cid,
	parentTs block.TipSet,
	ancestors []block.TipSet,
	blsMsgs [][]*types.UnsignedMessage,
	secpMsgs [][]*types.SignedMessage,
	parentWeight fbig.Int,
	parentReceiptRoot cid.Cid) error {

	electionTicket, err := sampling.SampleNthTicket(int(miner.ElectionLookback-1), ancestors)
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

	powerTable := NewPowerTableView(c.state.StateView(parentStateRoot))

	for i := 0; i < ts.Len(); i++ {
		blk := ts.At(i)

		// confirm block state root matches parent state root
		if !parentStateRoot.Equals(blk.StateRoot.Cid) {
			return ErrStateRootMismatch
		}

		// confirm block receipts match parent receipts
		if !parentReceiptRoot.Equals(blk.MessageReceipts.Cid) {
			return ErrReceiptRootMismatch
		}

		if !parentWeight.Equals(blk.ParentWeight) {
			return errors.Errorf("block %s has invalid parent weight %d", blk.Cid().String(), parentWeight)
		}
		workerAddr, err := powerTable.WorkerAddr(ctx, blk.Miner)
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

		// Verify PoStRandomness
		nullBlkCount := blk.Height - prevHeight - 1
		if !c.VerifyPoStRandomness(blk.EPoStInfo.PoStRandomness, electionTicket, workerAddr, nullBlkCount) {
			return errors.New("PoStRandomness invalid")
		}

		// Verify all partial tickets are winners
		sectorNum, err := powerTable.NumSectors(ctx, blk.Miner)
		if err != nil {
			return errors.Wrap(err, "failed to read sectorNum from power table")
		}
		networkPower, err := powerTable.Total(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to read networkPower from power table")
		}
		sectorSize, err := powerTable.SectorSize(ctx, blk.Miner)
		if err != nil {
			return errors.Wrap(err, "failed to read sectorSize from power table")
		}
		hasher := hasher.NewHasher()
		for i, candidate := range blk.EPoStInfo.Winners {
			hasher.Bytes(candidate.PartialTicket)
			// Dragons: must pass fault count value here, not zero.
			if !c.ElectionValidator.CandidateWins(hasher.Hash(), sectorNum, 0, networkPower.Uint64(), uint64(sectorSize)) {
				return errors.Errorf("partial ticket %d lost election", i)
			}
		}

		// Verify PoSt is valid
		allSectorInfos, err := powerTable.SortedSectorInfos(ctx, blk.Miner)
		if err != nil {
			return errors.Wrapf(err, "failed to read sector infos from power table")
		}
		valid, err := c.VerifyPoSt(c.postVerifier, allSectorInfos, uint64(sectorSize), blk.EPoStInfo.PoStRandomness, blk.EPoStInfo.PoStProof, blk.EPoStInfo.Winners, blk.Miner)
		if err != nil {
			return errors.Wrapf(err, "error checking PoSt")
		}
		if !valid {
			return errors.Errorf("invalid PoSt")
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
func (c *Expected) runMessages(ctx context.Context, st state.Tree, vms vm.Storage, ts block.TipSet, blsMessages [][]*types.UnsignedMessage, secpMessages [][]*types.SignedMessage, ancestors []block.TipSet) (state.Tree, []vm.MessageReceipt, error) {
	msgs := []vm.BlockMessagesInfo{}

	// build message information per block
	for i := 0; i < ts.Len(); i++ {
		blk := ts.At(i)

		msgInfo := vm.BlockMessagesInfo{
			BLSMessages:  blsMessages[i],
			SECPMessages: secpMessages[i],
			Miner:        blk.Miner,
		}

		msgs = append(msgs, msgInfo)
	}

	// process tipset
	receipts, err := c.processor.ProcessTipSet(ctx, st, vms, ts, msgs)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error validating tipset")
	}

	return st, receipts, nil
}

func (c *Expected) loadStateTree(ctx context.Context, id cid.Cid) (state.Tree, error) {
	return state.NewTreeLoader().LoadStateTree(ctx, c.cstore, id)
}

// PowerStateViewer a state viewer to the power state view interface.
type PowerStateViewer struct {
	*appstate.Viewer
}

// AsPowerStateViewer adapts a state viewer to a power state viewer.
func AsPowerStateViewer(v *appstate.Viewer) PowerStateViewer {
	return PowerStateViewer{v}
}

// StateView returns a power state view for a state root.
func (p *PowerStateViewer) StateView(root cid.Cid) PowerStateView {
	return p.Viewer.StateView(root)
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

func max(a, b abi.ChainEpoch) abi.ChainEpoch {
	if a >= b {
		return a
	}
	return b
}

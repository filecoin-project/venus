package consensus

import (
	"context"
	"time"

	address "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	cid "github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/drand"
	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics/tracing"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

var (
	// ErrStateRootMismatch is returned when the computed state root doesn't match the expected result.
	ErrStateRootMismatch = errors.New("blocks state root does not match computed result")
	// ErrUnorderedTipSets is returned when weight and minticket are the same between two tipsets.
	ErrUnorderedTipSets = errors.New("trying to order two identical tipsets")
	// ErrReceiptRootMismatch is returned when the block's receipt root doesn't match the receipt root computed for the parent tipset.
	ErrReceiptRootMismatch = errors.New("blocks receipt root does not match parent tip set")
)

const AllowableClockDriftSecs = uint64(1)

// challengeBits is the number of bits in the challenge ticket's domain
const challengeBits = 256

// expectedLeadersPerEpoch is the mean number of leaders per epoch
const expectedLeadersPerEpoch = 5

// WinningPoStSectorSetLookback is the past epoch offset for reading the
// winning post sector set
const WinningPoStSectorSetLookback = 10

// ElectionPowerTableLookback is the past epoch offset for reading the
// election power values
const ElectionPowerTableLookback = 10

// DRANDEpochLookback is the past filecoin epoch offset at which DRAND entries
// in that epoch should be included in a block.
const DRANDEpochLookback = 2

// A Processor processes all the messages in a block or tip set.
type Processor interface {
	// ProcessTipSet processes all messages in a tip set.
	ProcessTipSet(context.Context, state.Tree, vm.Storage, block.TipSet, block.TipSet, []vm.BlockMessagesInfo) ([]vm.MessageReceipt, error)
}

// TicketValidator validates that an input ticket is valid.
type TicketValidator interface {
	IsValidTicket(ctx context.Context, base block.TipSetKey, entry *drand.Entry, newPeriod bool, epoch abi.ChainEpoch, miner address.Address, workerSigner address.Address, ticket block.Ticket) error
}

// ElectionValidator validates that an election fairly produced a winner.
type ElectionValidator interface {
	IsWinner(challengeTicket []byte, minerPower, networkPower abi.StoragePower) bool
	// VerifyElectionProof(ctx context.Context, entry *drand.Entry, epoch abi.ChainEpoch, miner address.Address, workerSigner address.Address, vrfProof crypto.VRFPi) error
	// VerifyWinningPoSt(ctx context.Context, ep EPoStVerifier, seedEntry *drand.Entry, epoch abi.ChainEpoch, proofs []block.PoStProof, mIDAddr address.Address, sectors SectorsStateView) (bool, error)
}

// StateViewer provides views into the chain state.
type StateViewer interface {
	PowerStateView(root cid.Cid) PowerStateView
	FaultStateView(root cid.Cid) FaultStateView
}

type chainReader interface {
	GetTipSet(tsKey block.TipSetKey) (block.TipSet, error)
	GetTipSetStateRoot(tsKey block.TipSetKey) (cid.Cid, error)
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

	// chainState is a reference to the current chain state
	chainState chainReader

	// processor is what we use to process messages and pay rewards
	processor Processor

	// state produces snapshots
	state StateViewer

	blockTime time.Duration

	// postVerifier verifies PoSt proofs and associated data
	proofVerifier ProofVerifier

	clock clock.ChainEpochClock
	drand drand.Schedule
}

// Ensure Expected satisfies the Protocol interface at compile time.
var _ Protocol = (*Expected)(nil)

// NewExpected is the constructor for the Expected consenus.Protocol module.
func NewExpected(cs cbor.IpldStore, bs blockstore.Blockstore, processor Processor, state StateViewer, bt time.Duration,
	ev ElectionValidator, tv TicketValidator, pv ProofVerifier, chainState chainReader, clock clock.ChainEpochClock, drand drand.Schedule) *Expected {
	return &Expected{
		cstore:            cs,
		blockTime:         bt,
		bstore:            bs,
		processor:         processor,
		state:             state,
		ElectionValidator: ev,
		TicketValidator:   tv,
		proofVerifier:     pv,
		chainState:        chainState,
		clock:             clock,
		drand:             drand,
	}
}

// BlockTime returns the block time used by the consensus protocol.
func (c *Expected) BlockTime() time.Duration {
	return c.blockTime
}

// RunStateTransition applies the messages in a tipset to a state, and persists that new state.
// It errors if the tipset was not mined according to the EC rules, or if any of the messages
// in the tipset results in an error.
func (c *Expected) RunStateTransition(ctx context.Context, ts block.TipSet, blsMessages [][]*types.UnsignedMessage, secpMessages [][]*types.SignedMessage,
	parentWeight big.Int, parentStateRoot cid.Cid, parentReceiptRoot cid.Cid) (root cid.Cid, receipts []vm.MessageReceipt, err error) {
	ctx, span := trace.StartSpan(ctx, "Expected.RunStateTransition")
	span.AddAttributes(trace.StringAttribute("tipset", ts.String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	if err := c.validateMining(ctx, ts, parentStateRoot, blsMessages, secpMessages, parentWeight, parentReceiptRoot); err != nil {
		return cid.Undef, []vm.MessageReceipt{}, err
	}

	priorState, err := c.loadStateTree(ctx, parentStateRoot)
	if err != nil {
		return cid.Undef, []vm.MessageReceipt{}, err
	}
	vms := vm.NewStorage(c.bstore)
	var newState state.Tree
	newState, receipts, err = c.runMessages(ctx, priorState, vms, ts, blsMessages, secpMessages)
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
func (c *Expected) validateMining(ctx context.Context,
	ts block.TipSet,
	parentStateRoot cid.Cid,
	blsMsgs [][]*types.UnsignedMessage,
	secpMsgs [][]*types.SignedMessage,
	parentWeight big.Int,
	parentReceiptRoot cid.Cid) error {

	keyStateView := c.state.PowerStateView(parentStateRoot)
	sigValidator := appstate.NewSignatureValidator(keyStateView)
	faultsStateView := c.state.FaultStateView(parentStateRoot)
	keyPowerTable := NewPowerTableView(keyStateView, faultsStateView)

	tsHeight, err := ts.Height()
	if err != nil {
		return errors.Wrap(err, "could not get new tipset's height")
	}

	//sectorSetAncestor, err := chain.FindTipsetAtEpoch(ctx, ts, tsHeight-WinningPoStSectorSetLookback, c.chainState)
	//if err != nil {
	//	return errors.Wrap(err, "failed to find sector set lookback ancestor")
	//}
	//sectorSetStateRoot, err := c.chainState.GetTipSetStateRoot(sectorSetAncestor.Key())
	//if err != nil {
	//	return errors.Wrap(err, "failed to get state root for sectorSet ancestor")
	//}
	// sectorSetStateView := c.state.PowerStateView(sectorSetStateRoot)

	electionPowerAncestor, err := chain.FindTipsetAtEpoch(ctx, ts, tsHeight-ElectionPowerTableLookback, c.chainState)
	if err != nil {
		return errors.Wrap(err, "failed to find election power lookback ancestor")
	}
	electionPowerStateRoot, err := c.chainState.GetTipSetStateRoot(electionPowerAncestor.Key())
	if err != nil {
		return errors.Wrap(err, "failed to get state root for election power ancestor")
	}
	electionPowerStateView := c.state.PowerStateView(electionPowerStateRoot)
	electionPowerTable := NewPowerTableView(electionPowerStateView, faultsStateView)

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
			return errors.Errorf("block %s has invalid parent weight %d expected %d", blk.Cid().String(), blk.ParentWeight, parentWeight)
		}
		workerAddr, err := keyPowerTable.WorkerAddr(ctx, blk.Miner)
		if err != nil {
			return errors.Wrap(err, "failed to read worker address of block miner")
		}
		workerSignerAddr, err := keyPowerTable.SignerAddress(ctx, workerAddr)
		if err != nil {
			return errors.Wrapf(err, "failed to convert address, %s, to a signing address", workerAddr.String())
		}
		// Validate block signature
		if blk.BlockSig == nil {
			return errors.Errorf("invalid nil block signature")
		}
		if err := crypto.ValidateSignature(blk.SignatureData(), workerSignerAddr, *blk.BlockSig); err != nil {
			return errors.Wrap(err, "block signature invalid")
		}

		// Verify that the BLS signature aggregate is correct
		if err := sigValidator.ValidateBLSMessageAggregate(ctx, blsMsgs[i], blk.BLSAggregateSig); err != nil {
			return errors.Wrapf(err, "bls message verification failed for block %s", blk.Cid())
		}

		// Verify that all secp message signatures are correct
		for i, msg := range secpMsgs[i] {
			if err := sigValidator.ValidateMessageSignature(ctx, msg); err != nil {
				return errors.Wrapf(err, "invalid signature for secp message %d in block %s", i, blk.Cid())
			}
		}

		electionEntry, err := c.electionEntry(ctx, blk)
		if err != nil {
			return errors.Wrapf(err, "failed to get election entry")
		}
		// err = c.VerifyElectionProof(ctx, electionEntry, blk.Height, blk.Miner, workerSignerAddr, blk.ElectionProof.VRFProof)
		// if err != nil {
		// 	return errors.Wrapf(err, "failed to verify election proof")
		// }
		// TODO this is not using nominal power, which must take into account undeclared faults
		// TODO the nominal power must be tested against the minimum (power.minerNominalPowerMeetsConsensusMinimum)
		// See https://github.com/filecoin-project/go-filecoin/issues/3958
		minerPower, err := electionPowerTable.MinerClaimedPower(ctx, blk.Miner)
		if err != nil {
			return errors.Wrap(err, "failed to read miner claim from power table")
		}
		networkPower, err := electionPowerTable.NetworkTotalPower(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to read power table")
		}
		electionVRFDigest := blk.ElectionProof.VRFProof.Digest()
		wins := c.IsWinner(electionVRFDigest[:], minerPower, networkPower)
		if !wins {
			return errors.Errorf("Block did not win election")
		}

		//valid, err := c.VerifyElectionProof(ctx, c.drand, electionEntry, blk.Height, blk.PoStProofs, blk.Miner, sectorSetStateView)
		//if err != nil {
		//	return errors.Wrapf(err, "failed verifying winning post")
		//}
		//if !valid {
		//	return errors.Errorf("Invalid winning post")
		//}

		// Ticket was correctly generated by miner
		sampleEpoch := blk.Height - miner.ElectionLookback
		newPeriod := len(blk.BeaconEntries) > 0
		if err := c.IsValidTicket(ctx, blk.Parents, electionEntry, newPeriod, sampleEpoch, blk.Miner, workerSignerAddr, blk.Ticket); err != nil {
			return errors.Wrapf(err, "invalid ticket: %s in block %s", blk.Ticket.String(), blk.Cid())
		}
	}
	return nil
}

func (c *Expected) electionEntry(ctx context.Context, blk *block.Block) (*drand.Entry, error) {
	if len(blk.BeaconEntries) > 0 {
		return blk.BeaconEntries[len(blk.BeaconEntries)-1], nil
	}

	parent, err := c.chainState.GetTipSet(blk.Parents)
	if err != nil {
		return nil, err
	}
	return chain.FindLatestDRAND(ctx, parent, c.chainState)
}

// runMessages applies the messages of all blocks within the input
// tipset to the input base state.  Messages are extracted from tipset
// blocks sorted by their ticket bytes and run as a single state transition
// for the entire tipset. The output state must be flushed after calling to
// guarantee that the state transitions propagate.
// Messages that fail to apply are dropped on the floor (and no receipt is emitted).
func (c *Expected) runMessages(ctx context.Context, st state.Tree, vms vm.Storage, ts block.TipSet,
	blsMessages [][]*types.UnsignedMessage, secpMessages [][]*types.SignedMessage) (state.Tree, []vm.MessageReceipt, error) {
	msgs := []vm.BlockMessagesInfo{}

	// build message information per block
	for i := 0; i < ts.Len(); i++ {
		blk := ts.At(i)

		messageCount := len(blsMessages[i]) + len(secpMessages[i])
		if messageCount > block.BlockMessageLimit {
			return nil, nil, errors.Errorf("Number of messages in block %s is %d which exceeds block message limit", blk.Cid(), messageCount)
		}

		msgInfo := vm.BlockMessagesInfo{
			BLSMessages:  blsMessages[i],
			SECPMessages: secpMessages[i],
			Miner:        blk.Miner,
		}

		msgs = append(msgs, msgInfo)
	}

	// process tipset
	parent, err := ts.Parents()
	if err != nil {
		return nil, nil, err
	}
	pts, err := c.chainState.GetTipSet(parent)
	if err != nil {
		return nil, nil, err
	}
	receipts, err := c.processor.ProcessTipSet(ctx, st, vms, pts, ts, msgs)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error validating tipset")
	}

	return st, receipts, nil
}

func (c *Expected) loadStateTree(ctx context.Context, id cid.Cid) (*state.State, error) {
	return state.LoadState(ctx, c.cstore, id)
}

// DefaultStateViewer a state viewer to the power state view interface.
type DefaultStateViewer struct {
	*appstate.Viewer
}

// AsDefaultStateViewer adapts a state viewer to a power state viewer.
func AsDefaultStateViewer(v *appstate.Viewer) DefaultStateViewer {
	return DefaultStateViewer{v}
}

// PowerStateView returns a power state view for a state root.
func (v *DefaultStateViewer) PowerStateView(root cid.Cid) PowerStateView {
	return v.Viewer.StateView(root)
}

// FaultStateView returns a fault state view for a state root.
func (v *DefaultStateViewer) FaultStateView(root cid.Cid) FaultStateView {
	return v.Viewer.StateView(root)
}

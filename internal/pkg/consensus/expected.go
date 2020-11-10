package consensus

import (
	"bytes"
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/internal/pkg/beacon"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/fork"
	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics/tracing"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
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

// ElectionPowerTableLookback is the past epoch offset for reading the
// election power values
const ElectionPowerTableLookback = 10

// DRANDEpochLookback is the past filecoin epoch offset at which DRAND entries
// in that epoch should be included in a block.
const DRANDEpochLookback = 2

// A Processor processes all the messages in a block or tip set.
type Processor interface {
	// ProcessTipSet processes all messages in a tip set.
	ProcessTipSet(context.Context, state.Tree, *vm.Storage, *block.TipSet, *block.TipSet, []vm.BlockMessagesInfo, vm.VmOption) ([]types.MessageReceipt, error)
	// Todo add by force
	ProcessUnsignedMessage(context.Context, *types.UnsignedMessage, state.Tree, *vm.Storage, vm.VmOption) (*vm.Ret, error)
}

// TicketValidator validates that an input ticket is valid.
type TicketValidator interface {
	IsValidTicket(ctx context.Context, base block.TipSetKey, entry *block.BeaconEntry, newPeriod bool, epoch abi.ChainEpoch, miner address.Address, workerSigner address.Address, ticket block.Ticket) error
}

// StateViewer provides views into the chain state.
type StateViewer interface {
	PowerStateView(root cid.Cid) appstate.PowerStateView
	FaultStateView(root cid.Cid) appstate.FaultStateView
}

type chainReader interface {
	GetTipSet(tsKey block.TipSetKey) (*block.TipSet, error)
	GetHead() block.TipSetKey
	GetTipSetStateRoot(tsKey block.TipSetKey) (cid.Cid, error)
	GetGenesisBlock(ctx context.Context) (*block.Block, error)
}

type Randness interface {
	SampleChainRandomness(ctx context.Context, head block.TipSetKey, tag acrypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
	ChainGetRandomnessFromBeacon(ctx context.Context, tsk block.TipSetKey, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
}

// Expected implements expected consensus.
type Expected struct {
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

	messageStore *chain.MessageStore

	rnd Randness

	clock                       clock.ChainEpochClock
	drand                       beacon.Schedule
	fork                        fork.IFork
	circulatingSupplyCalculator *CirculatingSupplyCalculator
}

// Ensure Expected satisfies the Protocol interface at compile time.
var _ Protocol = (*Expected)(nil)

// NewExpected is the constructor for the Expected consenus.Protocol module.
func NewExpected(cs cbor.IpldStore,
	bs blockstore.Blockstore,
	processor Processor,
	state StateViewer,
	bt time.Duration,
	tv TicketValidator,
	pv ProofVerifier,
	chainState chainReader,
	clock clock.ChainEpochClock,
	drand beacon.Schedule,
	rnd Randness,
	messageStore *chain.MessageStore,
	fork fork.IFork,
) *Expected {
	c := &Expected{
		cstore:                      cs,
		blockTime:                   bt,
		bstore:                      bs,
		processor:                   processor,
		state:                       state,
		TicketValidator:             tv,
		proofVerifier:               pv,
		chainState:                  chainState,
		clock:                       clock,
		drand:                       drand,
		messageStore:                messageStore,
		rnd:                         rnd,
		fork:                        fork,
		circulatingSupplyCalculator: NewCirculatingSupplyCalculator(bs, chainState),
	}
	return c
}

// BlockTime returns the block time used by the consensus protocol.
func (c *Expected) BlockTime() time.Duration {
	return c.blockTime
}

// todo add by force
func (c *Expected) CallWithGas(ctx context.Context, msg *types.UnsignedMessage) (*vm.Ret, error) {
	head := c.chainState.GetHead()
	stateRoot, err := c.chainState.GetTipSetStateRoot(head)
	if err != nil {
		return nil, err
	}

	ts, err := c.chainState.GetTipSet(head)
	if err != nil {
		return nil, err
	}

	vms := vm.NewStorage(c.bstore)
	priorState, err := state.LoadState(ctx, vms, stateRoot)
	if err != nil {
		return nil, err
	}

	rnd := headRandomness{
		chain: c.rnd,
		head:  ts.Key(),
	}

	vmOption := vm.VmOption{
		CircSupplyCalculator: func(ctx context.Context, epoch abi.ChainEpoch, tree state.Tree) (abi.TokenAmount, error) {
			dertail, err := c.circulatingSupplyCalculator.GetCirculatingSupplyDetailed(ctx, epoch, tree)
			if err != nil {
				return abi.TokenAmount{}, err
			}
			return dertail.FilCirculating, nil
		},
		NtwkVersionGetter: c.fork.GetNtwkVersion,
		Rnd:               &rnd,
		BaseFee:           ts.At(0).ParentBaseFee,
		Epoch:             ts.At(0).Height,
	}
	return c.processor.ProcessUnsignedMessage(ctx, msg, priorState, vms, vmOption)
}

// RunStateTransition applies the messages in a tipset to a state, and persists that new state.
// It errors if the tipset was not mined according to the EC rules, or if any of the messages
// in the tipset results in an error.
func (c *Expected) RunStateTransition(ctx context.Context,
	ts *block.TipSet,
	secpMessages [][]*types.SignedMessage,
	blsMessages [][]*types.UnsignedMessage,
	parentStateRoot cid.Cid) (root cid.Cid, receipts []types.MessageReceipt, err error) {
	ctx, span := trace.StartSpan(ctx, "Expected.RunStateTransition")
	span.AddAttributes(trace.StringAttribute("tipset", ts.String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	vms := vm.NewStorage(c.bstore)
	priorState, err := state.LoadState(ctx, vms, parentStateRoot)
	if err != nil {
		return cid.Undef, []types.MessageReceipt{}, err
	}

	var newState state.Tree
	newState, receipts, err = c.runMessages(ctx, priorState, vms, ts, blsMessages, secpMessages)
	if err != nil {
		return cid.Undef, []types.MessageReceipt{}, err
	}
	err = vms.Flush()
	if err != nil {
		return cid.Undef, []types.MessageReceipt{}, err
	}

	root, err = newState.Flush(ctx)
	if err != nil {
		return cid.Undef, []types.MessageReceipt{}, err
	}
	return root, receipts, err
}

// validateMining checks validity of the ticket, proof, signature and miner
// address of every block in the tipset.
func (c *Expected) ValidateMining(ctx context.Context,
	ts *block.TipSet,
	parentStateRoot cid.Cid,
	parentWeight big.Int,
	parentReceiptRoot cid.Cid) error {

	var secpMsgs [][]*types.SignedMessage
	var blsMsgs [][]*types.UnsignedMessage
	for i := 0; i < ts.Len(); i++ {
		blk := ts.At(i)
		blksecpMsgs, blkblsMsgs, err := c.messageStore.LoadMetaMessages(ctx, blk.Messages.Cid)
		if err != nil {
			return errors.Wrapf(err, "syncing tip %s failed loading message list %s for block %s", ts.Key(), blk.Messages, blk.Cid())
		}

		blsMsgs = append(blsMsgs, blkblsMsgs)
		secpMsgs = append(secpMsgs, blksecpMsgs)
	}

	keyStateView := c.state.PowerStateView(parentStateRoot)
	sigValidator := appstate.NewSignatureValidator(keyStateView)
	faultsStateView := c.state.FaultStateView(parentStateRoot)
	keyPowerTable := appstate.NewPowerTableView(keyStateView, faultsStateView)

	var wg errgroup.Group
	for i := 0; i < ts.Len(); i++ {
		blk := ts.At(i)
		wg.Go(func() error {
			// Fetch the URL.
			return c.validateBlock(ctx, keyPowerTable, sigValidator, blk, parentStateRoot, parentWeight, parentReceiptRoot)
		})
	}
	return wg.Wait()
}

// Todo beacon check
func (c *Expected) validateBlock(ctx context.Context,
	keyPowerTable appstate.PowerTableView,
	sigValidator *appstate.SignatureValidator,
	blk *block.Block,
	parentStateRoot cid.Cid,
	parentWeight big.Int,
	parentReceiptRoot cid.Cid) error {
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

	blksecpMsgs, blkblsMsgs, err := c.messageStore.LoadMetaMessages(ctx, blk.Messages.Cid)
	if err != nil {
		return errors.Wrapf(err, "failed loading message list %s for block %s", blk.Messages, blk.Cid())
	}

	// Verify that the BLS signature aggregate is correct todo remove to sync fast
	if err := sigValidator.ValidateBLSMessageAggregate(ctx, blkblsMsgs, blk.BLSAggregateSig); err != nil {
		return errors.Wrapf(err, "bls message verification failed for block %s", blk.Cid())
	}

	// Verify that all secp message signatures are correct

	for i, msg := range blksecpMsgs {
		if err := sigValidator.ValidateMessageSignature(ctx, msg); err != nil {
			return errors.Wrapf(err, "invalid signature for secp message %d in block %s", i, blk.Cid())
		}
	}

	//round := tsHeight + base.NullRounds + 1
	//winner, err := IsRoundWinner(ctx, ts, round, blk.Miner, rbase, mbi, m.api)
	//if err != nil {
	//	return xerrors.Errorf("failed to check if we win next round: %w", err)
	//}
	//if winner == nil {
	//	return errors.Errorf("Block did not win election")
	//}

	//valid, err := c.VerifyElectionProof(ctx, c.drand, electionEntry, blk.Height, blk.PoStProofs, blk.Miner, sectorSetStateView)
	//if err != nil {
	//	return errors.Wrapf(err, "failed verifying winning post")
	//}
	//if !valid {
	//	return errors.Errorf("Invalid winning post")
	//}

	beaconBase, err := c.beaconBaseEntry(ctx, blk)
	if err != nil {
		return errors.Wrapf(err, "failed to get election entry")
	}

	// Ticket was correctly generated by miner
	sampleEpoch := blk.Height - constants.TicketRandomnessLookback
	bSmokeHeight := blk.Height > fork.UpgradeSmokeHeight
	if err := c.IsValidTicket(ctx, blk.Parents, beaconBase, bSmokeHeight, sampleEpoch, blk.Miner, workerSignerAddr, blk.Ticket); err != nil {
		fmt.Printf("invalid ticket: %s in block %s, err: %s", blk.Ticket.String(), blk.Cid(), err)
		//return errors.Wrapf(err, "invalid ticket: %s in block %s", blk.Ticket.String(), blk.Cid())
	}
	return nil
}

// Todo beacon check
func (c *Expected) ValidateBlockBeacon(b *block.Block, parentEpoch abi.ChainEpoch, prevEntry *block.BeaconEntry) error {
	return beacon.ValidateBlockValues(c.drand, b, parentEpoch, prevEntry)
}

func (c *Expected) ValidateBlockWinner(ctx context.Context, blk *block.Block, stateID cid.Cid, prevEntry *block.BeaconEntry) error {
	if blk.ElectionProof.WinCount < 1 {
		return xerrors.Errorf("block is not claiming to be a winner")
	}

	view := c.state.PowerStateView(stateID)
	if view == nil {
		return xerrors.New("power state view is null")
	}

	_, qaPower, err := view.MinerClaimedPower(ctx, blk.Miner)
	if err != nil {
		return xerrors.Errorf("get miner power failed: %w", err)
	}

	rBeacon := prevEntry
	if len(blk.BeaconEntries) != 0 {
		rBeacon = blk.BeaconEntries[len(blk.BeaconEntries)-1]
	}
	buf := new(bytes.Buffer)
	if err := blk.Miner.MarshalCBOR(buf); err != nil {
		return xerrors.Errorf("failed to marshal miner address to cbor: %w", err)
	}

	vrfBase, err := chain.DrawRandomness(rBeacon.Data, acrypto.DomainSeparationTag_ElectionProofProduction, blk.Height, buf.Bytes())
	if err != nil {
		return xerrors.Errorf("could not draw randomness: %w", err)
	}

	_, waddr, err := view.MinerControlAddresses(ctx, blk.Miner)
	if err != nil {
		return xerrors.Errorf("query worker address failed: %w", err)
	}

	if err := VerifyElectionPoStVRF(ctx, waddr, vrfBase, blk.ElectionProof.VRFProof); err != nil {
		return xerrors.Errorf("validating block election proof failed: %w", err)
	}

	totalPower, err := view.PowerNetworkTotal(ctx)
	if err != nil {
		return xerrors.Errorf("get miner power failed: %w", err)
	}

	j := blk.ElectionProof.ComputeWinCount(qaPower, totalPower.QualityAdjustedPower)
	if blk.ElectionProof.WinCount != j {
		return xerrors.Errorf("miner claims wrong number of wins: miner: %d, computed: %d", blk.ElectionProof.WinCount, j)
	}

	return nil
}

func (c *Expected) beaconBaseEntry(ctx context.Context, blk *block.Block) (*block.BeaconEntry, error) {
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
func (c *Expected) runMessages(ctx context.Context, st state.Tree, vms *vm.Storage, ts *block.TipSet,
	blsMessages [][]*types.UnsignedMessage, secpMessages [][]*types.SignedMessage) (state.Tree, []types.MessageReceipt, error) {
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
			WinCount:     blk.ElectionProof.WinCount,
		}

		msgs = append(msgs, msgInfo)
	}

	// process tipset
	var pts *block.TipSet
	if ts.EnsureHeight() > 0 {
		parent, err := ts.Parents()
		if err != nil {
			return nil, nil, err
		}
		pts, err = c.chainState.GetTipSet(parent)
		if err != nil {
			return nil, nil, err
		}
	} else {
		return st, []types.MessageReceipt{}, nil
	}

	rnd := headRandomness{
		chain: c.rnd,
		head:  ts.Key(),
	}

	vmOption := vm.VmOption{
		CircSupplyCalculator: func(ctx context.Context, epoch abi.ChainEpoch, tree state.Tree) (abi.TokenAmount, error) {
			dertail, err := c.circulatingSupplyCalculator.GetCirculatingSupplyDetailed(ctx, epoch, tree)
			if err != nil {
				return abi.TokenAmount{}, err
			}
			return dertail.FilCirculating, nil
		},
		NtwkVersionGetter: c.fork.GetNtwkVersion,
		Rnd:               &rnd,
		BaseFee:           ts.At(0).ParentBaseFee,
		Fork:              c.fork,
		Epoch:             ts.At(0).Height,
	}
	receipts, err := c.processor.ProcessTipSet(ctx, st, vms, pts, ts, msgs, vmOption)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error validating tipset")
	}

	return st, receipts, nil
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
func (v *DefaultStateViewer) PowerStateView(root cid.Cid) appstate.PowerStateView {
	return v.Viewer.StateView(root)
}

// FaultStateView returns a fault state view for a state root.
func (v *DefaultStateViewer) FaultStateView(root cid.Cid) appstate.FaultStateView {
	return v.Viewer.StateView(root)
}

// A chain randomness source with a fixed head tipset key.
type headRandomness struct {
	chain ChainRandomness
	head  block.TipSetKey
}

func (h *headRandomness) Randomness(ctx context.Context, tag acrypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return h.chain.SampleChainRandomness(ctx, h.head, tag, epoch, entropy)
}

func (h *headRandomness) GetRandomnessFromBeacon(ctx context.Context, tag acrypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return h.chain.ChainGetRandomnessFromBeacon(ctx, h.head, tag, epoch, entropy)
}

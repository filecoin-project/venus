package consensus

import (
	"context"
	"math/big"
	"time"

	address "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	fbig "github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	cid "github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics/tracing"
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
	// ErrUnorderedTipSets is returned when weight and minticket are the same between two tipsets.
	ErrUnorderedTipSets = errors.New("trying to order two identical tipsets")
	// ErrReceiptRootMismatch is returned when the block's receipt root doesn't match the receipt root computed for the parent tipset.
	ErrReceiptRootMismatch = errors.New("blocks receipt root does not match parent tip set")
)

// challengeBits is the number of bits in the challenge ticket's domain
const challengeBits = 256

// expectedLeadersPerEpoch is the mean number of leaders per epoch
const expectedLeadersPerEpoch = 5

// A Processor processes all the messages in a block or tip set.
type Processor interface {
	// ProcessTipSet processes all messages in a tip set.
	ProcessTipSet(context.Context, state.Tree, vm.Storage, block.TipSet, []vm.BlockMessagesInfo) ([]vm.MessageReceipt, error)
}

// TicketValidator validates that an input ticket is valid.
type TicketValidator interface {
	IsValidTicket(ctx context.Context, base block.TipSetKey, epoch abi.ChainEpoch, miner address.Address, worker address.Address, ticket block.Ticket) error
}

// ElectionValidator validates that an election fairly produced a winner.
type ElectionValidator interface {
	VerifyPoSt(ctx context.Context, ep EPoStVerifier, allSectorInfos []abi.SectorInfo, challengeSeed abi.PoStRandomness, proofs []block.EPoStProof, candidates []block.EPoStCandidate, mIDAddr address.Address) (bool, error)
	CandidateWins(challengeTicket []byte, sectorNum, faultNum, networkPower, sectorSize uint64) bool
	VerifyEPoStVrfProof(ctx context.Context, base block.TipSetKey, epoch abi.ChainEpoch, miner address.Address, worker address.Address, vrfProof abi.PoStRandomness) error
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
	postVerifier EPoStVerifier
}

// Ensure Expected satisfies the Protocol interface at compile time.
var _ Protocol = (*Expected)(nil)

// NewExpected is the constructor for the Expected consenus.Protocol module.
func NewExpected(cs cbor.IpldStore, bs blockstore.Blockstore, processor Processor, state StateViewer, bt time.Duration,
	ev ElectionValidator, tv TicketValidator, pv EPoStVerifier) *Expected {
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
func (c *Expected) RunStateTransition(ctx context.Context, ts block.TipSet, blsMessages [][]*types.UnsignedMessage, secpMessages [][]*types.SignedMessage,
	parentWeight fbig.Int, parentStateRoot cid.Cid, parentReceiptRoot cid.Cid) (root cid.Cid, receipts []vm.MessageReceipt, err error) {
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

	root, err = newState.Commit(ctx)
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
	parentWeight fbig.Int,
	parentReceiptRoot cid.Cid) error {

	stateView := c.state.StateView(parentStateRoot)
	sigValidator := NewSignatureValidator(stateView)
	powerTable := NewPowerTableView(stateView)

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
		workerSignerAddr, err := powerTable.SignerAddress(ctx, workerAddr)
		if err != nil {
			return errors.Wrapf(err, "failed to convert address, %s, to a signing address", workerAddr.String())
		}
		// Validate block signature
		if err := crypto.ValidateSignature(blk.SignatureData(), workerSignerAddr, blk.BlockSig); err != nil {
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

		// Epoch at which election post and ticket randomness must be sampled
		sampleEpoch := blk.Height - miner.ElectionLookback

		// Verify EPoSt VRF proof ("PoSt randomness")
		if err := c.VerifyEPoStVrfProof(ctx, blk.Parents, sampleEpoch, blk.Miner, workerSignerAddr, blk.EPoStInfo.VRFProof); err != nil {
			return errors.Wrapf(err, "failed to verify EPoSt VRF proof (PoSt randomness) in block %s", blk.Cid())
		}

		// Verify no duplicate challenge indexes
		challengeIndexes := make(map[int64]struct{})
		for _, winner := range blk.EPoStInfo.Winners {
			index := winner.SectorChallengeIndex
			if _, dup := challengeIndexes[index]; dup {
				return errors.Errorf("Duplicate partial ticket submitted, challenge idx: %d", index)
			}
			challengeIndexes[index] = struct{}{}
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
		vrfDigest := crypto.VRFPi(blk.EPoStInfo.VRFProof).Digest()
		valid, err := c.VerifyPoSt(ctx, c.postVerifier, allSectorInfos, vrfDigest[:],
			blk.EPoStInfo.PoStProofs, blk.EPoStInfo.Winners, blk.Miner)
		if err != nil {
			return errors.Wrapf(err, "error checking PoSt")
		}
		if !valid {
			return errors.Errorf("invalid PoSt")
		}

		// Ticket was correctly generated by miner
		if err := c.IsValidTicket(ctx, blk.Parents, sampleEpoch, blk.Miner, workerSignerAddr, blk.Ticket); err != nil {
			return errors.Wrapf(err, "invalid ticket: %s in block %s", blk.Ticket.String(), blk.Cid())
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
func (c *Expected) runMessages(ctx context.Context, st state.Tree, vms vm.Storage, ts block.TipSet,
	blsMessages [][]*types.UnsignedMessage, secpMessages [][]*types.SignedMessage) (state.Tree, []vm.MessageReceipt, error) {
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

func (c *Expected) loadStateTree(ctx context.Context, id cid.Cid) (*state.State, error) {
	return state.LoadState(ctx, c.cstore, id)
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

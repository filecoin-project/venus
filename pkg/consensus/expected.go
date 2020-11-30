package consensus

import "C"
import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Gurpartap/async"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/network"
	proof2 "github.com/filecoin-project/specs-actors/v2/actors/runtime/proof"
	"github.com/hashicorp/go-multierror"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/pkg/beacon"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/enccid"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/metrics/tracing"
	"github.com/filecoin-project/venus/pkg/specactors/adt"
	"github.com/filecoin-project/venus/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/account"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/power"
	"github.com/filecoin-project/venus/pkg/specactors/policy"
	appstate "github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	"github.com/filecoin-project/venus/pkg/vm/state"

	_ "github.com/filecoin-project/venus/pkg/consensus/lib/sigs/bls"  // enable bls signatures
	_ "github.com/filecoin-project/venus/pkg/consensus/lib/sigs/secp" // enable secp signatures
)

var (
	// ErrStateRootMismatch is returned when the computed state root doesn't match the expected result.
	ErrStateRootMismatch = errors.New("blocks state root does not match computed result")
	// ErrUnorderedTipSets is returned when weight and minticket are the same between two tipsets.
	ErrUnorderedTipSets = errors.New("trying to order two identical tipsets")
	// ErrReceiptRootMismatch is returned when the block's receipt root doesn't match the receipt root computed for the parent tipset.
	ErrReceiptRootMismatch = errors.New("blocks receipt root does not match parent tip set")
)

var logExpect = logging.Logger("consensus")

const AllowableClockDriftSecs = uint64(1)

// A Processor processes all the messages in a block or tip set.
type Processor interface {
	// ProcessTipSet processes all messages in a tip set.
	ProcessTipSet(context.Context, state.Tree, *vm.Storage, *block.TipSet, *block.TipSet, []vm.BlockMessagesInfo, vm.VmOption) ([]types.MessageReceipt, error)
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
	GetLatestBeaconEntry(ts *block.TipSet) (*block.BeaconEntry, error)
	GetTipSetByHeight(context.Context, *block.TipSet, abi.ChainEpoch, bool) (*block.TipSet, error)
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
	parent, ts *block.TipSet,
	parentWeight big.Int,
	parentReceiptRoot cid.Cid) error {

	parentStateRoot, err := c.chainState.GetTipSetStateRoot(parent.Key())
	if err != nil {
		return xerrors.Errorf("get parent tipset state failed %s", err)
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
			return c.validateBlock(ctx, keyPowerTable, sigValidator, parent, blk, parentWeight, parentReceiptRoot)
		})
	}
	return wg.Wait()
}

func (c *Expected) validateBlock(ctx context.Context,
	keyPowerTable appstate.PowerTableView,
	sigValidator *appstate.SignatureValidator,
	parent *block.TipSet,
	blk *block.Block,
	parentWeight big.Int,
	parentReceiptRoot cid.Cid) (err error) {

	validationStart := time.Now()
	defer func() {
		logExpect.Infow("block validation", "took", time.Since(validationStart), "height", blk.Height, "age", time.Since(time.Unix(int64(blk.Timestamp), 0)))
	}()

	// fast checks first
	if err := blockSanityChecks(blk); err != nil {
		return xerrors.Errorf("incoming header failed basic sanity checks: %v", err)
	}

	baseHeight, _ := parent.Height()
	nulls := blk.Height - (baseHeight + 1)
	if tgtTs := parent.MinTimestamp() + constants.BlockDelaySecs*uint64(nulls+1); blk.Timestamp != tgtTs {
		return xerrors.Errorf("block has wrong timestamp: %d != %d", blk.Timestamp, tgtTs)
	}

	now := uint64(time.Now().Unix())
	if blk.Timestamp > now+AllowableClockDriftSecs {
		return xerrors.Errorf("block was from the future (now=%d, blk=%d): %v", now, blk.Timestamp, ErrTemporal)
	}
	if blk.Timestamp > now {
		log.Warn("Got block from the future, but within threshold", blk.Timestamp, time.Now().Unix())
	}

	// get parent beacon
	prevBeacon, err := c.chainState.GetLatestBeaconEntry(parent)
	if err != nil {
		return xerrors.Errorf("failed to get latest beacon entry: %s", err)
	}

	// confirm block state root matches parent state root
	parentStateRoot, err := c.chainState.GetTipSetStateRoot(parent.Key())
	if err != nil {
		return xerrors.Errorf("get parent tipset state failed %s", err)
	}
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

	// get worker address
	workerAddr, err := keyPowerTable.WorkerAddr(ctx, blk.Miner)
	if err != nil {
		return errors.Wrap(err, "failed to read worker address of block miner")
	}
	workerSignerAddr, err := keyPowerTable.SignerAddress(ctx, workerAddr)
	if err != nil {
		return errors.Wrapf(err, "failed to convert address, %s, to a signing address", workerAddr.String())
	}

	msgsCheck := async.Err(func() error {
		if err := c.checkBlockMessages(ctx, sigValidator, blk, parent, parentStateRoot); err != nil {
			return xerrors.Errorf("block had invalid messages: %v", err)
		}
		return nil
	})

	minerCheck := async.Err(func() error {
		if err := c.minerIsValid(ctx, blk.Miner, parentStateRoot); err != nil {
			return xerrors.Errorf("minerIsValid failed: %v", err)
		}
		return nil
	})

	baseFeeCheck := async.Err(func() error {
		baseFee, err := c.messageStore.ComputeBaseFee(ctx, parent)
		if err != nil {
			return xerrors.Errorf("computing base fee: %v", err)
		}
		if big.Cmp(baseFee, blk.ParentBaseFee) != 0 {
			return xerrors.Errorf("base fee doesn't match: %s (header) != %s (computed)",
				blk.ParentBaseFee, baseFee)
		}
		return nil
	})

	blockSigCheck := async.Err(func() error {
		// Validate block signature
		if err := crypto.ValidateSignature(blk.SignatureData(), workerSignerAddr, *blk.BlockSig); err != nil {
			return errors.Wrap(err, "block signature invalid")
		}

		return nil
	})

	beaconValuesCheck := async.Err(func() error {
		parentHeight, _ := parent.Height()
		if err = c.ValidateBlockBeacon(blk, parentHeight, prevBeacon); err != nil {
			return err
		}
		return nil
	})

	tktsCheck := async.Err(func() error {
		beaconBase, err := c.beaconBaseEntry(ctx, blk)
		if err != nil {
			return errors.Wrapf(err, "failed to get election entry")
		}

		sampleEpoch := blk.Height - constants.TicketRandomnessLookback
		bSmokeHeight := blk.Height > fork.UpgradeSmokeHeight
		if err := c.IsValidTicket(ctx, blk.Parents, beaconBase, bSmokeHeight, sampleEpoch, blk.Miner, workerSignerAddr, blk.Ticket); err != nil {
			return errors.Wrapf(err, "invalid ticket: %s in block %s", blk.Ticket.String(), blk.Cid())
		}
		return nil
	})

	lbTs, lbStateRoot, err := c.GetLookbackTipSetForRound(ctx, parent, blk.Height)
	if err != nil {
		return xerrors.Errorf("failed to get lookback tipset for block: %v", err)
	}

	winnerCheck := async.Err(func() error {
		if err = c.ValidateBlockWinner(ctx, lbTs, lbStateRoot, parent, parentStateRoot, blk, prevBeacon); err != nil {
			return err
		}
		return nil
	})

	winPoStNv := c.fork.GetNtwkVersion(ctx, baseHeight)
	wproofCheck := async.Err(func() error {
		if err := c.VerifyWinningPoStProof(ctx, winPoStNv, blk, prevBeacon, lbStateRoot); err != nil {
			return xerrors.Errorf("invalid election post: %v", err)
		}
		return nil
	})

	await := []async.ErrorFuture{
		minerCheck,
		tktsCheck,
		blockSigCheck,
		beaconValuesCheck,
		wproofCheck,
		winnerCheck,
		msgsCheck,
		baseFeeCheck,
	}

	var merr error
	for _, fut := range await {
		if err := fut.AwaitContext(ctx); err != nil {
			merr = multierror.Append(merr, err)
		}
	}
	if merr != nil {
		mulErr := merr.(*multierror.Error)
		mulErr.ErrorFormat = func(es []error) string {
			if len(es) == 1 {
				return fmt.Sprintf("1 error occurred:\n\t* %+v\n\n", es[0])
			}

			points := make([]string, len(es))
			for i, err := range es {
				points[i] = fmt.Sprintf("* %+v", err)
			}

			return fmt.Sprintf(
				"%d errors occurred:\n\t%s\n\n",
				len(es), strings.Join(points, "\n\t"))
		}
		return mulErr
	}

	return nil
}

var ErrTemporal = errors.New("temporal error")

func blockSanityChecks(b *block.Block) error {
	if b.ElectionProof == nil {
		return xerrors.Errorf("block cannot have nil election proof")
	}

	if b.BlockSig == nil {
		return xerrors.Errorf("block had nil signature")
	}

	if b.BLSAggregateSig == nil {
		return xerrors.Errorf("block had nil bls aggregate signature")
	}

	return nil
}

// TODO: We should extract this somewhere else and make the message pool and miner use the same logic
func (c *Expected) checkBlockMessages(ctx context.Context, sigValidator *appstate.SignatureValidator, blk *block.Block, baseTs *block.TipSet, stateRoot cid.Cid) error {
	blksecpMsgs, blkblsMsgs, err := c.messageStore.LoadMetaMessages(ctx, blk.Messages.Cid)
	if err != nil {
		return errors.Wrapf(err, "failed loading message list %s for block %s", blk.Messages, blk.Cid())
	}
	{
		// Verify that the BLS signature aggregate is correct
		if err := sigValidator.ValidateBLSMessageAggregate(ctx, blkblsMsgs, blk.BLSAggregateSig); err != nil {
			return errors.Wrapf(err, "bls message verification failed for block %s", blk.Cid())
		}

		// Verify that all secp message signatures are correct
		for i, msg := range blksecpMsgs {
			if err := sigValidator.ValidateMessageSignature(ctx, msg); err != nil {
				return errors.Wrapf(err, "invalid signature for secp message %d in block %s", i, blk.Cid())
			}
		}
	}

	nonces := make(map[address.Address]uint64)
	vms := vm.NewStorage(c.bstore)
	st, err := state.LoadState(ctx, vms, stateRoot)
	if err != nil {
		return xerrors.Errorf("loading state: %v", err)
	}

	baseHeight, _ := baseTs.Height()
	pl := gas.PricelistByEpoch(baseHeight)
	var sumGasLimit int64
	checkMsg := func(msg types.ChainMsg) error {
		m := msg.VMMessage()

		// Phase 1: syntactic validation, as defined in the spec
		minGas := pl.OnChainMessage(msg.ChainLength())
		if err := m.ValidForBlockInclusion(minGas.Total(), c.fork.GetNtwkVersion(ctx, blk.Height)); err != nil {
			return err
		}

		// ValidForBlockInclusion checks if any single message does not exceed BlockGasLimit
		// So below is overflow safe
		sumGasLimit += int64(m.GasLimit)
		if sumGasLimit > constants.BlockGasLimit {
			return xerrors.Errorf("block gas limit exceeded")
		}

		// Phase 2: (Partial) semantic validation:
		// the sender exists and is an account actor, and the nonces make sense
		if _, ok := nonces[m.From]; !ok {
			// `GetActor` does not validate that this is an account actor.
			act, find, err := st.GetActor(ctx, m.From)
			if err != nil {
				return xerrors.Errorf("failed to get actor: %v", err)
			}

			if !find {
				return xerrors.Errorf("actor %s not found", m.From)
			}

			if !builtin.IsAccountActor(act.Code.Cid) {
				return xerrors.New("Sender must be an account actor")
			}
			nonces[m.From] = act.CallSeqNum
		}

		if nonces[m.From] != m.CallSeqNum {
			return xerrors.Errorf("wrong nonce (exp: %d, got: %d)", nonces[m.From], m.CallSeqNum)
		}
		nonces[m.From]++

		return nil
	}

	// Validate message arrays in a temporary blockstore.
	blsMsgs := make([]types.ChainMsg, len(blkblsMsgs))
	for i, m := range blkblsMsgs {
		if err := checkMsg(m); err != nil {
			return xerrors.Errorf("block had invalid bls message at index %d: %v", i, err)
		}

		blsMsgs[i] = m
	}

	secpMsgs := make([]types.ChainMsg, len(blksecpMsgs))
	for i, m := range blksecpMsgs {
		if err := checkMsg(m); err != nil {
			return xerrors.Errorf("block had invalid secpk message at index %d: %v", i, err)
		}

		secpMsgs[i] = m
	}

	bmroot, err := chain.GetChainMsgRoot(ctx, blsMsgs)
	if err != nil {
		return xerrors.Errorf("get blsMsgs root failed: %v", err)
	}

	smroot, err := chain.GetChainMsgRoot(ctx, secpMsgs)
	if err != nil {
		return xerrors.Errorf("get secpMsgs root failed: %v", err)
	}

	b, err := chain.MakeBlock(&types.TxMeta{
		BLSRoot:  enccid.NewCid(bmroot),
		SecpRoot: enccid.NewCid(smroot),
	})
	if err != nil {
		return xerrors.Errorf("serialize tx meta failed: %v", err)
	}
	if blk.Messages.Cid != b.Cid() {
		return fmt.Errorf("messages didnt match message root in header")
	}

	return nil
}

func (c *Expected) VerifyWinningPoStProof(ctx context.Context, nv network.Version, blk *block.Block, prevBeacon *block.BeaconEntry, lbst cid.Cid) error {
	if constants.InsecurePoStValidation {
		if len(blk.WinPoStProof) == 0 {
			return xerrors.Errorf("[INSECURE-POST-VALIDATION] No winning post proof given")
		}

		if string(blk.WinPoStProof[0].ProofBytes) == "valid proof" {
			return nil
		}
		return xerrors.Errorf("[INSECURE-POST-VALIDATION] winning post was invalid")
	}

	buf := new(bytes.Buffer)
	if err := blk.Miner.MarshalCBOR(buf); err != nil {
		return xerrors.Errorf("failed to marshal miner address: %v", err)
	}

	rbase := prevBeacon
	if len(blk.BeaconEntries) > 0 {
		rbase = blk.BeaconEntries[len(blk.BeaconEntries)-1]
	}

	rand, err := chain.DrawRandomness(rbase.Data, acrypto.DomainSeparationTag_WinningPoStChallengeSeed, blk.Height, buf.Bytes())
	if err != nil {
		return xerrors.Errorf("failed to get randomness for verifying winning post proof: %v", err)
	}

	mid, err := address.IDFromAddress(blk.Miner)
	if err != nil {
		return xerrors.Errorf("failed to get ID from miner address %s: %v", blk.Miner, err)
	}

	view := c.state.PowerStateView(lbst)
	if view == nil {
		return xerrors.New("power state view is null")
	}

	sectors, err := view.GetSectorsForWinningPoSt(ctx, nv, c.proofVerifier, lbst, blk.Miner, rand)
	if err != nil {
		return xerrors.Errorf("getting winning post sector set: %v", err)
	}

	proofs := make([]proof2.PoStProof, len(blk.WinPoStProof))
	for idx, pf := range blk.WinPoStProof {
		proofs[idx] = proof2.PoStProof{PoStProof: pf.PoStProof, ProofBytes: pf.ProofBytes}
	}
	ok, err := c.proofVerifier.VerifyWinningPoSt(ctx, proof2.WinningPoStVerifyInfo{
		Randomness:        rand,
		Proofs:            proofs,
		ChallengedSectors: sectors,
		Prover:            abi.ActorID(mid),
	})

	if err != nil {
		return xerrors.Errorf("failed to verify election post: %v", err)
	}

	if !ok {
		log.Errorf("invalid winning post (block: %s, %x; %v)", blk.Cid(), rand, sectors)
		return xerrors.Errorf("winning post was invalid")
	}

	return nil
}

func (c *Expected) ValidateBlockBeacon(blk *block.Block, parentEpoch abi.ChainEpoch, prevEntry *block.BeaconEntry) error {
	if os.Getenv("VENUS_IGNORE_DRAND") == "_yes_" {
		return nil
	}
	return beacon.ValidateBlockValues(c.drand, blk, parentEpoch, prevEntry)
}

func (c *Expected) minerIsValid(ctx context.Context, maddr address.Address, baseStateRoot cid.Cid) error {
	vms := vm.NewStorage(c.bstore)
	sm, err := state.LoadState(ctx, vms, baseStateRoot)
	if err != nil {
		return xerrors.Errorf("loading state: %s", err)
	}

	pact, find, err := sm.GetActor(ctx, power.Address)
	if err != nil {
		return xerrors.Errorf("get power actor failed: %s", err)
	}

	if !find {
		return xerrors.New("power actor not found")
	}

	ps, err := power.Load(adt.WrapStore(ctx, vms), pact)
	if err != nil {
		return err
	}

	_, exist, err := ps.MinerPower(maddr)
	if err != nil {
		return xerrors.Errorf("failed to look up miner's claim: %v", err)
	}

	if !exist {
		return xerrors.New("miner isn't valid")
	}

	return nil
}

func (c *Expected) minerHasMinPower(ctx context.Context, addr address.Address, ts *block.TipSet) (bool, error) {
	vms := vm.NewStorage(c.bstore)
	sm, err := state.LoadState(ctx, vms, ts.Blocks()[0].StateRoot.Cid)
	if err != nil {
		return false, xerrors.Errorf("loading state: %v", err)
	}

	pact, find, err := sm.GetActor(ctx, power.Address)
	if err != nil {
		return false, xerrors.Errorf("get power actor failed: %v", err)
	}

	if !find {
		return false, xerrors.New("power actor not found")
	}

	ps, err := power.Load(adt.WrapStore(ctx, vms), pact)
	if err != nil {
		return false, err
	}

	return ps.MinerNominalPowerMeetsConsensusMinimum(addr)
}

func (c *Expected) MinerEligibleToMine(ctx context.Context, addr address.Address, parentStateRoot cid.Cid, parentHeight abi.ChainEpoch, lookbackTs *block.TipSet) (bool, error) {
	hmp, err := c.minerHasMinPower(ctx, addr, lookbackTs)

	// TODO: We're blurring the lines between a "runtime network version" and a "Lotus upgrade epoch", is that unavoidable?
	if c.fork.GetNtwkVersion(ctx, parentHeight) <= network.Version3 {
		return hmp, err
	}

	if err != nil {
		return false, err
	}

	if !hmp {
		return false, nil
	}

	// Post actors v2, also check MinerEligibleForElection with base ts
	vms := vm.NewStorage(c.bstore)
	sm, err := state.LoadState(ctx, vms, parentStateRoot)
	if err != nil {
		return false, xerrors.Errorf("loading state: %v", err)
	}

	pact, find, err := sm.GetActor(ctx, power.Address)
	if err != nil {
		return false, xerrors.Errorf("get power actor failed: %v", err)
	}

	if !find {
		return false, xerrors.New("power actor not found")
	}

	pstate, err := power.Load(adt.WrapStore(ctx, c.cstore), pact)
	if err != nil {
		return false, err
	}

	mact, find, err := sm.GetActor(ctx, addr)
	if err != nil {
		return false, xerrors.Errorf("loading miner actor state: %v", err)
	}

	if !find {
		return false, xerrors.Errorf("miner actor %s not found", addr)
	}

	mstate, err := miner.Load(adt.WrapStore(ctx, vms), mact)
	if err != nil {
		return false, err
	}

	// Non-empty power claim.
	if claim, found, err := pstate.MinerPower(addr); err != nil {
		return false, err
	} else if !found {
		return false, err
	} else if claim.QualityAdjPower.LessThanEqual(big.Zero()) {
		return false, err
	}

	// No fee debt.
	if debt, err := mstate.FeeDebt(); err != nil {
		return false, err
	} else if !debt.IsZero() {
		return false, err
	}

	// No active consensus faults.
	if mInfo, err := mstate.Info(); err != nil {
		return false, err
	} else if parentHeight <= mInfo.ConsensusFaultElapsed {
		return false, nil
	}

	return true, nil
}

func (c *Expected) GetLookbackTipSetForRound(ctx context.Context, ts *block.TipSet, round abi.ChainEpoch) (*block.TipSet, cid.Cid, error) {
	var lbr abi.ChainEpoch
	lb := policy.GetWinningPoStSectorSetLookback(c.fork.GetNtwkVersion(ctx, round))
	if round > lb {
		lbr = round - lb
	}

	// more null blocks than our lookback
	h, _ := ts.Height()
	if lbr >= h {
		// This should never happen at this point, but may happen before
		// network version 3 (where the lookback was only 10 blocks).
		st, err := c.chainState.GetTipSetStateRoot(ts.Key())
		if err != nil {
			return nil, cid.Undef, err
		}
		return ts, st, nil
	}

	// Get the tipset after the lookback tipset, or the next non-null one.
	nextTs, err := c.chainState.GetTipSetByHeight(ctx, ts, lbr+1, false)
	if err != nil {
		return nil, cid.Undef, xerrors.Errorf("failed to get lookback tipset+1: %v", err)
	}

	nextTh, _ := nextTs.Height()
	if lbr > nextTh {
		return nil, cid.Undef, xerrors.Errorf("failed to find non-null tipset %s (%d) which is known to exist, found %s (%d)", ts.Key(), h, nextTs.Key(), nextTh)
	}

	pKey, err := nextTs.Parents()
	if err != nil {
		return nil, cid.Undef, err
	}
	lbts, err := c.chainState.GetTipSet(pKey)
	if err != nil {
		return nil, cid.Undef, xerrors.Errorf("failed to resolve lookback tipset: %v", err)
	}

	return lbts, nextTs.Blocks()[0].StateRoot.Cid, nil
}

// ResolveToKeyAddr returns the public key type of address (`BLS`/`SECP256K1`) of an account actor identified by `addr`.
func GetMinerWorkerRaw(ctx context.Context, stateID cid.Cid, bstore blockstore.Blockstore, addr address.Address) (address.Address, error) {
	vms := vm.NewStorage(bstore)
	state, err := state.LoadState(ctx, vms, stateID)
	if err != nil {
		return address.Undef, xerrors.Errorf("loading state: %v", err)
	}

	act, find, err := state.GetActor(ctx, addr)
	if err != nil {
		return address.Undef, xerrors.Errorf("(get sset) failed to load miner actor: %v", err)
	}

	if !find {
		return address.Undef, xerrors.Errorf("actor not found for %s", addr)
	}

	mas, err := miner.Load(adt.WrapStore(ctx, vms), act)
	if err != nil {
		return address.Undef, xerrors.Errorf("(get sset) failed to load miner actor state: %v", err)
	}

	info, err := mas.Info()
	if err != nil {
		return address.Undef, xerrors.Errorf("failed to load actor info: %v", err)
	}

	if info.Worker.Protocol() == address.BLS || info.Worker.Protocol() == address.SECP256K1 {
		return addr, nil
	}

	actWorker, find, err := state.GetActor(ctx, info.Worker)
	if err != nil {
		return address.Undef, xerrors.Errorf("(get sset) failed to load miner actor: %v", err)
	}

	if !find {
		return address.Undef, xerrors.Errorf("actor not found for %s", addr)
	}

	aast, err := account.Load(adt.WrapStore(context.TODO(), vms), actWorker)
	if err != nil {
		return address.Undef, xerrors.Errorf("failed to get account actor state for %s: %v", addr, err)
	}

	return aast.PubkeyAddress()
}

func (c *Expected) ValidateBlockWinner(ctx context.Context, lbTs *block.TipSet, lbRoot cid.Cid, baseTs *block.TipSet, baseRoot cid.Cid,
	blk *block.Block, prevEntry *block.BeaconEntry) error {
	if blk.ElectionProof.WinCount < 1 {
		return xerrors.Errorf("block is not claiming to be a winner")
	}

	baseHeight, _ := baseTs.Height()
	eligible, err := c.MinerEligibleToMine(ctx, blk.Miner, baseRoot, baseHeight, lbTs)
	if err != nil {
		return xerrors.Errorf("determining if miner has min power failed: %v", err)
	}

	if !eligible {
		return xerrors.New("block's miner is ineligible to mine")
	}

	rBeacon := prevEntry
	if len(blk.BeaconEntries) != 0 {
		rBeacon = blk.BeaconEntries[len(blk.BeaconEntries)-1]
	}
	buf := new(bytes.Buffer)
	if err := blk.Miner.MarshalCBOR(buf); err != nil {
		return xerrors.Errorf("failed to marshal miner address to cbor: %s", err)
	}

	vrfBase, err := chain.DrawRandomness(rBeacon.Data, acrypto.DomainSeparationTag_ElectionProofProduction, blk.Height, buf.Bytes())
	if err != nil {
		return xerrors.Errorf("could not draw randomness: %s", err)
	}

	waddr, err := GetMinerWorkerRaw(ctx, baseRoot, c.bstore, blk.Miner)
	if err != nil {
		return xerrors.Errorf("query worker address failed: %s", err)
	}

	if err := VerifyElectionPoStVRF(ctx, waddr, vrfBase, blk.ElectionProof.VRFProof); err != nil {
		return xerrors.Errorf("validating block election proof failed: %s", err)
	}

	view := c.state.PowerStateView(lbRoot)
	if view == nil {
		return xerrors.New("power state view is null")
	}

	_, qaPower, err := view.MinerClaimedPower(ctx, blk.Miner)
	if err != nil {
		return xerrors.Errorf("get miner power failed: %s", err)
	}

	tpow, err := view.PowerNetworkTotal(ctx)
	if err != nil {
		return xerrors.Errorf("get network total power failed: %s", err)
	}

	j := blk.ElectionProof.ComputeWinCount(qaPower, tpow.QualityAdjustedPower)
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

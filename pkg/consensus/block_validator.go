package consensus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	blockadt "github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/filecoin-project/venus/pkg/crypto/sigs"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/prometheus/common/log"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/Gurpartap/async"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/network"
	proof2 "github.com/filecoin-project/specs-actors/v2/actors/runtime/proof"
	"github.com/filecoin-project/venus/pkg/beacon"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/specactors/adt"
	"github.com/filecoin-project/venus/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/power"
	appstate "github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/types"
	bstore "github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	"github.com/hashicorp/go-multierror"
	lru "github.com/hashicorp/golang-lru"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"golang.org/x/xerrors"
)

var ErrTemporal = errors.New("temporal error")
var ErrSoftFailure = errors.New("soft validation failure")
var ErrInsufficientPower = errors.New("incoming block's miner does not have minimum power")

type BlockValidator struct {
	// TicketValidator validates ticket generation
	tv           TicketValidator
	bstore       blockstore.Blockstore
	messageStore *chain.MessageStore
	drand        beacon.Schedule
	// cstore is used for loading state trees during message running.
	cstore cbor.IpldStore
	// postVerifier verifies PoSt proofs and associated data
	proofVerifier ProofVerifier
	// state produces snapshots
	state StateViewer
	// Provides and stores validated tipsets and their state roots.
	chainState chainReader
	// Selects the heaviest of two chains
	chainSelector    *ChainSelector
	fork             fork.IFork
	config           *config.NetworkParamsConfig
	gasPirceSchedule *gas.PricesSchedule

	validateBlockCache *lru.ARCCache
}

func NewBlockValidator(tv TicketValidator,
	bstore blockstore.Blockstore,
	messageStore *chain.MessageStore,
	drand beacon.Schedule,
	cstore cbor.IpldStore,
	proofVerifier ProofVerifier,
	state StateViewer,
	chainState chainReader,
	chainSelector *ChainSelector,
	fork fork.IFork,
	config *config.NetworkParamsConfig,
	gasPirceSchedule *gas.PricesSchedule) *BlockValidator {
	validateBlockCache, _ := lru.NewARC(2048)
	return &BlockValidator{
		tv:                 tv,
		bstore:             bstore,
		messageStore:       messageStore,
		drand:              drand,
		cstore:             cstore,
		proofVerifier:      proofVerifier,
		state:              state,
		chainState:         chainState,
		chainSelector:      chainSelector,
		fork:               fork,
		config:             config,
		gasPirceSchedule:   gasPirceSchedule,
		validateBlockCache: validateBlockCache,
	}
}

func (bv *BlockValidator) ValidateBlockMsg(ctx context.Context, blk *types.BlockMsg) pubsub.ValidationResult {
	validationStart := time.Now()
	defer func() {
		logExpect.Debugw("block validation header", "Cid", blk.Cid(), "took", time.Since(validationStart), "height", blk.Header.Height, "age", time.Since(time.Unix(int64(blk.Header.Timestamp), 0)))
	}()

	return bv.validateBlockMsg(ctx, blk)
}

func (bv *BlockValidator) ValidateFullBlock(ctx context.Context, blk *types.BlockHeader) (err error) {
	validationStart := time.Now()
	defer func() {
		logExpect.Infow("block validation", "Cid", blk.Cid(), "took", time.Since(validationStart), "height", blk.Height, "age", time.Since(time.Unix(int64(blk.Timestamp), 0)), "Err", err)
	}()

	if _, ok := bv.validateBlockCache.Get(blk.Cid()); ok {
		return nil
	}

	err = bv.validateBlock(ctx, blk)

	if err == nil {
		bv.validateBlockCache.Add(blk.Cid(), struct{}{})
	}
	return err
}

func (bv *BlockValidator) validateBlock(ctx context.Context, blk *types.BlockHeader) error {
	parent, err := bv.chainState.GetTipSet(blk.Parents)
	if err != nil {
		return xerrors.Errorf("load parent tipset failed %w", err)
	}
	parentWeight, err := bv.chainSelector.Weight(ctx, parent)
	if err != nil {
		return xerrors.Errorf("calc parent weight failed %w", err)
	}
	parentReceiptRoot, err := bv.chainState.GetTipSetReceiptsRoot(parent)
	if err != nil {
		return xerrors.Errorf("get parent tipset state failed %w", err)
	}
	// confirm block state root matches parent state root
	rootAfterCalc, err := bv.chainState.GetTipSetStateRoot(parent)
	if err != nil {
		return xerrors.Errorf("get parent tipset state failed %w", err)
	}
	if !rootAfterCalc.Equals(blk.ParentStateRoot) {
		return xerrors.Errorf("%w (%s != %s)", ErrStateRootMismatch, rootAfterCalc, blk.ParentStateRoot)
	}

	if err := blockSanityChecks(blk); err != nil {
		return xerrors.Errorf("incoming header failed basic sanity checks: %w", err)
	}

	baseHeight := parent.Height()
	nulls := blk.Height - (baseHeight + 1)
	if tgtTS := parent.MinTimestamp() + bv.config.BlockDelay*uint64(nulls+1); blk.Timestamp != tgtTS {
		return xerrors.Errorf("block has wrong timestamp: %d != %d", blk.Timestamp, tgtTS)
	}

	now := uint64(time.Now().Unix())
	if blk.Timestamp > now+AllowableClockDriftSecs {
		return xerrors.Errorf("block was from the future (now=%d, blk=%d): %v", now, blk.Timestamp, ErrTemporal)
	}
	if blk.Timestamp > now {
		logExpect.Warn("Got block from the future, but within threshold", blk.Timestamp, time.Now().Unix())
	}

	// get parent beacon
	prevBeacon, err := bv.chainState.GetLatestBeaconEntry(parent)
	if err != nil {
		return xerrors.Errorf("failed to get latest beacon entry: %w", err)
	}

	// confirm block receipts match parent receipts
	if !parentReceiptRoot.Equals(blk.ParentMessageReceipts) {
		return ErrReceiptRootMismatch
	}

	if !parentWeight.Equals(blk.ParentWeight) {
		return xerrors.Errorf("block %s has invalid parent weight %d expected %d", blk.Cid().String(), blk.ParentWeight, parentWeight)
	}

	// get worker address
	version := bv.fork.GetNtwkVersion(ctx, blk.Height)
	lbTS, lbStateRoot, err := bv.chainState.GetLookbackTipSetForRound(ctx, parent, blk.Height, version)
	if err != nil {
		return xerrors.Errorf("failed to get lookback tipset for block: %w", err)
	}

	powerStateView := bv.state.PowerStateView(lbStateRoot)
	workerAddr, err := powerStateView.GetMinerWorkerRaw(ctx, blk.Miner)
	if err != nil {
		return xerrors.Errorf("query worker address failed: %w", err)
	}

	minerCheck := async.Err(func() error {
		if err := bv.minerIsValid(ctx, blk.Miner, blk.ParentStateRoot); err != nil {
			return xerrors.Errorf("minerIsValid failed: %w", err)
		}
		return nil
	})

	baseFeeCheck := async.Err(func() error {
		baseFee, err := bv.messageStore.ComputeBaseFee(ctx, parent, bv.config.ForkUpgradeParam)
		if err != nil {
			return xerrors.Errorf("computing base fee: %w", err)
		}

		if big.Cmp(baseFee, blk.ParentBaseFee) != 0 {
			return xerrors.Errorf("base fee doesn't match: %s (header) != %s (computed)", blk.ParentBaseFee, baseFee)
		}
		return nil
	})

	blockSigCheck := async.Err(func() error {
		// Validate block signature
		return crypto.ValidateSignature(blk.SignatureData(), workerAddr, *blk.BlockSig)
	})

	beaconValuesCheck := async.Err(func() error {
		parentHeight := parent.Height()
		if err = bv.ValidateBlockBeacon(blk, parentHeight, prevBeacon); err != nil {
			return err
		}
		return nil
	})

	tktsCheck := async.Err(func() error {
		beaconBase, err := bv.beaconBaseEntry(ctx, blk)
		if err != nil {
			return xerrors.Errorf("failed to get election entry %w", err)
		}

		sampleEpoch := blk.Height - constants.TicketRandomnessLookback
		bSmokeHeight := blk.Height > bv.config.ForkUpgradeParam.UpgradeSmokeHeight
		if err := bv.tv.IsValidTicket(ctx, blk.Parents, beaconBase, bSmokeHeight, sampleEpoch, blk.Miner, workerAddr, blk.Ticket); err != nil {
			return xerrors.Errorf("invalid ticket: %s in block %s %w", blk.Ticket.String(), blk.Cid(), err)
		}
		return nil
	})

	winnerCheck := async.Err(func() error {
		if err = bv.ValidateBlockWinner(ctx, workerAddr, lbTS, lbStateRoot, parent, parent.At(0).ParentStateRoot, blk, prevBeacon); err != nil {
			return err
		}
		return nil
	})

	winPoStNv := bv.fork.GetNtwkVersion(ctx, baseHeight)
	wproofCheck := async.Err(func() error {
		if err := bv.VerifyWinningPoStProof(ctx, winPoStNv, blk, prevBeacon, lbStateRoot); err != nil {
			return xerrors.Errorf("invalid election post: %w", err)
		}
		return nil
	})

	msgsCheck := async.Err(func() error {
		keyStateView := bv.state.PowerStateView(blk.ParentStateRoot)
		sigValidator := appstate.NewSignatureValidator(keyStateView)
		if err := bv.checkBlockMessages(ctx, sigValidator, blk, parent); err != nil {
			return xerrors.Errorf("block had invalid messages: %w", err)
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
		baseFeeCheck,
		msgsCheck,
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

func (bv *BlockValidator) validateBlockMsg(ctx context.Context, blk *types.BlockMsg) pubsub.ValidationResult {
	// validate the block meta: the Message CID in the header must match the included messages
	err := bv.validateMsgMeta(ctx, blk)
	if err != nil {
		logExpect.Warnf("error validating message metadata: %s", err)
		return pubsub.ValidationReject
	}

	// we want to ensure that it is a block from a known miner; we reject blocks from unknown miners
	// to prevent spam attacks.
	// the logic works as follows: we lookup the miner in the chain for its key.
	// if we can find it then it's a known miner and we can validate the signature.
	// if we can't find it, we check whether we are (near) synced in the chain.
	// if we are not synced we cannot validate the block and we must ignore it.
	// if we are synced and the miner is unknown, then the block is rejcected.
	key, err := bv.checkPowerAndGetWorkerKey(ctx, blk.Header)
	if err != nil {
		if err != ErrSoftFailure {
			logExpect.Errorf("received block from unknown miner or miner that doesn't meet min power over pubsub; rejecting message")
			return pubsub.ValidationReject
		}

		logExpect.Errorf("cannot validate block message; unknown miner or miner that doesn't meet min power in unsynced chain")
		return pubsub.ValidationIgnore
	}

	err = sigs.CheckBlockSignature(ctx, blk.Header, key)
	if err != nil {
		logExpect.Errorf("block signature verification failed: %s", err)
		return pubsub.ValidationReject
	}

	if blk.Header.ElectionProof.WinCount < 1 {
		logExpect.Errorf("block is not claiming to be winning")
		return pubsub.ValidationReject
	}

	return pubsub.ValidationAccept
}

func (bv *BlockValidator) validateMsgMeta(ctx context.Context, msg *types.BlockMsg) error {
	// TODO there has to be a simpler way to do this without the blockstore dance
	// block headers use adt0
	store := blockadt.WrapStore(ctx, cbor.NewCborStore(bstore.NewTemporary()))
	bmArr := blockadt.MakeEmptyArray(store)
	smArr := blockadt.MakeEmptyArray(store)

	for i, m := range msg.BlsMessages {
		c := cbg.CborCid(m)
		if err := bmArr.Set(uint64(i), &c); err != nil {
			return err
		}
	}

	for i, m := range msg.SecpkMessages {
		c := cbg.CborCid(m)
		if err := smArr.Set(uint64(i), &c); err != nil {
			return err
		}
	}

	bmroot, err := bmArr.Root()
	if err != nil {
		return err
	}

	smroot, err := smArr.Root()
	if err != nil {
		return err
	}

	mrcid, err := store.Put(store.Context(), &types.TxMeta{
		BLSRoot:  bmroot,
		SecpRoot: smroot,
	})

	if err != nil {
		return err
	}

	if msg.Header.Messages != mrcid {
		return fmt.Errorf("messages didn't match root cid in header")
	}

	return nil
}

func (bv *BlockValidator) checkPowerAndGetWorkerKey(ctx context.Context, bh *types.BlockHeader) (address.Address, error) {
	// we check that the miner met the minimum power at the lookback tipset

	baseTS := bv.chainState.GetHead()
	version := bv.fork.GetNtwkVersion(ctx, bh.Height)
	lbts, lbst, err := bv.chainState.GetLookbackTipSetForRound(ctx, baseTS, bh.Height, version)
	if err != nil {
		log.Warnf("failed to load lookback tipset for incoming block: %s", err)
		return address.Undef, ErrSoftFailure
	}

	powerStateView := bv.state.PowerStateView(lbst)
	key, err := powerStateView.GetMinerWorkerRaw(ctx, bh.Miner)
	if err != nil {
		log.Warnf("failed to resolve worker key for miner %s: %s", bh.Miner, err)
		return address.Undef, ErrSoftFailure
	}

	// NOTE: we check to see if the miner was eligible in the lookback
	// tipset - 1 for historical reasons. DO NOT use the lookback state
	// returned by GetLookbackTipSetForRound.

	eligible, err := bv.MinerEligibleToMine(ctx, bh.Miner, baseTS.At(0).ParentStateRoot, baseTS.Height(), lbts)
	if err != nil {
		log.Warnf("failed to determine if incoming block's miner has minimum power: %s", err)
		return address.Undef, ErrSoftFailure
	}

	if !eligible {
		log.Warnf("incoming block's miner is ineligible")
		return address.Undef, ErrInsufficientPower
	}

	return key, nil
}

func (bv *BlockValidator) minerIsValid(ctx context.Context, maddr address.Address, baseStateRoot cid.Cid) error {
	vms := cbor.NewCborStore(bv.bstore)
	sm, err := tree.LoadState(ctx, vms, baseStateRoot)
	if err != nil {
		return xerrors.Errorf("loading state: %w", err)
	}

	pact, find, err := sm.GetActor(ctx, power.Address)
	if err != nil {
		return xerrors.Errorf("get power actor failed: %w", err)
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
		return xerrors.Errorf("failed to look up miner's claim: %w", err)
	}

	if !exist {
		return xerrors.New("miner isn't valid")
	}

	return nil
}

func (bv *BlockValidator) ValidateBlockBeacon(blk *types.BlockHeader, parentEpoch abi.ChainEpoch, prevEntry *types.BeaconEntry) error {
	if os.Getenv("VENUS_IGNORE_DRAND") == "_yes_" {
		return nil
	}
	return beacon.ValidateBlockValues(bv.drand, blk, parentEpoch, prevEntry)
}

func (bv *BlockValidator) beaconBaseEntry(ctx context.Context, blk *types.BlockHeader) (*types.BeaconEntry, error) {
	if len(blk.BeaconEntries) > 0 {
		return blk.BeaconEntries[len(blk.BeaconEntries)-1], nil
	}

	parent, err := bv.chainState.GetTipSet(blk.Parents)
	if err != nil {
		return nil, err
	}
	return chain.FindLatestDRAND(ctx, parent, bv.chainState)
}

func (bv *BlockValidator) ValidateBlockWinner(ctx context.Context, waddr address.Address, lbTS *types.TipSet, lbRoot cid.Cid, baseTS *types.TipSet, baseRoot cid.Cid,
	blk *types.BlockHeader, prevEntry *types.BeaconEntry) error {
	if blk.ElectionProof.WinCount < 1 {
		return xerrors.Errorf("block is not claiming to be a winner")
	}

	baseHeight := baseTS.Height()
	eligible, err := bv.MinerEligibleToMine(ctx, blk.Miner, baseRoot, baseHeight, lbTS)
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

	if err := VerifyElectionPoStVRF(ctx, waddr, vrfBase, blk.ElectionProof.VRFProof); err != nil {
		return xerrors.Errorf("validating block election proof failed: %s", err)
	}

	view := bv.state.PowerStateView(lbRoot)
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

func (bv *BlockValidator) MinerEligibleToMine(ctx context.Context, addr address.Address, parentStateRoot cid.Cid, parentHeight abi.ChainEpoch, lookbackTS *types.TipSet) (bool, error) {
	hmp, err := bv.minerHasMinPower(ctx, addr, lookbackTS)

	// TODO: We're blurring the lines between a "runtime network version" and a "Lotus upgrade epoch", is that unavoidable?
	if bv.fork.GetNtwkVersion(ctx, parentHeight) <= network.Version3 {
		return hmp, err
	}

	if err != nil {
		return false, err
	}

	if !hmp {
		return false, nil
	}

	// Post actors v2, also check MinerEligibleForElection with base ts
	vms := cbor.NewCborStore(bv.bstore)
	sm, err := tree.LoadState(ctx, vms, parentStateRoot)
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

	pstate, err := power.Load(adt.WrapStore(ctx, bv.cstore), pact)
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
		return false, nil
	} else if claim.QualityAdjPower.LessThanEqual(big.Zero()) {
		logExpect.Infof("miner address:%v", addr.String())
		logExpect.Warnf("miner quality adjust power:%v is less than zero", claim.QualityAdjPower)
		return false, nil
	}

	// No fee debt.
	if debt, err := mstate.FeeDebt(); err != nil {
		return false, err
	} else if !debt.IsZero() {
		logExpect.Warnf("the debt:%v is not zero", debt)
		return false, nil
	}

	// No active consensus faults.
	if mInfo, err := mstate.Info(); err != nil {
		return false, err
	} else if parentHeight <= mInfo.ConsensusFaultElapsed {
		return false, nil
	}

	return true, nil
}

func (bv *BlockValidator) minerHasMinPower(ctx context.Context, addr address.Address, ts *types.TipSet) (bool, error) {
	vms := cbor.NewCborStore(bv.bstore)
	sm, err := tree.LoadState(ctx, vms, ts.Blocks()[0].ParentStateRoot)
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

func (bv *BlockValidator) VerifyWinningPoStProof(ctx context.Context, nv network.Version, blk *types.BlockHeader, prevBeacon *types.BeaconEntry, lbst cid.Cid) error {
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

	view := bv.state.PowerStateView(lbst)
	if view == nil {
		return xerrors.New("power state view is null")
	}

	sectors, err := view.GetSectorsForWinningPoSt(ctx, nv, bv.proofVerifier, lbst, blk.Miner, rand)
	if err != nil {
		return xerrors.Errorf("getting winning post sector set: %v", err)
	}

	proofs := make([]proof2.PoStProof, len(blk.WinPoStProof))
	for idx, pf := range blk.WinPoStProof {
		proofs[idx] = proof2.PoStProof{PoStProof: pf.PoStProof, ProofBytes: pf.ProofBytes}
	}

	ok, err := bv.proofVerifier.VerifyWinningPoSt(ctx, proof2.WinningPoStVerifyInfo{
		Randomness:        rand,
		Proofs:            proofs,
		ChallengedSectors: sectors,
		Prover:            abi.ActorID(mid),
	})

	if err != nil {
		return xerrors.Errorf("failed to verify election post: %v", err)
	}

	if !ok {
		logExpect.Errorf("invalid winning post (block: %s, %x; %v)", blk.Cid(), rand, sectors)
		return xerrors.Errorf("winning post was invalid")
	}

	return nil
}

// TODO: We should extract this somewhere else and make the message pool and miner use the same logic
func (bv *BlockValidator) checkBlockMessages(ctx context.Context, sigValidator *appstate.SignatureValidator, blk *types.BlockHeader, baseTS *types.TipSet) (err error) {
	blksecpMsgs, blkblsMsgs, err := bv.messageStore.LoadMetaMessages(ctx, blk.Messages)
	if err != nil {
		return xerrors.Errorf("failed loading message list %s for block %s %v", blk.Messages, blk.Cid(), err)
	}

	{
		// Verify that the BLS signature aggregate is correct
		if err := sigValidator.ValidateBLSMessageAggregate(ctx, blkblsMsgs, blk.BLSAggregate); err != nil {
			return xerrors.Errorf("bls message verification failed for block %s %v", blk.Cid(), err)
		}

		// Verify that all secp message signatures are correct
		for i, msg := range blksecpMsgs {
			if err := sigValidator.ValidateMessageSignature(ctx, msg); err != nil {
				return xerrors.Errorf("invalid signature for secp message %d in block %s %v", i, blk.Cid(), err)
			}
		}
	}

	nonces := make(map[address.Address]uint64)
	vms := cbor.NewCborStore(bv.bstore)
	st, err := tree.LoadState(ctx, vms, blk.ParentStateRoot)
	if err != nil {
		return xerrors.Errorf("loading state: %v", err)
	}

	baseHeight := baseTS.Height()
	pl := bv.gasPirceSchedule.PricelistByEpoch(baseHeight)
	var sumGasLimit int64
	checkMsg := func(msg types.ChainMsg) error {
		m := msg.VMMessage()

		// Phase 1: syntactic validation, as defined in the spec
		minGas := pl.OnChainMessage(msg.ChainLength())
		if err := m.ValidForBlockInclusion(minGas.Total(), bv.fork.GetNtwkVersion(ctx, blk.Height)); err != nil {
			return err
		}

		// ValidForBlockInclusion checks if any single message does not exceed BlockGasLimit
		// So below is overflow safe
		sumGasLimit += m.GasLimit
		if sumGasLimit > constants.BlockGasLimit {
			return xerrors.Errorf("block gas limit exceeded")
		}

		// Phase 2: (Partial) semantic validation:
		// the sender exists and is an account actor, and the nonces make sense
		var sender address.Address
		if bv.fork.GetNtwkVersion(ctx, blk.Height) >= network.Version13 {
			sender, err = st.LookupID(m.From)
			if err != nil {
				return err
			}
		} else {
			sender = m.From
		}

		if _, ok := nonces[sender]; !ok {
			// `GetActor` does not validate that this is an account actor.
			act, find, err := st.GetActor(ctx, sender)
			if err != nil {
				return xerrors.Errorf("failed to get actor: %v", err)
			}

			if !find {
				return xerrors.Errorf("actor %s not found", sender)
			}

			if !builtin.IsAccountActor(act.Code) {
				return xerrors.New("Sender must be an account actor")
			}
			nonces[sender] = act.Nonce
		}

		if nonces[sender] != m.Nonce {
			return xerrors.Errorf("wrong nonce (exp: %d, got: %d)", nonces[sender], m.Nonce)
		}
		nonces[sender]++

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

	txMeta := &types.TxMeta{
		BLSRoot:  bmroot,
		SecpRoot: smroot,
	}
	b, err := chain.MakeBlock(txMeta)
	if err != nil {
		return xerrors.Errorf("serialize tx meta failed: %v", err)
	}
	if blk.Messages != b.Cid() {
		return fmt.Errorf("messages didnt match message root in header")
	}
	return nil
}

// ValidateMsgMeta performs structural and content hash validation of the
// messages within this block. If validation passes, it stores the messages in
// the underlying IPLD block store.
func (bv *BlockValidator) ValidateMsgMeta(fblk *types.FullBlock) error {
	if msgc := len(fblk.BLSMessages) + len(fblk.SECPMessages); msgc > constants.BlockMessageLimit {
		return xerrors.Errorf("block %s has too many messages (%d)", fblk.Header.Cid(), msgc)
	}

	// TODO: IMPORTANT(GARBAGE). These message puts and the msgmeta
	// computation need to go into the 'temporary' side of the blockstore when
	// we implement that

	// We use a temporary bstore here to avoid writing intermediate pieces
	// into the blockstore.
	blockstore := bstore.NewTemporary()
	var bcids, scids []cid.Cid

	for _, m := range fblk.BLSMessages {
		c, err := chain.PutMessage(blockstore, m)
		if err != nil {
			return xerrors.Errorf("putting bls message to blockstore after msgmeta computation: %v", err)
		}
		bcids = append(bcids, c)
	}

	for _, m := range fblk.SECPMessages {
		c, err := chain.PutMessage(blockstore, m)
		if err != nil {
			return xerrors.Errorf("putting bls message to blockstore after msgmeta computation: %w", err)
		}
		scids = append(scids, c)
	}

	// Compute the root CID of the combined message trie.
	smroot, err := chain.ComputeMsgMeta(blockstore, bcids, scids)
	if err != nil {
		return xerrors.Errorf("validating msgmeta, compute failed: %v", err)
	}

	// Check that the message trie root matches with what's in the block.
	if fblk.Header.Messages != smroot {
		return xerrors.Errorf("messages in full block did not match msgmeta root in header (%s != %s)", fblk.Header.Messages, smroot)
	}

	// Finally, flush
	return bstore.CopyParticial(context.TODO(), blockstore, bv.bstore, smroot)
}

func blockSanityChecks(b *types.BlockHeader) error {
	if b.ElectionProof == nil {
		return xerrors.Errorf("block cannot have nil election proof")
	}

	if b.BlockSig == nil {
		return xerrors.Errorf("block had nil signature")
	}

	if b.BLSAggregate == nil {
		return xerrors.Errorf("block had nil bls aggregate signature")
	}

	return nil
}

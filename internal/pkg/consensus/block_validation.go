package consensus

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Gurpartap/async"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/runtime/proof"
	"github.com/hashicorp/go-multierror"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/go-filecoin/internal/pkg/beacon"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

var log = logging.Logger("consensus")

type messageStore interface {
	LoadMessages(context.Context, cid.Cid) ([]*types.SignedMessage, []*types.UnsignedMessage, error)
	LoadReceipts(context.Context, cid.Cid) ([]vm.MessageReceipt, error)
	ComputeBaseFee(context.Context, *block.TipSet) (abi.TokenAmount, error)
}

type chainState interface {
	GetActorAt(context.Context, block.TipSetKey, address.Address) (*actor.Actor, error)
	GetTipSet(block.TipSetKey) (block.TipSet, error)
	GetTipSetStateRoot(context.Context, block.TipSetKey) (cid.Cid, error)
	StateView(block.TipSetKey) (*state.View, error)
	AccountStateView(block.TipSetKey) (state.AccountStateView, error)
	Weight(context.Context, *block.TipSet) (abi.TokenAmount, error)
	GetBlock(context.Context, cid.Cid) (*block.Block, error)
	BeaconSchedule() beacon.Schedule
}

// BlockValidator defines an interface used to validate a blocks syntax and
// semantics.
type BlockValidator interface {
	BlockSemanticValidator
	BlockSyntaxValidator
}

// SyntaxValidator defines and interface used to validate block's syntax and the
// syntax of constituent messages
type SyntaxValidator interface {
	BlockSyntaxValidator
	MessageSyntaxValidator
}

// BlockSemanticValidator defines an interface used to validate a blocks
// semantics.
type BlockSemanticValidator interface {
	ValidateHeaderSemantic(ctx context.Context, child *block.Block, parents block.TipSet) error
	ValidateMessagesSemantic(ctx context.Context, child *block.Block, parents block.TipSetKey) error
}

// BlockSyntaxValidator defines an interface used to validate a blocks
// syntax.
type BlockSyntaxValidator interface {
	ValidateSyntax(ctx context.Context, blk *block.Block) error
}

// MessageSyntaxValidator defines an interface used to validate a message's
// syntax.
type MessageSyntaxValidator interface {
	ValidateSignedMessageSyntax(ctx context.Context, smsg *types.SignedMessage) error
	ValidateUnsignedMessageSyntax(ctx context.Context, msg *types.UnsignedMessage) error
}

// DefaultBlockValidator implements the BlockValidator interface.
type DefaultBlockValidator struct {
	clock.ChainEpochClock
	ms messageStore
	cs chainState
}

// WrappedSyntaxValidator implements syntax validator interface
type WrappedSyntaxValidator struct {
	BlockSyntaxValidator
	MessageSyntaxValidator
}

// NewDefaultBlockValidator returns a new DefaultBlockValidator. It uses `blkTime`
// to validate blocks and uses the DefaultBlockValidationClock.
func NewDefaultBlockValidator(c clock.ChainEpochClock, m messageStore, cs chainState) *DefaultBlockValidator {
	return &DefaultBlockValidator{
		ChainEpochClock: c,
		ms:              m,
		cs:              cs,
	}
}

// ValidateHeaderSemantic checks validation conditions on a header that can be
// checked given only the parent header.
func (dv *DefaultBlockValidator) ValidateHeaderSemantic(ctx context.Context, child *block.Block, parents block.TipSet) error {
	ph, err := parents.Height()
	if err != nil {
		return err
	}

	if child.Height <= ph {
		return fmt.Errorf("block %s has invalid height %d", child.Cid().String(), child.Height)
	}

	return nil
}

func (dv *DefaultBlockValidator) validateMessage(msg *types.UnsignedMessage, expectedCallSeqNum map[address.Address]uint64, fromActor *actor.Actor) error {
	callSeq, ok := expectedCallSeqNum[msg.From]
	if !ok {
		callSeq = fromActor.CallSeqNum
	}

	// ensure message is in the correct order
	if callSeq != msg.CallSeqNum {
		return fmt.Errorf("callseqnum (%d) out of order (expected %d) from %s", msg.CallSeqNum, callSeq, msg.From)
	}

	expectedCallSeqNum[msg.From] = callSeq + 1
	return nil
}

// ValidateFullSemantic checks validation conditions on a block's messages that don't require message execution.
func (dv *DefaultBlockValidator) ValidateMessagesSemantic(ctx context.Context, child *block.Block, parents block.TipSetKey) error {
	// validate call sequence numbers
	secpMsgs, blsMsgs, err := dv.ms.LoadMessages(ctx, child.Messages.Cid)
	if err != nil {
		return errors.Wrapf(err, "block validation failed loading message list %s for block %s", child.Messages, child.Cid())
	}

	expectedCallSeqNum := map[address.Address]uint64{}
	for _, msg := range blsMsgs {
		msgCid, err := msg.Cid()
		if err != nil {
			return err
		}

		from, err := dv.getAndValidateFromActor(ctx, msg, parents)
		if err != nil {
			return errors.Wrapf(err, "from actor %s for message %s of block %s invalid", msg.From, msgCid, child.Cid())
		}

		err = dv.validateMessage(msg, expectedCallSeqNum, from)
		if err != nil {
			return errors.Wrapf(err, "message %s of block %s invalid", msgCid, child.Cid())
		}
	}

	for _, msg := range secpMsgs {
		msgCid, err := msg.Cid()
		if err != nil {
			return err
		}

		from, err := dv.getAndValidateFromActor(ctx, &msg.Message, parents)
		if err != nil {
			return errors.Wrapf(err, "from actor %s for message %s of block %s invalid", msg.Message.From, msgCid, child.Cid())
		}

		err = dv.validateMessage(&msg.Message, expectedCallSeqNum, from)
		if err != nil {
			return errors.Wrapf(err, "message %s of block %s invalid", msgCid, child.Cid())
		}
	}

	return nil
}

func (dv *DefaultBlockValidator) getAndValidateFromActor(ctx context.Context, msg *types.UnsignedMessage, parents block.TipSetKey) (*actor.Actor, error) {
	actor, err := dv.cs.GetActorAt(ctx, parents, msg.From)
	if err != nil {
		return nil, err
	}

	// ensure actor is an account actor
	if !actor.Code.Equals(builtin.AccountActorCodeID) {
		return nil, errors.New("sent from non-account actor")
	}

	return actor, nil
}

var ErrTemporal = errors.New("temporal error")

func (dv *DefaultBlockValidator) blockSanityChecks(h *block.Block) error {
	if h.ElectionProof == nil {
		return xerrors.Errorf("block cannot have nil election proof")
	}

	if len(h.Ticket.VRFProof) <= 0 {
		return xerrors.Errorf("block cannot have nil ticket")
	}

	if h.BlockSig == nil {
		return xerrors.Errorf("block had nil signature")
	}

	if h.BLSAggregateSig == nil {
		return xerrors.Errorf("block had nil bls aggregate signature")
	}

	return nil
}

func GetLookbackTipSetForRound(ctx context.Context, ch chainState, ts *block.TipSet, round abi.ChainEpoch) (*block.TipSet, error) {
	var lbr abi.ChainEpoch
	if round > WinningPoStSectorSetLookback {
		lbr = round - WinningPoStSectorSetLookback
	}

	// more null blocks than our lookback
	ph, err := ts.Height()
	if err != nil {
		return nil, xerrors.Errorf("failed to get lookback tipset: %w", err)
	}
	if lbr > ph {
		return ts, nil
	}

	//
	targetTipset, err := chain.FindTipsetAtEpoch(ctx, *ts, lbr, ch)
	if err != nil {
		return nil, err
	}

	return &targetTipset, nil
}

//func (dv *DefaultBlockValidator) verifyBlsAggregate(ctx context.Context, sig *crypto.Signature, msgs []cid.Cid, pubks [][]byte) error {
//	msgsS := make([]blst.Message, len(msgs))
//	for i := 0; i < len(msgs); i++ {
//		msgsS[i] = msgs[i].Bytes()
//	}
//
//	if len(msgs) == 0 {
//		return nil
//	}
//
//	valid := new(bls.Signature).AggregateVerifyCompressed(sig.Data, pubks,
//		msgsS, []byte(bls.DST))
//	if !valid {
//		return xerrors.New("bls aggregate signature failed to verify")
//	}
//	return nil
//}

func (dv *DefaultBlockValidator) checkBlockMessages(ctx context.Context, b *block.Block, baseTs *block.TipSet) error {
	// ToDo review: block sig
	view, err := dv.cs.AccountStateView(b.Parents)
	if err != nil {
		return errors.Wrapf(err, "failed to load state at %v", b.Parents)
	}

	sigValidator := state.NewSignatureValidator(view)

	secpMsgs, blsMsgs, err := dv.ms.LoadMessages(ctx, b.Messages.Cid)
	if err != nil {
		return errors.Wrapf(err, "block validation failed loading message list %s for block %s", b.Messages, b.Cid())
	}

	// ensure message is properly signed
	if err := sigValidator.ValidateBLSMessageAggregate(ctx, blsMsgs, b.BLSAggregateSig); err != nil {
		return errors.Wrap(err, fmt.Errorf("invalid signature by sender over message data").Error())
	}

	// ToDo nonce check
	callSeqNums := make(map[address.Address]uint64)
	baseHeight, err := baseTs.Height()
	if err != nil {
		return err
	}
	pl := gas.PricelistByEpoch(baseHeight)
	var sumGasLimit int64
	checkMsg := func(msg types.ChainMsg) error {
		m := msg.VMMessage()

		// Phase 1: syntactic validation, as defined in the spec
		minGas := pl.OnChainMessage(msg.ChainLength())
		if err := m.ValidForBlockInclusion(minGas.Total()); err != nil {
			return err
		}

		// ValidForBlockInclusion checks if any single message does not exceed BlockGasLimit
		// So below is overflow safe
		sumGasLimit += int64(m.GasLimit)
		if sumGasLimit > types.BlockGasLimit {
			return xerrors.Errorf("block gas limit exceeded")
		}

		// Phase 2: (Partial) semantic validation:
		// the sender exists and is an account actor, and the nonces make sense
		if _, ok := callSeqNums[m.From]; !ok {
			// `GetActor` does not validate that this is an account actor.
			act, err := dv.cs.GetActorAt(ctx, baseTs.Key(), m.From)
			if err != nil {
				return xerrors.Errorf("failed to get actor: %w", err)
			}

			if !act.IsAccountActor() {
				return xerrors.New("Sender must be an account actor")
			}
			callSeqNums[m.From] = act.CallSeqNum
		}

		if callSeqNums[m.From] != m.CallSeqNum {
			return xerrors.Errorf("wrong nonce (exp: %d, got: %d)", callSeqNums[m.From], m.CallSeqNum)
		}
		callSeqNums[m.From]++

		return nil
	}

	// ToDo lotus中存储到adt的逻辑没有引入
	for i, m := range blsMsgs {
		if err := checkMsg(m); err != nil {
			return xerrors.Errorf("block had invalid bls message at index %d: %w", i, err)
		}
	}

	for i, m := range secpMsgs {
		if err := checkMsg(m); err != nil {
			return xerrors.Errorf("block had invalid secpk message at index %d: %w", i, err)
		}

		// `From` being an account actor is only validated inside the `vm.ResolveToKeyAddr` call
		// in `StateManager.ResolveToKeyAddress` here (and not in `checkMsg`).
		view, err := dv.cs.AccountStateView(b.Parents)
		if err != nil {
			return errors.Wrapf(err, "failed to load state at %v", b.Parents)
		}

		sigValidator := state.NewSignatureValidator(view)

		if err := sigValidator.ValidateMessageSignature(ctx, m); err != nil {
			return errors.Wrap(err, fmt.Errorf("invalid signature by sender over message data").Error())
		}
	}

	return nil
}

func (dv *DefaultBlockValidator) VerifyWinningPoStProof(ctx context.Context, b *block.Block, prevBeacon block.BeaconEntry, lbst cid.Cid, waddr address.Address) error {
	if InsecurePoStValidation {
		if len(b.WinPoStProof) == 0 {
			return xerrors.Errorf("[INSECURE-POST-VALIDATION] No winning post proof given")
		}

		if string(b.WinPoStProof[0].ProofBytes) == "valid proof" {
			return nil
		}
		return xerrors.Errorf("[INSECURE-POST-VALIDATION] winning post was invalid")
	}

	buf := new(bytes.Buffer)
	if err := b.Miner.MarshalCBOR(buf); err != nil {
		return xerrors.Errorf("failed to marshal miner address: %w", err)
	}

	// ToDo 完善sector接口后再做
	//rbase := prevBeacon
	//if len(b.BeaconEntries) > 0 {
	//	rbase = *b.BeaconEntries[len(b.BeaconEntries)-1]
	//}

	//rand, err := cst.DrawRandomness(rbase.Data, crypto.DomainSeparationTag_WinningPoStChallengeSeed, b.Height, buf.Bytes())
	//if err != nil {
	//	return xerrors.Errorf("failed to get randomness for verifying winning post proof: %w", err)
	//}
	//
	//mid, err := address.IDFromAddress(b.Miner)
	//if err != nil {
	//	return xerrors.Errorf("failed to get ID from miner address %s: %w", b.Miner, err)
	//}

	//sectors, err := cst.GetSectorsForWinningPoSt(ctx, DefaultProofVerifier(), lbst, b.Miner, rand)
	//if err != nil {
	//	return xerrors.Errorf("getting winning post sector set: %w", err)
	//}
	//
	//proofs := make([]proof.PoStProof,len(b.WinPoStProof))
	//for i, proof := range b.WinPoStProof{
	//	proofs[i].ProofBytes=proof.ProofBytes[:]
	//	proofs[i].PoStProof=proof.PoStProof
	//}
	//ok, err := DefaultProofVerifier().VerifyWinningPoSt(ctx, proof.WinningPoStVerifyInfo{
	//	Randomness:        rand,
	//	Proofs:            proofs,
	//	ChallengedSectors: sectors,
	//	Prover:            abi.ActorID(mid),
	//})
	//if err != nil {
	//	return xerrors.Errorf("failed to verify election post: %w", err)
	//}
	//
	//if !ok {
	//	log.Errorf("invalid winning post (block: %s, %x; %v)", b.Cid(), rand, sectors)
	//	return xerrors.Errorf("winning post was invalid")
	//}

	return nil
}

// ValidateSyntax validates a single block is correctly formed.
// ToDo review
func (dv *DefaultBlockValidator) ValidateSyntax(ctx context.Context, blk *block.Block) (err error) {
	defer func() {
		if rerr := recover(); rerr != nil {
			err = xerrors.Errorf("validate block panic: %w", rerr)
			return
		}
	}()

	if err := dv.blockSanityChecks(blk); err != nil {
		return xerrors.Errorf("incoming header failed basic sanity checks: %w", err)
	}

	baseTs, err := dv.cs.GetTipSet(blk.Parents)
	if err != nil {
		return xerrors.Errorf("load parent tipset failed (%s): %w", blk.Parents, err)
	}

	lbts, err := GetLookbackTipSetForRound(ctx, dv.cs, &baseTs, blk.Height)
	if err != nil {
		return xerrors.Errorf("failed to get lookback tipset for block: %w", err)
	}

	lbst, err := dv.cs.GetTipSetStateRoot(ctx, lbts.Key())
	if err != nil {
		return xerrors.Errorf("failed to compute lookback tipset state: %w", err)
	}

	prevBeacon, err := chain.FindLatestDRAND(ctx, baseTs, dv.cs) // prevBeacon
	if err != nil {
		return xerrors.Errorf("failed to get latest beacon entry: %w", err)
	}

	// fast checks first
	baseHight, err := baseTs.Height()
	if err != nil {
		return xerrors.Errorf("failed to get base tipset height: %w", err)
	}
	nulls := blk.Height - (baseHight + 1)
	if tgtTs := baseTs.MinTimestamp() + builtin.EpochDurationSeconds*uint64(nulls+1); blk.Timestamp != tgtTs {
		return xerrors.Errorf("block has wrong timestamp: %d != %d", blk.Timestamp, tgtTs)
	}

	now := uint64(dv.Now().Unix())
	if blk.Timestamp > now+AllowableClockDriftSecs {
		return xerrors.Errorf("block was from the future (now=%d, blk=%d): %w", now, blk.Timestamp, ErrTemporal)
	}
	if blk.Timestamp > now {
		log.Warn("Got block from the future, but within threshold", blk.Timestamp, dv.Now().Unix())
	}

	msgsCheck := async.Err(func() error {
		if err := dv.checkBlockMessages(ctx, blk, &baseTs); err != nil {
			return xerrors.Errorf("block had invalid messages: %w", err)
		}
		return nil
	})

	//minerCheck := async.Err(func() error {
	//	if err := syncer.minerIsValid(ctx, h.Miner, baseTs); err != nil {
	//		return xerrors.Errorf("minerIsValid failed: %w", err)
	//	}
	//	return nil
	//})

	baseFeeCheck := async.Err(func() error {
		baseFee, err := dv.ms.ComputeBaseFee(ctx, &baseTs)
		if err != nil {
			return xerrors.Errorf("computing base fee: %w", err)
		}

		if big.Cmp(baseFee, blk.ParentBaseFee) != 0 {
			return xerrors.Errorf("base fee doesn't match: %s (header) != %s (computed)",
				blk.ParentBaseFee, baseFee)
		}
		return nil
	})

	pweight, err := dv.cs.Weight(ctx, &baseTs)
	if err != nil {
		return xerrors.Errorf("getting parent weight: %w", err)
	}

	if big.Cmp(pweight, blk.ParentWeight) != 0 {
		return xerrors.Errorf("parrent weight different: %s (header) != %s (computed)",
			blk.ParentWeight, pweight)
	}

	if lbst != blk.StateRoot.Cid {
		b, err := dv.cs.GetBlock(ctx, blk.StateRoot.Cid)
		if err != nil {
			log.Error("failed to load messages for tipset during tipset state mismatch error: ", err)
		}else{
			secpMsgs, blsMsgs, err := dv.ms.LoadMessages(ctx, b.Messages.Cid)
			if err != nil {
				log.Error("failed to load messages for tipset during tipset state mismatch error: ", err)
			}else{
				for _, blsm := range blsMsgs {
					mm := blsm.VMMessage()
					log.Warnf("Message: from=%s to=%s method=%d params=%x", mm.From, mm.To, mm.Method, mm.Params)
				}

				for _, secm := range secpMsgs {
					mm := secm.VMMessage()
					log.Warnf("Message: from=%s to=%s method=%d params=%x", mm.From, mm.To, mm.Method, mm.Params)
				}
			}
		}

		return xerrors.Errorf("parent state root did not match computed state (%s != %s)", lbst, blk.StateRoot)
	}

	view, err := dv.cs.StateView(blk.Parents)
	if err != nil {
		return errors.Wrapf(err, "failed to load state at %v", blk.Parents)
	}

	mi, err := view.MinerInfo(ctx,blk.Miner)
	if err != nil {
		return xerrors.Errorf("Get MinerInfo failed: %w", err)
	}
	waddr := mi.Worker

	winnerCheck := async.Err(func() error {
		if blk.ElectionProof.WinCount < 1 {
			return xerrors.Errorf("block is not claiming to be a winner")
		}

		view, err := dv.cs.StateView(blk.Parents)
		if err != nil {
			return xerrors.Errorf("get state view failed: %w", err)
		}

		hp, err := view.MinerHasMinPower(ctx, blk.Miner)
		if err != nil {
			return xerrors.Errorf("determining if miner has min power failed: %w", err)
		}

		if !hp {
			return xerrors.New("block's miner does not meet minimum power threshold")
		}

		rBeacon := prevBeacon
		if len(blk.BeaconEntries) != 0 {
			rBeacon = blk.BeaconEntries[len(blk.BeaconEntries)-1]
		}
		buf := new(bytes.Buffer)
		if err := blk.Miner.MarshalCBOR(buf); err != nil {
			return xerrors.Errorf("failed to marshal miner address to cbor: %w", err)
		}

		vrfBase, err := cst.DrawRandomness(rBeacon.Data, crypto.DomainSeparationTag_ElectionProofProduction, blk.Height, buf.Bytes())
		if err != nil {
			return xerrors.Errorf("could not draw randomness: %w", err)
		}

		if err := VerifyElectionPoStVRF(ctx, waddr, vrfBase, blk.ElectionProof.VRFProof); err != nil {
			return xerrors.Errorf("validating block election proof failed: %w", err)
		}

		//slashed, err := stmgr.GetMinerSlashed(ctx, syncer.sm, baseTs, h.Miner)
		//if err != nil {
		//	return xerrors.Errorf("failed to check if block miner was slashed: %w", err)
		//}
		//
		//if slashed {
		//	return xerrors.Errorf("received block was from slashed or invalid miner")
		//}

		_, qaPower, err := view.MinerClaimedPower(ctx, blk.Miner)
		if err != nil {
			return xerrors.Errorf("get miner power failed: %w", err)
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
	})

	//blockSigCheck := async.Err(func() error {
	//	if err := sigs.CheckBlockSignature(ctx, blk, waddr); err != nil {
	//		return xerrors.Errorf("check block signature failed: %w", err)
	//	}
	//	return nil
	//})

	beaconValuesCheck := async.Err(func() error {
		if os.Getenv("LOTUS_IGNORE_DRAND") == "_yes_" {
			return nil
		}

		if err := beacon.ValidateBlockValues(dv.cs.BeaconSchedule(), blk, baseHight, prevBeacon); err != nil {
			return xerrors.Errorf("failed to validate blocks random beacon values: %w", err)
		}
		return nil
	})

	tktsCheck := async.Err(func() error {
		buf := new(bytes.Buffer)
		if err := blk.Miner.MarshalCBOR(buf); err != nil {
			return xerrors.Errorf("failed to marshal miner address to cbor: %w", err)
		}

		if blk.Height > beacon.UpgradeSmokeHeight {
			ticket, err := baseTs.MinTicket()
			if err != nil {
				return xerrors.Errorf("failed to get base ticket: %w", err)
			}
			buf.Write(ticket.VRFProof)
		}

		beaconBase := prevBeacon
		if len(blk.BeaconEntries) != 0 {
			beaconBase = blk.BeaconEntries[len(blk.BeaconEntries)-1]
		}

		vrfBase, err := cst.DrawRandomness(beaconBase.Data, crypto.DomainSeparationTag_TicketProduction, blk.Height-TicketRandomnessLookback, buf.Bytes())
		if err != nil {
			return xerrors.Errorf("failed to compute vrf base for ticket: %w", err)
		}

		err = VerifyElectionPoStVRF(ctx, waddr, vrfBase, blk.Ticket.VRFProof)
		if err != nil {
			return xerrors.Errorf("validating block tickets failed: %w", err)
		}
		return nil
	})

	wproofCheck := async.Err(func() error {
		if err := dv.VerifyWinningPoStProof(ctx, blk, *prevBeacon, lbst, waddr); err != nil {
			return xerrors.Errorf("invalid election post: %w", err)
		}
		return nil
	})

	await := []async.ErrorFuture{
		// minerCheck,
		tktsCheck,
		//blockSigCheck,
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

	//
	//if err := syncer.store.MarkBlockAsValidated(ctx, b.Cid()); err != nil {
	//	return xerrors.Errorf("caching block validation %s: %w", b.Cid(), err)
	//}

	return nil
}

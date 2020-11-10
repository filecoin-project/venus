package poster

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	xerrors "github.com/pkg/errors"
	"github.com/raulk/clock"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/dline"
	miner0 "github.com/filecoin-project/specs-actors/actors/builtin/miner"
	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/venus/internal/app/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/venus/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/message"
	actors "github.com/filecoin-project/venus/internal/pkg/specactors"
	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin/miner"
	"github.com/filecoin-project/venus/internal/pkg/specactors/policy"
	appstate "github.com/filecoin-project/venus/internal/pkg/state"
	"github.com/filecoin-project/venus/internal/pkg/types"
	sectorstorage "github.com/filecoin-project/venus/vendors/sector-storage"
)

var log = logging.Logger("poster")

// Epochs
const MessageConfidence = uint64(5)

// Clock is the global clock for the system. In standard builds,
// we use a real-time clock, which maps to the `time` package.
//
// Tests that need control of time can replace this variable with
// clock.NewMock(). Always use real time for socket/stream deadlines.
var Clock = clock.New()

// Poster listens for changes to the chain head and generates and submits a PoSt if one is required.
type Poster struct {
	postMutex      sync.Mutex
	postCancel     context.CancelFunc
	scheduleCancel context.CancelFunc
	challenge      abi.Randomness

	minerAddr   address.Address
	workerAddr  address.Address
	outbox      *message.Outbox
	mgr         sectorstorage.SectorManager
	chain       *cst.ChainStateReadWriter
	stateViewer *appstate.Viewer
	waiter      *msg.Waiter

	//todo set by module
	proofType    abi.RegisteredPoStProof
	faultTracker sectorstorage.FaultTracker
}

// NewPoster creates a Poster struct
func NewPoster(
	minerAddr address.Address,
	outbox *message.Outbox,
	mgr sectorstorage.SectorManager,
	chain *cst.ChainStateReadWriter,
	stateViewer *appstate.Viewer,
	proofType abi.RegisteredPoStProof,
	faultTracker sectorstorage.FaultTracker,
	waiter *msg.Waiter) *Poster {

	//workerAddr todo get workerAddr from chain
	return &Poster{
		minerAddr:    minerAddr,
		outbox:       outbox,
		mgr:          mgr,
		chain:        chain,
		stateViewer:  stateViewer,
		waiter:       waiter,
		challenge:    abi.Randomness{},
		proofType:    proofType,
		faultTracker: faultTracker,
	}
}

// HandleNewHead submits a new chain head for possible fallback PoSt.
func (p *Poster) HandleNewHead(ctx context.Context, newHead *block.TipSet) error {
	return p.startPoStIfNeeded(ctx, newHead)
}

// StopPoSting stops the posting scheduler if running and any outstanding PoSts.
func (p *Poster) StopPoSting() {
	p.postMutex.Lock()
	defer p.postMutex.Unlock()

	if p.scheduleCancel != nil {
		p.postCancel()

		p.scheduleCancel()
		p.scheduleCancel = nil
	}
}

func (p *Poster) startPoStIfNeeded(ctx context.Context, newHead *block.TipSet) error {
	p.postMutex.Lock()
	defer p.postMutex.Unlock()

	if p.postCancel != nil {
		// already posting
		return nil
	}

	tipsetHeight, err := newHead.Height()
	if err != nil {
		return err
	}

	root, err := p.chain.GetTipSetStateRoot(ctx, newHead.Key())
	if err != nil {
		return err
	}

	stateView := p.stateViewer.StateView(root)
	diInfo, err := stateView.StateMinerProvingDeadline(ctx, p.minerAddr, newHead)
	if err != nil {
		return err
	}

	// exit if we haven't yet hit the deadline
	if tipsetHeight < diInfo.Open {
		return nil
	}

	randomness, err := p.getChallenge(ctx, newHead.Key(), diInfo.Challenge)
	if err != nil {
		return err
	}

	// If we have not already seen this randomness, either the deadline has changed
	// or the chain as reorged to a point prior to the challenge. Either way,
	// it is time to start a new PoSt.
	if bytes.Equal(p.challenge, randomness) {
		return nil
	}
	p.challenge = randomness

	// stop existing PoSt, if one exists
	p.cancelPoSt()

	ctx, p.postCancel = context.WithCancel(ctx)
	go p.doPoSt(ctx, stateView, diInfo, newHead)

	return nil
}

func (p *Poster) doPoSt(ctx context.Context, stateView *appstate.View, di *dline.Info, newHead *block.TipSet) {
	defer p.safeCancelPoSt()
	ctx, span := trace.StartSpan(ctx, "storage.runPost")
	defer span.End()

	go func() {
		// TODO: extract from runPost, run on fault cutoff boundaries

		// check faults / recoveries for the *next* deadline. It's already too
		// late to declare them for this deadline
		declDeadline := (di.Index + 2) % miner.WPoStPeriodDeadlines

		deadline, err := stateView.StateMinerDeadlineForIdx(context.TODO(), p.minerAddr, declDeadline, newHead.Key())
		if err != nil {
			log.Errorf("getting deadline error: %v", err)
			return
		}

		partitions := make([]miner.Partition, 0)
		if err := deadline.ForEachPartition(func(idx uint64, part miner.Partition) error {
			partitions = append(partitions, part)
			return nil
		}); err != nil {
			log.Errorf("getting partitions error: %v", err)
			return
		}

		var (
			sigmsg     cid.Cid
			recoveries []miner.RecoveryDeclaration
			faults     []miner.FaultDeclaration
		)

		if recoveries, sigmsg, err = p.checkNextRecoveries(context.TODO(), declDeadline, partitions); err != nil {
			// TODO: This is potentially quite bad, but not even trying to post when this fails is objectively worse
			log.Errorf("checking sector recoveries: %v", err)
		}
		log.Info("checking sector recoveries success", len(recoveries), " msgcid:", sigmsg)

		if faults, sigmsg, err = p.checkNextFaults(context.TODO(), declDeadline, partitions); err != nil {
			// TODO: This is also potentially really bad, but we try to post anyways
			log.Errorf("checking sector faults: %v", err)
		}
		log.Info("checking sector faults success", len(faults), " msgcid:", sigmsg)
	}()

	buf := new(bytes.Buffer)
	if err := p.minerAddr.MarshalCBOR(buf); err != nil {
		log.Errorf("failed to marshal address to cbor: %w", err)
		return
	}

	rand, err := p.chain.ChainGetRandomnessFromBeacon(ctx, newHead.Key(), acrypto.DomainSeparationTag_WindowedPoStChallengeSeed, di.Challenge, buf.Bytes())
	if err != nil {
		log.Errorf("failed to get chain randomness for window post (ts=%d; deadline=%d): %w", newHead.EnsureHeight(), di, err)
		return
	}

	// Get the partitions for the given deadline
	deadline, err := stateView.StateMinerDeadlineForIdx(context.TODO(), p.minerAddr, di.Index, newHead.Key())
	if err != nil {
		log.Errorf("getting deadline error: %v", err)
		return
	}

	partitions := make([]miner.Partition, 0)
	if err := deadline.ForEachPartition(func(idx uint64, part miner.Partition) error {
		partitions = append(partitions, part)
		return nil
	}); err != nil {
		log.Errorf("getting partitions error: %v", err)
		return
	}

	// Split partitions into batches, so as not to exceed the number of sectors
	// allowed in a single message
	partitionBatches, err := p.batchPartitions(partitions)
	if err != nil {
		log.Error(err)
		return
	}

	// Generate proofs in batches
	posts := make([]miner.SubmitWindowedPoStParams, 0, len(partitionBatches))
	for batchIdx, batch := range partitionBatches {
		batchPartitionStartIdx := 0
		for _, batch := range partitionBatches[:batchIdx] {
			batchPartitionStartIdx += len(batch)
		}

		params := miner.SubmitWindowedPoStParams{
			Deadline:   di.Index,
			Partitions: make([]miner.PoStPartition, 0, len(batch)),
			Proofs:     nil,
		}

		skipCount := uint64(0)
		postSkipped := bitfield.New()
		var postOut []builtin.PoStProof
		somethingToProve := true

		for retries := 0; retries < 5; retries++ {
			var partitions []miner.PoStPartition
			var sinfos []builtin.SectorInfo
			for partIdx, partition := range batch {
				// TODO: Can do this in parallel
				toProve, err := partition.ActiveSectors()
				if err != nil {
					log.Errorf("getting active sectors: %w", err)
					return
				}

				recoveries, err := partition.RecoveringSectors()
				if err != nil {
					log.Errorf("get recoveries error: %w", err)
					return
				}
				toProve, err = bitfield.MergeBitFields(toProve, recoveries)
				if err != nil {
					log.Errorf("adding recoveries to set of sectors to prove: %w", err)
					return
				}

				good, err := p.checkSectors(ctx, toProve)
				if err != nil {
					log.Errorf("checking sectors to skip: %w", err)
					return
				}

				good, err = bitfield.SubtractBitField(good, postSkipped)
				if err != nil {
					log.Errorf("toProve - postSkipped: %w", err)
					return
				}

				skipped, err := bitfield.SubtractBitField(toProve, good)
				if err != nil {
					log.Errorf("toProve - good: %w", err)
					return
				}

				sc, err := skipped.Count()
				if err != nil {
					log.Errorf("getting skipped sector count: %w", err)
					return
				}

				skipCount += sc
				sectors, err := partition.AllSectors()
				if err != nil {
					log.Errorf("getting partition sectors error: %w", err)
					return
				}
				ssi, err := p.sectorsForProof(ctx, stateView, good, sectors, newHead)
				if err != nil {
					log.Errorf("getting sorted sector info: %w", err)
					return
				}

				if len(ssi) == 0 {
					continue
				}

				sinfos = append(sinfos, ssi...)
				partitions = append(partitions, miner.PoStPartition{
					Index:   uint64(batchPartitionStartIdx + partIdx),
					Skipped: skipped,
				})
			}

			if len(sinfos) == 0 {
				// nothing to prove for this batch
				somethingToProve = false
				break
			}

			// Generate proof
			height, _ := newHead.Height()
			log.Infow("running window post",
				"chain-random", rand,
				"deadline", di,
				"height", height,
				"skipped", skipCount)

			tsStart := Clock.Now()

			mid, err := address.IDFromAddress(p.minerAddr)
			if err != nil {
				log.Error(err)
				return
			}

			var ps []abi.SectorID
			postOut, ps, err = p.mgr.GenerateWindowPoSt(ctx, abi.ActorID(mid), sinfos, abi.PoStRandomness(rand))
			elapsed := time.Since(tsStart)

			log.Infow("computing window post", "batch", batchIdx, "elapsed", elapsed)

			if err == nil {
				// Proof generation successful, stop retrying
				params.Partitions = append(params.Partitions, partitions...)

				break
			}

			// Proof generation failed, so retry

			if len(ps) == 0 {
				log.Errorf("running window post failed: %w", err)
				return
			}

			log.Warnw("generate window post skipped sectors", "sectors", ps, "error", err, "try", retries)

			skipCount += uint64(len(ps))
			for _, sector := range ps {
				postSkipped.Set(uint64(sector.Number))
			}
		}

		// Nothing to prove for this batch, try the next batch
		if !somethingToProve {
			continue
		}

		if len(postOut) == 0 {
			log.Errorf("received no proofs back from generate window post")
			return
		}

		params.Proofs = postOut

		posts = append(posts, params)
	}

	// Compute randomness after generating proofs so as to reduce the impact
	// of chain reorgs (which change randomness)
	commEpoch := di.Open
	commRand, err := p.chain.SampleChainRandomness(ctx, newHead.Key(), acrypto.DomainSeparationTag_PoStChainCommit, commEpoch, nil)
	if err != nil {
		log.Errorf("failed to get chain randomness for window post (ts=%d; deadline=%d): %w", newHead.EnsureHeight(), commEpoch, err)
		return
	}

	for i := range posts {
		posts[i].ChainCommitEpoch = commEpoch
		posts[i].ChainCommitRand = commRand
	}

	err = p.sendPoSt(ctx, posts)
	if err != nil {
		log.Error("error sending window PoSt: ", err)
		return
	}
}

func (p *Poster) sendPoSt(ctx context.Context, proofs []miner.SubmitWindowedPoStParams) error {
	for _, winPostParams := range proofs {
		mcid, errCh, err := p.outbox.Send(
			ctx,
			p.workerAddr,
			p.minerAddr,
			types.ZeroAttoFIL,
			types.NewGasFeeCap(1),
			types.NewGasFeeCap(1), //todo add by force 费用估算
			types.NewGas(10000),
			true,
			miner.Methods.SubmitWindowedPoSt,
			winPostParams,
		)
		if err != nil {
			return err
		}
		if err := <-errCh; err != nil {
			return err
		}

		// wait until we see the post on chain at least once
		err = p.waiter.Wait(ctx, mcid, msg.DefaultMessageWaitLookback, func(_ *block.Block, _ *types.SignedMessage, recp *types.MessageReceipt) error {
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Poster) sectorsForProof(ctx context.Context, stateViewer *appstate.View, goodSectors, allSectors bitfield.BitField, ts *block.TipSet) ([]builtin.SectorInfo, error) {
	sset, err := stateViewer.StateMinerSectors(ctx, p.minerAddr, &goodSectors, ts.Key())
	if err != nil {
		return nil, err
	}

	if len(sset) == 0 {
		return nil, nil
	}

	substitute := builtin.SectorInfo{
		SectorNumber: sset[0].ID,
		SealedCID:    sset[0].Info.SealedCID,
		SealProof:    sset[0].Info.SealProof,
	}

	sectorByID := make(map[uint64]builtin.SectorInfo, len(sset))
	for _, sector := range sset {
		sectorByID[uint64(sector.ID)] = builtin.SectorInfo{
			SectorNumber: sector.ID,
			SealedCID:    sector.Info.SealedCID,
			SealProof:    sector.Info.SealProof,
		}
	}

	proofSectors := make([]builtin.SectorInfo, 0, len(sset))
	if err := allSectors.ForEach(func(sectorNo uint64) error {
		if info, found := sectorByID[sectorNo]; found {
			proofSectors = append(proofSectors, info)
		} else {
			proofSectors = append(proofSectors, substitute)
		}
		return nil
	}); err != nil {
		return nil, xerrors.Errorf("iterating partition sector bitmap: %w", err)
	}

	return proofSectors, nil
}

func (p *Poster) batchPartitions(partitions []miner.Partition) ([][]miner.Partition, error) {
	// Get the number of sectors allowed in a partition, for this builtin size
	sectorsPerPartition, err := policy.GetMaxPoStPartitions(p.proofType)
	if err != nil {
		return nil, xerrors.Errorf("getting sectors per partition: %w", err)
	}

	// We don't want to exceed the number of sectors allowed in a message.
	// So given the number of sectors in a partition, work out the number of
	// partitions that can be in a message without exceeding sectors per
	// message:
	// floor(number of sectors allowed in a message / sectors per partition)
	// eg:
	// max sectors per message  7:  ooooooo
	// sectors per partition    3:  ooo
	// partitions per message   2:  oooOOO
	//                              <1><2> (3rd doesn't fit)
	partitionsPerMsg := int(miner0.AddressedSectorsMax / sectorsPerPartition)

	// The number of messages will be:
	// ceiling(number of partitions / partitions per message)
	batchCount := len(partitions) / partitionsPerMsg
	if len(partitions)%partitionsPerMsg != 0 {
		batchCount++
	}

	// Split the partitions into batches
	batches := make([][]miner.Partition, 0, batchCount)
	for i := 0; i < len(partitions); i += partitionsPerMsg {
		end := i + partitionsPerMsg
		if end > len(partitions) {
			end = len(partitions)
		}
		batches = append(batches, partitions[i:end])
	}
	return batches, nil
}

func (p *Poster) checkNextRecoveries(ctx context.Context, dlIdx uint64, partitions []miner.Partition) ([]miner.RecoveryDeclaration, cid.Cid, error) {
	ctx, span := trace.StartSpan(ctx, "storage.checkNextRecoveries")
	defer span.End()

	faulty := uint64(0)
	params := &miner.DeclareFaultsRecoveredParams{
		Recoveries: []miner.RecoveryDeclaration{},
	}

	for partIdx, partition := range partitions {
		faults, err := partition.FaultySectors()
		if err != nil {
			return nil, cid.Undef, err
		}

		recoveries, err := partition.RecoveringSectors()
		if err != nil {
			return nil, cid.Undef, err
		}

		unrecovered, err := bitfield.SubtractBitField(faults, recoveries)
		if err != nil {
			return nil, cid.Undef, xerrors.Errorf("subtracting recovered set from fault set: %w", err)
		}

		uc, err := unrecovered.Count()
		if err != nil {
			return nil, cid.Undef, xerrors.Errorf("counting unrecovered sectors: %w", err)
		}

		if uc == 0 {
			continue
		}

		faulty += uc

		recovered, err := p.checkSectors(ctx, unrecovered)
		if err != nil {
			return nil, cid.Undef, xerrors.Errorf("checking unrecovered sectors: %w", err)
		}

		// if all sectors failed to recover, don't declare recoveries
		recoveredCount, err := recovered.Count()
		if err != nil {
			return nil, cid.Undef, xerrors.Errorf("counting recovered sectors: %w", err)
		}

		if recoveredCount == 0 {
			continue
		}

		params.Recoveries = append(params.Recoveries, miner.RecoveryDeclaration{
			Deadline:  dlIdx,
			Partition: uint64(partIdx),
			Sectors:   recovered,
		})
	}

	recoveries := params.Recoveries
	if len(recoveries) == 0 {
		if faulty != 0 {
			log.Warnw("No recoveries to declare", "deadline", dlIdx, "faulty", faulty)
		}

		return recoveries, cid.Undef, nil
	}

	enc, aerr := actors.SerializeParams(params)
	if aerr != nil {
		return recoveries, cid.Undef, xerrors.Errorf("could not serialize declare recoveries parameters: %w", aerr)
	}

	msg := &types.UnsignedMessage{
		To:     p.minerAddr,
		From:   p.workerAddr,
		Method: miner.Methods.DeclareFaultsRecovered,
		Params: enc,
		Value:  big.NewInt(0),
	}
	spec := &types.MessageSendSpec{MaxFee: abi.TokenAmount(big.NewInt(100000000))} //todo add by force s.feeCfg.MaxWindowPoStGasFee)}
	p.setSender(ctx, msg, spec)

	mcid, _, err := p.outbox.UnSignedSend(ctx, *msg)
	if err != nil {
		return recoveries, mcid, xerrors.Errorf("pushing message to mpool: %w", err)
	}

	log.Warnw("declare faults recovered Message CID", "cid", mcid)

	err = p.waiter.Wait(context.TODO(), mcid, MessageConfidence, func(b *block.Block, signedMessage *types.SignedMessage, receipt *types.MessageReceipt) error {
		if receipt.ExitCode != 0 {
			return xerrors.Errorf("declare faults recovered wait non-0 exit code: %d", receipt.ExitCode)
		}
		return nil
	})
	if err != nil {
		return recoveries, mcid, xerrors.Errorf("declare faults recovered wait error: %w", err)
	}

	return recoveries, mcid, nil
}

func (p *Poster) checkNextFaults(ctx context.Context, dlIdx uint64, partitions []miner.Partition) ([]miner.FaultDeclaration, cid.Cid, error) {
	ctx, span := trace.StartSpan(ctx, "storage.checkNextFaults")
	defer span.End()

	bad := uint64(0)
	params := &miner.DeclareFaultsParams{
		Faults: []miner.FaultDeclaration{},
	}

	for partIdx, partition := range partitions {
		toCheck, err := partition.ActiveSectors()
		if err != nil {
			return nil, cid.Undef, xerrors.Errorf("getting active sectors: %w", err)
		}

		good, err := p.checkSectors(ctx, toCheck)
		if err != nil {
			return nil, cid.Undef, xerrors.Errorf("checking sectors: %w", err)
		}

		faulty, err := bitfield.SubtractBitField(toCheck, good)
		if err != nil {
			return nil, cid.Undef, xerrors.Errorf("calculating faulty sector set: %w", err)
		}

		c, err := faulty.Count()
		if err != nil {
			return nil, cid.Undef, xerrors.Errorf("counting faulty sectors: %w", err)
		}

		if c == 0 {
			continue
		}

		bad += c

		params.Faults = append(params.Faults, miner.FaultDeclaration{
			Deadline:  dlIdx,
			Partition: uint64(partIdx),
			Sectors:   faulty,
		})
	}

	faults := params.Faults
	if len(faults) == 0 {
		return faults, cid.Undef, nil
	}

	log.Errorw("DETECTED FAULTY SECTORS, declaring faults", "count", bad)

	enc, aerr := actors.SerializeParams(params)
	if aerr != nil {
		return faults, cid.Undef, xerrors.Errorf("could not serialize declare faults parameters: %w", aerr)
	}

	msg := &types.UnsignedMessage{
		To:     p.minerAddr,
		From:   p.workerAddr,
		Method: miner.Methods.DeclareFaults,
		Params: enc,
		Value:  big.NewInt(0), // TODO: Is there a fee?
	}
	spec := &types.MessageSendSpec{MaxFee: abi.TokenAmount(big.NewInt(100000000))} // s.feeCfg.MaxWindowPoStGasFee)}
	p.setSender(ctx, msg, spec)

	mcid, _, err := p.outbox.UnSignedSend(ctx, *msg)
	if err != nil {
		return faults, mcid, xerrors.Errorf("pushing message to mpool: %w", err)
	}

	log.Warnw("declare faults Message CID", "cid", mcid)

	err = p.waiter.Wait(context.TODO(), mcid, MessageConfidence, func(b *block.Block, signedMessage *types.SignedMessage, receipt *types.MessageReceipt) error {
		if receipt.ExitCode != 0 {
			return xerrors.Errorf("declare faults wait non-0 exit code: %d", receipt.ExitCode)
		}
		return nil
	})

	if err != nil {
		return faults, mcid, xerrors.Errorf("declare faults wait error: %w", err)
	}
	return faults, mcid, nil
}

func (p *Poster) checkSectors(ctx context.Context, check bitfield.BitField) (bitfield.BitField, error) {
	spt, err := p.proofType.RegisteredSealProof()
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("getting seal proof type: %w", err)
	}

	mid, err := address.IDFromAddress(p.minerAddr)
	if err != nil {
		return bitfield.BitField{}, err
	}

	sectors := make(map[abi.SectorID]struct{})
	var tocheck []abi.SectorID
	err = check.ForEach(func(snum uint64) error {
		s := abi.SectorID{
			Miner:  abi.ActorID(mid),
			Number: abi.SectorNumber(snum),
		}

		tocheck = append(tocheck, s)
		sectors[s] = struct{}{}
		return nil
	})
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("iterating over bitfield: %w", err)
	}

	bad, err := p.faultTracker.CheckProvable(ctx, spt, tocheck)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("checking provable sectors: %w", err)
	}
	for _, id := range bad {
		delete(sectors, id)
	}

	log.Warnw("Checked sectors", "checked", len(tocheck), "good", len(sectors))

	sbf := bitfield.New()
	for s := range sectors {
		sbf.Set(uint64(s.Number))
	}

	return sbf, nil
}

func (p *Poster) setSender(ctx context.Context, msg *types.UnsignedMessage, spec *types.MessageSendSpec) {
	//todo use control address can complete it later
	/*mi, err := s.api.StateMinerInfo(ctx, s.actor, types.EmptyTSK)
	if err != nil {
		log.Errorw("error getting miner info", "error", err)

		// better than just failing
		msg.From = s.worker
		return
	}

	gm, err := s.api.GasEstimateMessageGas(ctx, msg, spec, types.EmptyTSK)
	if err != nil {
		log.Errorw("estimating gas", "error", err)
		msg.From = s.worker
		return
	}
	*msg = *gm

	minFunds := big.Add(msg.RequiredFunds(), msg.Value)

	pa, err := AddressFor(ctx, s.api, mi, PoStAddr, minFunds)
	if err != nil {
		log.Errorw("error selecting address for window post", "error", err)
		msg.From = s.worker
		return
	}

	msg.From = pa*/
}

func (p *Poster) getChallenge(ctx context.Context, head block.TipSetKey, at abi.ChainEpoch) (abi.Randomness, error) {
	buf := new(bytes.Buffer)
	err := p.minerAddr.MarshalCBOR(buf)
	if err != nil {
		return nil, err
	}

	return p.chain.SampleChainRandomness(ctx, head, acrypto.DomainSeparationTag_WindowedPoStChallengeSeed, at, buf.Bytes())
}

func (p *Poster) safeCancelPoSt() {
	p.postMutex.Lock()
	defer p.postMutex.Unlock()

	p.cancelPoSt()
}

func (p *Poster) cancelPoSt() {
	if p.postCancel != nil {
		p.postCancel()
		p.postCancel = nil
	}
}

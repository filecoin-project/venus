package mining

// The Worker Mines on Input received from a Scheduler.  The Worker is
// responsible for generating the necessary proofs, checking for success,
// generating new blocks, and forwarding them out to the wider node.

import (
	"context"
	"time"

	address "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	cid "github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/drand"
	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

var log = logging.Logger("mining")

// Output is the result of a single mining attempt. It may have a new block header (with included messages),
// an error, or zero-value fields indicating that the miner did not win the necessary election.
type Output struct {
	Header       *block.Block
	BLSMessages  []*types.SignedMessage
	SECPMessages []*types.SignedMessage
	Err          error
}

func NewOutput(b *block.Block, BLSMessages, SECPMessages []*types.SignedMessage) Output {
	return Output{Header: b, BLSMessages: BLSMessages, SECPMessages: SECPMessages, Err: nil}
}

func NewOutputEmpty() Output {
	return Output{}
}

func NewOutputErr(e error) Output {
	return Output{Err: e}
}

// Worker is the interface called by the Scheduler to run the mining work being
// scheduled.
type Worker interface {
	Mine(runCtx context.Context, base block.TipSet, nullBlkCount uint64, outCh chan<- Output) bool
}

// GetStateTree is a function that gets the aggregate state tree of a TipSet. It's
// its own function to facilitate testing.
type GetStateTree func(context.Context, block.TipSetKey) (state.Tree, error)

// GetWeight is a function that calculates the weight of a TipSet.  Weight is
// expressed as two uint64s comprising a rational number.
type GetWeight func(context.Context, block.TipSet) (big.Int, error)

// MessageSource provides message candidates for mining into blocks
type MessageSource interface {
	// Pending returns a slice of un-mined messages.
	Pending() []*types.SignedMessage
	// Remove removes a message from the source permanently
	Remove(message cid.Cid)
}

// A MessageApplier processes all the messages in a message pool.
type MessageApplier interface {
	// Dragons: add something back or remove
}

type workerPorcelainAPI interface {
	consensus.ChainRandomness
	BlockTime() time.Duration
	PowerStateView(baseKey block.TipSetKey) (consensus.PowerStateView, error)
	FaultsStateView(baseKey block.TipSetKey) (consensus.FaultStateView, error)
}

type electionUtil interface {
	GenerateElectionProof(ctx context.Context, entry *drand.Entry, epoch abi.ChainEpoch, miner address.Address, worker address.Address, signer types.Signer) (crypto.VRFPi, error)
	IsWinner(challengeTicket []byte, minerPower, networkPower abi.StoragePower) bool
	GenerateWinningPoSt(ctx context.Context, allSectorInfos []abi.SectorInfo, entry *drand.Entry, epoch abi.ChainEpoch, ep postgenerator.PoStGenerator, maddr address.Address) ([]block.PoStProof, error)
}

// ticketGenerator creates tickets.
type ticketGenerator interface {
	MakeTicket(ctx context.Context, base block.TipSetKey, epoch abi.ChainEpoch, miner address.Address, entry *drand.Entry, newPeriod bool, worker address.Address, signer types.Signer) (block.Ticket, error)
}

type tipSetMetadata interface {
	GetTipSetStateRoot(key block.TipSetKey) (cid.Cid, error)
	GetTipSetReceiptsRoot(key block.TipSetKey) (cid.Cid, error)
}

type messageMessageQualifier interface {
	PenaltyCheck(ctx context.Context, msg *types.UnsignedMessage) error
}

// DefaultWorker runs a mining job.
type DefaultWorker struct {
	api workerPorcelainAPI

	minerAddr      address.Address
	minerOwnerAddr address.Address
	workerSigner   types.Signer

	tsMetadata     tipSetMetadata
	getStateTree   GetStateTree
	getWeight      GetWeight
	election       electionUtil
	ticketGen      ticketGenerator
	messageSource  MessageSource
	penaltyChecker messageMessageQualifier
	messageStore   chain.MessageWriter // nolint: structcheck
	blockstore     blockstore.Blockstore
	clock          clock.ChainEpochClock
	poster         postgenerator.PoStGenerator
	chainState     chain.TipSetProvider
	drand          drand.IFace
}

// WorkerParameters use for NewDefaultWorker parameters
type WorkerParameters struct {
	API workerPorcelainAPI

	MinerAddr      address.Address
	MinerOwnerAddr address.Address
	WorkerSigner   types.Signer

	// consensus things
	TipSetMetadata   tipSetMetadata
	GetStateTree     GetStateTree
	MessageQualifier messageMessageQualifier
	GetWeight        GetWeight
	Election         electionUtil
	TicketGen        ticketGenerator
	Drand            drand.IFace

	// core filecoin things
	MessageSource MessageSource
	MessageStore  chain.MessageWriter
	Blockstore    blockstore.Blockstore
	Clock         clock.ChainEpochClock
	Poster        postgenerator.PoStGenerator
	ChainState    chain.TipSetProvider
}

// NewDefaultWorker instantiates a new Worker.
func NewDefaultWorker(parameters WorkerParameters) *DefaultWorker {
	return &DefaultWorker{
		api:            parameters.API,
		getStateTree:   parameters.GetStateTree,
		getWeight:      parameters.GetWeight,
		messageSource:  parameters.MessageSource,
		messageStore:   parameters.MessageStore,
		penaltyChecker: parameters.MessageQualifier,
		blockstore:     parameters.Blockstore,
		minerAddr:      parameters.MinerAddr,
		minerOwnerAddr: parameters.MinerOwnerAddr,
		workerSigner:   parameters.WorkerSigner,
		election:       parameters.Election,
		ticketGen:      parameters.TicketGen,
		tsMetadata:     parameters.TipSetMetadata,
		clock:          parameters.Clock,
		poster:         parameters.Poster,
		chainState:     parameters.ChainState,
		drand:          parameters.Drand,
	}
}

// Mine implements the DefaultWorkers main mining function..
// The returned bool indicates if this miner created a new block or not.
func (w *DefaultWorker) Mine(ctx context.Context, base block.TipSet, nullBlkCount uint64, outCh chan<- Output) (won bool) {
	log.Info("Worker.Mine")
	if !base.Defined() {
		log.Warn("Worker.Mine returning because it can't mine on an empty tipset")
		outCh <- NewOutputErr(errors.New("bad input tipset with no blocks sent to Mine()"))
		return
	}
	baseEpoch, err := base.Height()
	if err != nil {
		log.Warnf("Worker.Mine couldn't read base height %s", err)
		outCh <- NewOutputErr(err)
		return
	}
	currEpoch := baseEpoch + abi.ChainEpoch(1) + abi.ChainEpoch(nullBlkCount)

	log.Debugf("Mining on tipset %s, at epoch %d with %d null blocks.", base.String(), baseEpoch, nullBlkCount)
	if ctx.Err() != nil {
		log.Warnf("Worker.Mine returning with ctx error %s", ctx.Err().Error())
		return
	}

	// Read uncached worker address
	keyView, err := w.api.PowerStateView(base.Key())
	if err != nil {
		outCh <- NewOutputErr(err)
		return
	}
	_, workerAddr, err := keyView.MinerControlAddresses(ctx, w.minerAddr)
	if err != nil {
		outCh <- NewOutputErr(err)
		return
	}

	// Look-back for the election ticket.
	// The parameter is interpreted as: lookback=1 means parent tipset. Subtract one here because the base from
	// which the lookback is counted is already the parent, rather than "current" tipset.
	// The sampling code will handle this underflowing past the genesis.
	lookbackEpoch := currEpoch - miner.ElectionLookback

	workerSignerAddr, err := keyView.AccountSignerAddress(ctx, workerAddr)
	if err != nil {
		outCh <- NewOutputErr(err)
		return
	}

	drandEntries, err := w.drandEntriesForEpoch(ctx, base, nullBlkCount)
	if err != nil {
		log.Errorf("Worker.Mine failed to collect drand entries for block %s", err)
		outCh <- NewOutputErr(err)
		return
	}

	// Determine if we've won election
	electionEntry, err := w.electionEntry(ctx, base, drandEntries)
	if err != nil {
		log.Errorf("Worker.Mine failed to calculate drand entry for election randomness %s", err)
		outCh <- NewOutputErr(err)
		return
	}

	newPeriod := len(drandEntries) > 0
	nextTicket, err := w.ticketGen.MakeTicket(ctx, base.Key(), lookbackEpoch, w.minerAddr, electionEntry, newPeriod, workerSignerAddr, w.workerSigner)
	if err != nil {
		log.Warnf("Worker.Mine couldn't generate next ticket %s", err)
		outCh <- NewOutputErr(err)
		return
	}

	electionVRFProof, err := w.election.GenerateElectionProof(ctx, electionEntry, currEpoch, w.minerAddr, workerSignerAddr, w.workerSigner)
	if err != nil {
		log.Errorf("Worker.Mine failed to generate electionVRFProof %s", err)
	}
	electionVRFDigest := electionVRFProof.Digest()
	electionPowerAncestor, err := w.lookbackTipset(ctx, base, nullBlkCount, consensus.ElectionPowerTableLookback)
	if err != nil {
		log.Errorf("Worker.Mine couldn't get ancestor tipset: %s", err.Error())
		outCh <- NewOutputErr(err)
		return
	}
	electionPowerTable, err := w.getPowerTable(electionPowerAncestor.Key(), base.Key())
	if err != nil {
		log.Errorf("Worker.Mine couldn't get snapshot for tipset: %s", err.Error())
		outCh <- NewOutputErr(err)
		return
	}
	networkPower, err := electionPowerTable.NetworkTotalPower(ctx)
	if err != nil {
		log.Errorf("failed to get network power: %s", err)
		outCh <- NewOutputErr(err)
		return
	}
	minerPower, err := electionPowerTable.MinerClaimedPower(ctx, w.minerAddr)
	if err != nil {
		log.Errorf("failed to get power claim for miner: %s", err)
		outCh <- NewOutputErr(err)
		return
	}
	wins := w.election.IsWinner(electionVRFDigest[:], minerPower, networkPower)
	if !wins {
		// no winners we are done
		won = false
		return
	}

	// we have a winning block
	sectorSetAncestor, err := w.lookbackTipset(ctx, base, nullBlkCount, consensus.WinningPoStSectorSetLookback)
	if err != nil {
		log.Errorf("Worker.Mine couldn't get ancestor tipset: %s", err.Error())
		outCh <- NewOutputErr(err)
		return
	}
	winningPoStSectorSetView, err := w.getPowerTable(sectorSetAncestor.Key(), base.Key())
	if err != nil {
		log.Errorf("Worker.Mine couldn't get snapshot for tipset: %s", err.Error())
		outCh <- NewOutputErr(err)
		return
	}
	sortedSectorInfos, err := winningPoStSectorSetView.SortedSectorInfos(ctx, w.minerAddr)
	if err != nil {
		log.Warnf("Worker.Mine failed to get ssi for %s", w.minerAddr)
		outCh <- NewOutputErr(err)
		return
	}

	posts, err := w.election.GenerateWinningPoSt(ctx, sortedSectorInfos, electionEntry, currEpoch, w.poster, w.minerAddr)
	if err != nil {
		log.Warnf("Worker.Mine failed to generate post")
		outCh <- NewOutputErr(err)
		return
	}

	next := w.Generate(ctx, base, nextTicket, electionVRFProof, abi.ChainEpoch(nullBlkCount), posts, drandEntries)
	if next.Err == nil {
		log.Debugf("Worker.Mine generates new winning block! %s", next.Header.Cid().String())
	}
	outCh <- next
	won = true
	return
}

func (w *DefaultWorker) getPowerTable(powerKey, faultsKey block.TipSetKey) (consensus.PowerTableView, error) {
	powerView, err := w.api.PowerStateView(powerKey)
	if err != nil {
		return consensus.PowerTableView{}, err
	}
	faultsView, err := w.api.FaultsStateView(faultsKey)
	if err != nil {
		return consensus.PowerTableView{}, err
	}
	return consensus.NewPowerTableView(powerView, faultsView), nil
}

func (w *DefaultWorker) lookbackTipset(ctx context.Context, base block.TipSet, nullBlkCount uint64, lookback uint64) (block.TipSet, error) {
	if lookback <= nullBlkCount+1 { // new block looks back to base
		return base, nil
	}
	baseHeight, err := base.Height()
	if err != nil {
		return block.UndefTipSet, err
	}
	targetEpoch := abi.ChainEpoch(uint64(baseHeight) + 1 + nullBlkCount - lookback)

	return chain.FindTipsetAtEpoch(ctx, base, targetEpoch, w.chainState)
}

// drandEntriesForEpoch returns the array of drand entries that should be
// included in the next block.  The return value maay be nil.
func (w *DefaultWorker) drandEntriesForEpoch(ctx context.Context, base block.TipSet, nullBlkCount uint64) ([]*drand.Entry, error) {
	baseHeight, err := base.Height()
	if err != nil {
		return nil, err
	}
	// Special case genesis
	var rounds []drand.Round
	lastTargetEpoch := abi.ChainEpoch(uint64(baseHeight) + nullBlkCount + 1 - consensus.DRANDEpochLookback)
	if baseHeight == abi.ChainEpoch(0) {
		// no latest entry, targetEpoch undefined as its before genesis

		// There should be a first genesis drand round from time before genesis
		// and then we grab everything between this round and genesis time
		startTime := w.drand.StartTimeOfRound(w.drand.FirstFilecoinRound())
		endTime := w.clock.StartTimeOfEpoch(lastTargetEpoch + 1)
		rounds = w.drand.RoundsInInterval(startTime, endTime)
	} else {
		latestEntry, err := chain.FindLatestDRAND(ctx, base, w.chainState)
		if err != nil {
			return nil, err
		}

		startTime := w.drand.StartTimeOfRound(latestEntry.Round)
		// end of interval is beginning of next epoch after lastTargetEpoch so
		//  we add 1 to lastTargetEpoch
		endTime := w.clock.StartTimeOfEpoch(lastTargetEpoch + 1)
		rounds = w.drand.RoundsInInterval(startTime, endTime)
		// first round is round of latestEntry so omit the 0th round
		rounds = rounds[1:]
	}

	entries := make([]*drand.Entry, len(rounds))
	for i, round := range rounds {
		entries[i], err = w.drand.ReadEntry(ctx, round)
		if err != nil {
			return nil, err
		}
	}
	return entries, nil
}

func (w *DefaultWorker) electionEntry(ctx context.Context, base block.TipSet, drandEntriesInBlock []*drand.Entry) (*drand.Entry, error) {
	numEntries := len(drandEntriesInBlock)
	if numEntries > 0 {
		return drandEntriesInBlock[numEntries-1], nil
	}

	return chain.FindLatestDRAND(ctx, base, w.chainState)
}

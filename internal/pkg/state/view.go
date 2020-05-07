package state

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/sector-storage/ffiwrapper"

	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/account"
	notinit "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	paychActor "github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/filecoin-project/specs-actors/actors/builtin/power"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
)

// Viewer builds state views from state root CIDs.
type Viewer struct {
	ipldStore cbor.IpldStore
}

// NewViewer creates a new state
func NewViewer(store cbor.IpldStore) *Viewer {
	return &Viewer{store}
}

// StateView returns a new state view.
func (c *Viewer) StateView(root cid.Cid) *View {
	return NewView(c.ipldStore, root)
}

// View is a read-only interface to a snapshot of application-level actor state.
// This object interprets the actor state, abstracting the concrete on-chain structures so as to
// hide the complications of protocol versions.
// Exported methods on this type avoid exposing concrete state structures (which may be subject to versioning)
// where possible.
type View struct {
	ipldStore cbor.IpldStore
	root      cid.Cid
}

// NewView creates a new state view
func NewView(store cbor.IpldStore, root cid.Cid) *View {
	return &View{
		ipldStore: store,
		root:      root,
	}
}

// InitNetworkName Returns the network name from the init actor state.
func (v *View) InitNetworkName(ctx context.Context) (string, error) {
	initState, err := v.loadInitActor(ctx)
	if err != nil {
		return "", err
	}
	return initState.NetworkName, nil
}

// InitResolveAddress Returns ID address if public key address is given.
func (v *View) InitResolveAddress(ctx context.Context, a addr.Address) (addr.Address, error) {
	if a.Protocol() == addr.ID {
		return a, nil
	}

	initState, err := v.loadInitActor(ctx)
	if err != nil {
		return addr.Undef, err
	}

	state := &notinit.State{
		AddressMap: initState.AddressMap,
	}
	return state.ResolveAddress(StoreFromCbor(ctx, v.ipldStore), a)
}

// Returns public key address if id address is given
func (v *View) AccountSignerAddress(ctx context.Context, a addr.Address) (addr.Address, error) {
	if a.Protocol() == addr.SECP256K1 || a.Protocol() == addr.BLS {
		return a, nil
	}

	accountActorState, err := v.loadAccountActor(ctx, a)
	if err != nil {
		return addr.Undef, err
	}

	return accountActorState.Address, nil
}

// MinerControlAddresses returns the owner and worker addresses for a miner actor
func (v *View) MinerControlAddresses(ctx context.Context, maddr addr.Address) (owner, worker addr.Address, err error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return addr.Undef, addr.Undef, err
	}
	return minerState.Info.Owner, minerState.Info.Worker, nil
}

// MinerPeerID returns the PeerID for a miner actor
func (v *View) MinerPeerID(ctx context.Context, maddr addr.Address) (peer.ID, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return "", err
	}
	return minerState.Info.PeerId, nil
}

type MinerSectorConfiguration struct {
	SealProofType              abi.RegisteredProof
	SectorSize                 abi.SectorSize
	WindowPoStPartitionSectors uint64
}

// MinerSectorConfiguration returns the sector size for a miner actor
func (v *View) MinerSectorConfiguration(ctx context.Context, maddr addr.Address) (*MinerSectorConfiguration, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return nil, err
	}
	return &MinerSectorConfiguration{
		SealProofType:              minerState.Info.SealProofType,
		SectorSize:                 minerState.Info.SectorSize,
		WindowPoStPartitionSectors: minerState.Info.WindowPoStPartitionSectors,
	}, nil
}

// MinerSectorCount counts all the on-chain sectors
func (v *View) MinerSectorCount(ctx context.Context, maddr addr.Address) (int, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return 0, err
	}
	count := 0
	var sector miner.SectorOnChainInfo
	sectors, err := v.asArray(ctx, minerState.Sectors)
	if err != nil {
		return 0, err
	}

	err = sectors.ForEach(&sector, func(_ int64) error {
		count++
		return nil
	})
	return count, err
}

// MinerDeadlineInfo returns information relevant to the current proving deadline
func (v *View) MinerDeadlineInfo(ctx context.Context, maddr addr.Address, epoch abi.ChainEpoch) (index uint64, open, close, challenge abi.ChainEpoch, _ error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	deadlineInfo := minerState.DeadlineInfo(epoch)
	return deadlineInfo.Index, deadlineInfo.Open, deadlineInfo.Close, deadlineInfo.Challenge, nil
}

// MinerPartitionIndexesForDeadline returns all partitions that need to be proven in the proving period deadline for the given epoch
func (v *View) MinerPartitionIndicesForDeadline(ctx context.Context, maddr addr.Address, deadlineIndex uint64) ([]uint64, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return nil, err
	}

	deadlines, err := minerState.LoadDeadlines(StoreFromCbor(ctx, v.ipldStore))
	if err != nil {
		return nil, err
	}

	// compute first partition index
	partitionSize := minerState.Info.WindowPoStPartitionSectors
	start, sectorCount, err := miner.PartitionsForDeadline(deadlines, partitionSize, deadlineIndex)
	if err != nil {
		return nil, err
	}

	// if deadline contains no sectors, return no partitions
	if sectorCount == 0 {
		return nil, nil
	}

	// compute number of partitions
	partitionCount, _, err := miner.DeadlineCount(deadlines, partitionSize, deadlineIndex)
	if err != nil {
		return nil, err
	}

	partitions := make([]uint64, partitionCount)
	for i := uint64(0); i < partitionCount; i++ {
		partitions[i] = start + i
	}

	return partitions, err
}

// MinerSectorInfoForPartitions retrieves sector info for sectors needed to be proven over for the given proving window partitions.
// NOTE: exposes on-chain structures directly because specs-storage requires it.
func (v *View) MinerSectorInfoForDeadline(ctx context.Context, maddr addr.Address, deadlineIndex uint64, partitions []uint64) ([]abi.SectorInfo, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return nil, err
	}

	deadlines, err := minerState.LoadDeadlines(StoreFromCbor(ctx, v.ipldStore))
	if err != nil {
		return nil, err
	}

	// This is copied from miner.Actor SubmitWindowedPoSt. It should be logic in miner.State.
	partitionSize := minerState.Info.WindowPoStPartitionSectors
	partitionsSectors, err := miner.ComputePartitionsSectors(deadlines, partitionSize, deadlineIndex, partitions)
	if err != nil {
		return nil, err
	}

	provenSectors, err := abi.BitFieldUnion(partitionsSectors...)
	if err != nil {
		return nil, err
	}

	// Extract a fault set relevant to the sectors being submitted, for expansion into a map.
	declaredFaults, err := bitfield.IntersectBitField(provenSectors, minerState.Faults)
	if err != nil {
		return nil, err
	}

	declaredRecoveries, err := bitfield.IntersectBitField(declaredFaults, minerState.Recoveries)
	if err != nil {
		return nil, err
	}

	expectedFaults, err := bitfield.SubtractBitField(declaredFaults, declaredRecoveries)
	if err != nil {
		return nil, err
	}

	nonFaults, err := bitfield.SubtractBitField(provenSectors, expectedFaults)
	if err != nil {
		return nil, err
	}

	empty, err := nonFaults.IsEmpty()
	if err != nil {
		return nil, err
	}

	if empty {
		return nil, fmt.Errorf("no non-faulty sectors in partitions %+v", partitions)
	}

	// Select a non-faulty sector as a substitute for faulty ones.
	goodSectorNo, err := nonFaults.First()
	if err != nil {
		return nil, err
	}

	// Load sector infos for proof
	sectors, err := minerState.LoadSectorInfosWithFaultMask(StoreFromCbor(ctx, v.ipldStore), provenSectors, expectedFaults, abi.SectorNumber(goodSectorNo))
	if err != nil {
		return nil, err
	}

	out := make([]abi.SectorInfo, len(sectors))
	for i, sector := range sectors {
		out[i] = sector.AsSectorInfo()
	}

	return out, nil
}

// MinerSuccessfulPoSts counts how many successful window PoSts have been made this proving period so far.
func (v *View) MinerSuccessfulPoSts(ctx context.Context, maddr addr.Address) (uint64, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return 0, err
	}

	return minerState.PostSubmissions.Count()
}

// MinerDeadlines returns a bitfield of sectors in a proving period
// NOTE: exposes on-chain structures directly because it's referenced directly by the storage-fsm module.
// This is in conflict with the general goal of the state view of hiding the chain state representations from
// consumers in order to support versioning that representation through protocol upgrades.
// See https://github.com/filecoin-project/storage-fsm/issues/13
func (v *View) MinerDeadlines(ctx context.Context, maddr addr.Address) (*miner.Deadlines, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return nil, err
	}

	return minerState.LoadDeadlines(StoreFromCbor(ctx, v.ipldStore))
}

// MinerInfo returns information about the next proving period
// NOTE: exposes on-chain structures directly (but not necessary to)
func (v *View) MinerInfo(ctx context.Context, maddr addr.Address) (miner.MinerInfo, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return miner.MinerInfo{}, err
	}

	return minerState.Info, err
}

func (v *View) MinerProvingPeriodStart(ctx context.Context, maddr addr.Address) (abi.ChainEpoch, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return 0, err
	}
	return minerState.ProvingPeriodStart, nil
}

// MinerSectorsForEach Iterates over the sectors in a miner's proving set.
func (v *View) MinerSectorsForEach(ctx context.Context, maddr addr.Address,
	f func(abi.SectorNumber, cid.Cid, abi.RegisteredProof, []abi.DealID) error) error {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return err
	}

	sectors, err := v.asArray(ctx, minerState.Sectors)
	if err != nil {
		return err
	}

	// This version for the new actors
	var sector miner.SectorOnChainInfo
	return sectors.ForEach(&sector, func(secnum int64) error {
		// Add more fields here as required by new callers.
		return f(sector.Info.SectorNumber, sector.Info.SealedCID, sector.Info.RegisteredProof, sector.Info.DealIDs)
	})
}

// MinerExists Returns true iff the miner exists.
func (v *View) MinerExists(ctx context.Context, maddr addr.Address) (bool, error) {
	_, err := v.loadMinerActor(ctx, maddr)
	if err == nil {
		return true, nil
	}
	if err == types.ErrNotFound {
		return false, nil
	}
	return false, err
}

// MinerFaults Returns all sector ids that are faults
func (v *View) MinerFaults(ctx context.Context, maddr addr.Address) ([]uint64, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return nil, err
	}

	return minerState.Faults.All(miner.SectorsMax)
}

// MinerGetPrecommittedSector Looks up info for a miners precommitted sector.
// NOTE: exposes on-chain structures directly for storage FSM API.
func (v *View) MinerGetPrecommittedSector(ctx context.Context, maddr addr.Address, sectorNum uint64) (*miner.SectorPreCommitOnChainInfo, bool, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return nil, false, err
	}

	return minerState.GetPrecommittedSector(StoreFromCbor(ctx, v.ipldStore), abi.SectorNumber(sectorNum))
}

// MarketEscrowBalance looks up a token amount in the escrow table for the given address
func (v *View) MarketEscrowBalance(ctx context.Context, addr addr.Address) (found bool, amount abi.TokenAmount, err error) {
	marketState, err := v.loadMarketActor(ctx)
	if err != nil {
		return false, abi.NewTokenAmount(0), err
	}

	escrow, err := v.asMap(ctx, marketState.EscrowTable)
	if err != nil {
		return false, abi.NewTokenAmount(0), err
	}

	var value abi.TokenAmount
	found, err = escrow.Get(adt.AddrKey(addr), &value)
	return
}

// MarketComputeDataCommitment takes deal ids and uses associated commPs to compute commD for a sector that contains the deals
func (v *View) MarketComputeDataCommitment(ctx context.Context, registeredProof abi.RegisteredProof, dealIDs []abi.DealID) (cid.Cid, error) {
	marketState, err := v.loadMarketActor(ctx)
	if err != nil {
		return cid.Undef, err
	}

	deals, err := v.asArray(ctx, marketState.Proposals)
	if err != nil {
		return cid.Undef, err
	}

	// map deals to pieceInfo
	pieceInfos := make([]abi.PieceInfo, len(dealIDs))
	for i, id := range dealIDs {
		var proposal market.DealProposal
		found, err := deals.Get(uint64(id), &proposal)
		if err != nil {
			return cid.Undef, err
		}

		if !found {
			return cid.Undef, fmt.Errorf("Could not find deal id %d", id)
		}

		pieceInfos[i].PieceCID = proposal.PieceCID
		pieceInfos[i].Size = proposal.PieceSize
	}

	return ffiwrapper.GenerateUnsealedCID(registeredProof, pieceInfos)
}

// NOTE: exposes on-chain structures directly for storage FSM interface.
func (v *View) MarketDealProposal(ctx context.Context, dealID abi.DealID) (market.DealProposal, error) {
	marketState, err := v.loadMarketActor(ctx)
	if err != nil {
		return market.DealProposal{}, err
	}

	deals, err := v.asArray(ctx, marketState.Proposals)
	if err != nil {
		return market.DealProposal{}, err
	}

	var proposal market.DealProposal
	found, err := deals.Get(uint64(dealID), &proposal)
	if err != nil {
		return market.DealProposal{}, err
	}
	if !found {
		return market.DealProposal{}, fmt.Errorf("Could not find deal id %d", dealID)
	}

	return proposal, nil
}

// NOTE: exposes on-chain structures directly for storage FSM and market module interfaces.
func (v *View) MarketDealState(ctx context.Context, dealID abi.DealID) (*market.DealState, bool, error) {
	marketState, err := v.loadMarketActor(ctx)
	if err != nil {
		return nil, false, err
	}

	dealStates, err := v.asDealStateArray(ctx, marketState.States)
	if err != nil {
		return nil, false, err
	}
	return dealStates.Get(dealID)
}

// NOTE: exposes on-chain structures directly for market interface.
// The callback receives a pointer to a transient object; take a copy or drop the reference outside the callback.
func (v *View) MarketDealStatesForEach(ctx context.Context, f func(id abi.DealID, state *market.DealState) error) error {
	marketState, err := v.loadMarketActor(ctx)
	if err != nil {
		return err
	}

	dealStates, err := v.asDealStateArray(ctx, marketState.States)
	if err != nil {
		return err
	}

	var ds market.DealState
	return dealStates.ForEach(&ds, func(dealId int64) error {
		return f(abi.DealID(dealId), &ds)
	})
}

type NetworkPower struct {
	RawBytePower         abi.StoragePower
	QualityAdjustedPower abi.StoragePower
	MinerCount           int64
	MinPowerMinerCount   int64
}

// Returns the storage power actor's values for network total power.
func (v *View) PowerNetworkTotal(ctx context.Context) (*NetworkPower, error) {
	st, err := v.loadPowerActor(ctx)
	if err != nil {
		return nil, err
	}
	return &NetworkPower{
		RawBytePower:         st.TotalRawBytePower,
		QualityAdjustedPower: st.TotalQualityAdjPower,
		MinerCount:           st.MinerCount,
		MinPowerMinerCount:   st.NumMinersMeetingMinPower,
	}, nil
}

// Returns the power of a miner's committed sectors.
func (v *View) MinerClaimedPower(ctx context.Context, miner addr.Address) (raw, qa abi.StoragePower, err error) {
	minerResolved, err := v.InitResolveAddress(ctx, miner)
	if err != nil {
		return big.Zero(), big.Zero(), err
	}
	powerState, err := v.loadPowerActor(ctx)
	if err != nil {
		return big.Zero(), big.Zero(), err
	}
	claim, err := v.loadPowerClaim(ctx, powerState, minerResolved)
	if err != nil {
		return big.Zero(), big.Zero(), err
	}
	return claim.RawBytePower, claim.QualityAdjPower, nil
}

// PaychActorParties returns the From and To addresses for the given payment channel
func (v *View) PaychActorParties(ctx context.Context, paychAddr addr.Address) (from, to addr.Address, err error) {
	a, err := v.loadActor(ctx, paychAddr)
	if err != nil {
		return addr.Undef, addr.Undef, err
	}
	var state paychActor.State
	err = v.ipldStore.Get(ctx, a.Head.Cid, &state)
	if err != nil {
		return addr.Undef, addr.Undef, err
	}
	return state.From, state.To, nil
}

func (v *View) loadPowerClaim(ctx context.Context, powerState *power.State, miner addr.Address) (*power.Claim, error) {
	claims, err := v.asMap(ctx, powerState.Claims)
	if err != nil {
		return nil, err
	}

	var claim power.Claim
	found, err := claims.Get(adt.AddrKey(miner), &claim)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, types.ErrNotFound
	}
	return &claim, nil
}

func (v *View) loadInitActor(ctx context.Context) (*notinit.State, error) {
	actr, err := v.loadActor(ctx, builtin.InitActorAddr)
	if err != nil {
		return nil, err
	}
	var state notinit.State
	err = v.ipldStore.Get(ctx, actr.Head.Cid, &state)
	return &state, err
}

func (v *View) loadMinerActor(ctx context.Context, address addr.Address) (*miner.State, error) {
	resolvedAddr, err := v.InitResolveAddress(ctx, address)
	if err != nil {
		return nil, err
	}
	actr, err := v.loadActor(ctx, resolvedAddr)
	if err != nil {
		return nil, err
	}
	var state miner.State
	err = v.ipldStore.Get(ctx, actr.Head.Cid, &state)
	return &state, err
}

func (v *View) loadPowerActor(ctx context.Context) (*power.State, error) {
	actr, err := v.loadActor(ctx, builtin.StoragePowerActorAddr)
	if err != nil {
		return nil, err
	}
	var state power.State
	err = v.ipldStore.Get(ctx, actr.Head.Cid, &state)
	return &state, err
}

func (v *View) loadMarketActor(ctx context.Context) (*market.State, error) {
	actr, err := v.loadActor(ctx, builtin.StorageMarketActorAddr)
	if err != nil {
		return nil, err
	}
	var state market.State
	err = v.ipldStore.Get(ctx, actr.Head.Cid, &state)
	return &state, err
}

func (v *View) loadAccountActor(ctx context.Context, a addr.Address) (*account.State, error) {
	resolvedAddr, err := v.InitResolveAddress(ctx, a)
	if err != nil {
		return nil, err
	}
	actr, err := v.loadActor(ctx, resolvedAddr)
	if err != nil {
		return nil, err
	}
	var state account.State
	err = v.ipldStore.Get(ctx, actr.Head.Cid, &state)
	return &state, err
}

func (v *View) loadActor(ctx context.Context, address addr.Address) (*actor.Actor, error) {
	tree, err := v.asMap(ctx, v.root)
	if err != nil {
		return nil, err
	}

	var actr actor.Actor
	found, err := tree.Get(adt.AddrKey(address), &actr)
	if !found {
		return nil, types.ErrNotFound
	}

	return &actr, err
}

func (v *View) asArray(ctx context.Context, root cid.Cid) (*adt.Array, error) {
	return adt.AsArray(StoreFromCbor(ctx, v.ipldStore), root)
}

func (v *View) asMap(ctx context.Context, root cid.Cid) (*adt.Map, error) {
	return adt.AsMap(StoreFromCbor(ctx, v.ipldStore), root)
}

func (v *View) asDealStateArray(ctx context.Context, root cid.Cid) (*market.DealMetaArray, error) {
	return market.AsDealStateArray(StoreFromCbor(ctx, v.ipldStore), root)
}

func (v *View) asBalanceTable(ctx context.Context, root cid.Cid) (*adt.BalanceTable, error) {
	return adt.AsBalanceTable(StoreFromCbor(ctx, v.ipldStore), root)
}

// StoreFromCbor wraps a cbor ipldStore for ADT access.
func StoreFromCbor(ctx context.Context, ipldStore cbor.IpldStore) adt.Store {
	return &cstore{ctx, ipldStore}
}

type cstore struct {
	ctx context.Context
	cbor.IpldStore
}

func (s *cstore) Context() context.Context {
	return s.ctx
}

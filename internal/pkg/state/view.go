package state

import (
	"context"
	"errors"
	"fmt"

	"github.com/filecoin-project/chain-validation/drivers"

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

// MinerSectorSize returns the sector size for a miner actor
func (v *View) MinerSectorSize(ctx context.Context, maddr addr.Address) (abi.SectorSize, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return 0, err
	}
	return minerState.Info.SectorSize, nil
}

// MinerSectorCount counts all the on-chain sectors
func (v *View) MinerSectorCount(ctx context.Context, maddr addr.Address) (int, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return 0, err
	}
	count := 0
	var sector miner.SectorOnChainInfo
	err = v.asArray(ctx, minerState.Sectors).ForEach(&sector, func(i int64) error {
		count++
		return nil
	})
	return count, err
}

// MinerProvingPeriod Returns the start and end of the miner's current/next proving window.
func (v *View) MinerProvingPeriod(ctx context.Context, maddr addr.Address) (start abi.ChainEpoch, end abi.ChainEpoch, failureCount int, err error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return 0, 0, 0, err
	}
	start = minerState.PoStState.ProvingPeriodStart
	end = start + power.WindowedPostChallengeDuration
	// Dragons: change the return to be int64 and all its uses to support it
	failureCount = (int)(minerState.PoStState.NumConsecutiveFailures)
	return
}

// MinerProvingSetForEach Iterates over the sectors in a miner's proving set.
func (v *View) MinerProvingSetForEach(ctx context.Context, maddr addr.Address,
	f func(abi.SectorNumber, cid.Cid, abi.RegisteredProof) error) error {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return err
	}

	// This version for the new actors
	var sector miner.SectorOnChainInfo
	return v.asArray(ctx, minerState.ProvingSet).ForEach(&sector, func(i int64) error {
		// Add more fields here as required by new callers.
		return f(sector.Info.SectorNumber, sector.Info.SealedCID, sector.Info.RegisteredProof)
	})
}

// MinerFaults Returns all sector ids that are faults
func (v *View) MinerFaults(ctx context.Context, maddr addr.Address) ([]uint64, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return nil, err
	}

	return minerState.FaultSet.All(miner.MaxFaultsCount)
}

// MinerGetPrecommittedSector Looks up info for a miners precommitted sector.
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

	var value abi.TokenAmount
	found, err = v.asMap(ctx, marketState.EscrowTable).Get(adt.AddrKey(addr), &value)
	return
}

// MarketGetDeal finds a deal by (resolved) provider and deal id
func (v *View) MarketHasDealID(ctx context.Context, addr addr.Address, dealID abi.DealID) (bool, error) {
	marketState, err := v.loadMarketActor(ctx)
	if err != nil {
		return false, err
	}

	found := false
	byParty := market.AsSetMultimap(StoreFromCbor(ctx, v.ipldStore), marketState.DealIDsByParty)
	if err = byParty.ForEach(addr, func(i abi.DealID) error {
		if i == dealID {
			found = true
		}
		return nil
	}); err != nil {
		return false, err
	}
	return found, err
}

// MarketComputeDataCommitment takes deal ids and uses associated commPs to compute commD for a sector that contains the deals
func (v *View) MarketComputeDateCommitment(ctx context.Context, registeredProof abi.RegisteredProof, dealIDs []abi.DealID) (cid.Cid, error) {
	marketState, err := v.loadMarketActor(ctx)
	deals := v.asMap(ctx, marketState.Proposals)
	if err != nil {
		return cid.Undef, err
	}

	// map deals to pieceInfo
	pieceInfos := make([]abi.PieceInfo, len(dealIDs))
	for i, id := range dealIDs {
		var proposal market.DealProposal
		found, err := deals.Get(adt.UIntKey(uint64(id)), &proposal)
		if err != nil {
			return cid.Undef, err
		}

		if !found {
			return cid.Undef, errors.New(fmt.Sprintf("Could not find deal id %d", id))
		}

		pieceInfos[i].PieceCID = proposal.PieceCID
		pieceInfos[i].Size = proposal.PieceSize
	}

	return drivers.NewChainValidationSyscalls().ComputeUnsealedSectorCID(registeredProof, pieceInfos)
}

func (v *View) MarketStorageDeal(ctx context.Context, dealID abi.DealID) (market.DealProposal, error) {
	marketState, err := v.loadMarketActor(ctx)
	deals := v.asMap(ctx, marketState.Proposals)
	if err != nil {
		return market.DealProposal{}, err
	}

	// map deals to pieceInfo
	var proposal market.DealProposal
	found, err := deals.Get(adt.UIntKey(uint64(dealID)), &proposal)
	if !found {
		return market.DealProposal{}, errors.New(fmt.Sprintf("Could not find deal id %d", dealID))
	}

	return proposal, nil
}

func (v *View) MarketDealState(ctx context.Context, dealID abi.DealID) (*market.DealState, error) {
	marketState, err := v.loadMarketActor(ctx)
	dealStates := v.asDealStateArray(ctx, marketState.States)
	if err != nil {
		return nil, err
	}

	return dealStates.Get(dealID)
}

// NetworkTotalPower Returns the storage power actor's value for network total power.
func (v *View) NetworkTotalPower(ctx context.Context) (abi.StoragePower, error) {
	powerState, err := v.loadPowerActor(ctx)
	if err != nil {
		return big.Zero(), err
	}
	return powerState.TotalNetworkPower, nil
}

// MinerClaimedPower Returns the power of a miner's committed sectors.
func (v *View) MinerClaimedPower(ctx context.Context, miner addr.Address) (abi.StoragePower, error) {
	minerResolved, err := v.InitResolveAddress(ctx, miner)
	if err != nil {
		return big.Zero(), err
	}

	powerState, err := v.loadPowerActor(ctx)
	if err != nil {
		return big.Zero(), err
	}
	claim, err := v.loadPowerClaim(ctx, powerState, minerResolved)
	if err != nil {
		return big.Zero(), err
	}
	return claim.Power, nil
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
	var claim power.Claim
	found, err := v.asMap(ctx, powerState.Claims).Get(adt.AddrKey(miner), &claim)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, types.ErrNotFound
	}
	return &claim, nil
}

// MinerPledgeCollateral returns the locked and balance amounts for a miner actor
func (v *View) MinerPledgeCollateral(ctx context.Context, miner addr.Address) (locked abi.TokenAmount, balance abi.TokenAmount, err error) {
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
	locked = claim.Pledge
	escrow := v.asBalanceTable(ctx, powerState.EscrowTable)
	balance, err = escrow.Get(minerResolved)
	if err != nil {
		return big.Zero(), big.Zero(), err
	}
	return
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
	tree := v.asMap(ctx, v.root)
	var actr actor.Actor
	found, err := tree.Get(adt.AddrKey(address), &actr)
	if !found {
		return nil, types.ErrNotFound
	}

	return &actr, err
}

func (v *View) asArray(ctx context.Context, root cid.Cid) *adt.Array {
	return adt.AsArray(StoreFromCbor(ctx, v.ipldStore), root)
}

func (v *View) asMap(ctx context.Context, root cid.Cid) *adt.Map {
	return adt.AsMap(StoreFromCbor(ctx, v.ipldStore), root)
}

func (v *View) asDealStateArray(ctx context.Context, root cid.Cid) *market.DealMetaArray {
	return market.AsDealStateArray(StoreFromCbor(ctx, v.ipldStore), root)
}

func (v *View) asBalanceTable(ctx context.Context, root cid.Cid) *adt.BalanceTable {
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

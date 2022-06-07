package state

import (
	"context"
	"fmt"
	"strconv"

	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/dline"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"

	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/builtin/v8/miner"
	vmstate "github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/account"
	notinit "github.com/filecoin-project/venus/venus-shared/actors/builtin/init"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/market"
	lminer "github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
	paychActor "github.com/filecoin-project/venus/venus-shared/actors/builtin/paych"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/power"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/reward"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/verifreg"
)

// Viewer builds state views from state root CIDs.
// remove tipset argument in many function of viewer
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
	initState, err := v.LoadInitState(ctx)
	if err != nil {
		return "", err
	}
	return initState.NetworkName()
}

// InitResolveAddress Returns ID address if public key address is given.
func (v *View) InitResolveAddress(ctx context.Context, a addr.Address) (addr.Address, error) {
	if a.Protocol() == addr.ID {
		return a, nil
	}

	initState, err := v.LoadInitState(ctx)
	if err != nil {
		return addr.Undef, err
	}
	rAddr, found, err := initState.ResolveAddress(a)
	if err != nil {
		return addr.Undef, err
	}

	if !found {
		return addr.Undef, fmt.Errorf("not found resolve address")
	}

	return rAddr, nil
}

// ResolveToKeyAddr returns the public key type of address (`BLS`/`SECP256K1`) of an account actor identified by `addr`.
func (v *View) GetMinerWorkerRaw(ctx context.Context, maddr addr.Address) (addr.Address, error) {
	minerState, err := v.LoadMinerState(ctx, maddr)
	if err != nil {
		return addr.Undef, err
	}

	minerInfo, err := minerState.Info()
	if err != nil {
		return addr.Undef, err
	}
	return v.ResolveToKeyAddr(ctx, minerInfo.Worker)
}

// MinerInfo returns info about the indicated miner
func (v *View) MinerInfo(ctx context.Context, maddr addr.Address, nv network.Version) (*lminer.MinerInfo, error) {
	minerState, err := v.LoadMinerState(ctx, maddr)
	if err != nil {
		return nil, err
	}

	info, err := minerState.Info()
	if err != nil {
		return nil, err
	}

	return &info, nil
}

// Loads sector info from miner state.
func (v *View) MinerSectorInfo(ctx context.Context, maddr addr.Address, sectorNum abi.SectorNumber) (*miner.SectorOnChainInfo, error) {
	minerState, err := v.LoadMinerState(ctx, maddr)
	if err != nil {
		return nil, err
	}

	info, err := minerState.GetSector(sectorNum)
	if err != nil {
		return nil, err
	}

	return info, nil
}

//GetSectorsForWinningPoSt return sector of winning post challenge result
func (v *View) GetSectorsForWinningPoSt(ctx context.Context, nv network.Version, pv ffiwrapper.Verifier, maddr addr.Address, rand abi.PoStRandomness) ([]builtin.ExtendedSectorInfo, error) {
	mas, err := v.LoadMinerState(ctx, maddr)
	if err != nil {
		return nil, fmt.Errorf("failed to load miner actor state: %s", err)
	}

	var provingSectors bitfield.BitField
	if nv < network.Version7 {
		allSectors, err := lminer.AllPartSectors(mas, lminer.Partition.AllSectors)
		if err != nil {
			return nil, fmt.Errorf("get all sectors: %v", err)
		}

		faultySectors, err := lminer.AllPartSectors(mas, lminer.Partition.FaultySectors)
		if err != nil {
			return nil, fmt.Errorf("get faulty sectors: %v", err)
		}

		provingSectors, err = bitfield.SubtractBitField(allSectors, faultySectors)
		if err != nil {
			return nil, fmt.Errorf("calc proving sectors: %v", err)
		}
	} else {
		provingSectors, err = lminer.AllPartSectors(mas, lminer.Partition.ActiveSectors)
		if err != nil {
			return nil, fmt.Errorf("get active sectors sectors: %v", err)
		}
	}

	numProvSect, err := provingSectors.Count()
	if err != nil {
		return nil, fmt.Errorf("failed to count bits: %s", err)
	}

	// TODO(review): is this right? feels fishy to me
	if numProvSect == 0 {
		return nil, nil
	}

	info, err := mas.Info()
	if err != nil {
		return nil, fmt.Errorf("getting miner info: %s", err)
	}

	mid, err := addr.IDFromAddress(maddr)
	if err != nil {
		return nil, fmt.Errorf("getting miner ID: %s", err)
	}

	proofType, err := lminer.WinningPoStProofTypeFromWindowPoStProofType(nv, info.WindowPoStProofType)
	if err != nil {
		return nil, fmt.Errorf("determining winning post proof type: %v", err)
	}

	ids, err := pv.GenerateWinningPoStSectorChallenge(ctx, proofType, abi.ActorID(mid), rand, numProvSect)
	if err != nil {
		return nil, fmt.Errorf("generating winning post challenges: %s", err)
	}

	iter, err := provingSectors.BitIterator()
	if err != nil {
		return nil, fmt.Errorf("iterating over proving sectors: %s", err)
	}

	// Select winning sectors by _index_ in the all-sectors bitfield.
	selectedSectors := bitfield.New()
	prev := uint64(0)
	for _, n := range ids {
		sno, err := iter.Nth(n - prev)
		if err != nil {
			return nil, fmt.Errorf("iterating over proving sectors: %s", err)
		}
		selectedSectors.Set(sno)
		prev = n
	}

	sectors, err := mas.LoadSectors(&selectedSectors)
	if err != nil {
		return nil, fmt.Errorf("loading proving sectors: %s", err)
	}

	out := make([]builtin.ExtendedSectorInfo, len(sectors))
	for i, sinfo := range sectors {
		out[i] = builtin.ExtendedSectorInfo{
			SealProof:    sinfo.SealProof,
			SectorNumber: sinfo.SectorNumber,
			SealedCID:    sinfo.SealedCID,
			SectorKey:    sinfo.SectorKeyCID,
		}
	}

	return out, nil
}

// StateSectorPreCommitInfo returns the PreCommit info for the specified miner's sector
func (v *View) SectorPreCommitInfo(ctx context.Context, maddr addr.Address, sid abi.SectorNumber) (*miner.SectorPreCommitOnChainInfo, error) {
	mas, err := v.LoadMinerState(ctx, maddr)
	if err != nil {
		return nil, fmt.Errorf("(get sset) failed to load miner actor: %v", err)
	}

	return mas.GetPrecommittedSector(sid)
}

// StateSectorPartition finds deadline/partition with the specified sector
func (v *View) StateSectorPartition(ctx context.Context, maddr addr.Address, sectorNumber abi.SectorNumber) (*lminer.SectorLocation, error) {
	mas, err := v.LoadMinerState(ctx, maddr)
	if err != nil {
		return nil, fmt.Errorf("(get sset) failed to load miner actor: %v", err)
	}

	return mas.FindSector(sectorNumber)
}

// MinerDeadlineInfo returns information relevant to the current proving deadline
func (v *View) MinerDeadlineInfo(ctx context.Context, maddr addr.Address, epoch abi.ChainEpoch) (index uint64, open, close, challenge abi.ChainEpoch, _ error) {
	minerState, err := v.LoadMinerState(ctx, maddr)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	deadlineInfo, err := minerState.DeadlineInfo(epoch)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	return deadlineInfo.Index, deadlineInfo.Open, deadlineInfo.Close, deadlineInfo.Challenge, nil
}

// MinerExists Returns true iff the miner exists.
func (v *View) MinerExists(ctx context.Context, maddr addr.Address) (bool, error) {
	_, err := v.LoadMinerState(ctx, maddr)
	if err == nil {
		return true, nil
	}
	if err == types.ErrActorNotFound {
		return false, nil
	}
	return false, err
}

// MinerGetPrecommittedSector Looks up info for a miners precommitted sector.
// NOTE: exposes on-chain structures directly for storage FSM API.
func (v *View) MinerGetPrecommittedSector(ctx context.Context, maddr addr.Address, sectorNum abi.SectorNumber) (*miner.SectorPreCommitOnChainInfo, bool, error) {
	minerState, err := v.LoadMinerState(ctx, maddr)
	if err != nil {
		return nil, false, err
	}

	info, err := minerState.GetPrecommittedSector(sectorNum)
	if err != nil {
		return nil, false, err
	}
	return info, true, nil
}

// MarketEscrowBalance looks up a token amount in the escrow table for the given address
func (v *View) MarketEscrowBalance(ctx context.Context, addr addr.Address) (found bool, amount abi.TokenAmount, err error) {
	marketState, err := v.LoadMarketState(ctx)
	if err != nil {
		return false, abi.NewTokenAmount(0), err
	}

	state, err := marketState.EscrowTable()
	if err != nil {
		return false, abi.NewTokenAmount(0), err
	}

	amount, err = state.Get(addr)
	if err != nil {
		return false, abi.NewTokenAmount(0), err
	}

	return true, amount, nil
}

// NOTE: exposes on-chain structures directly for storage FSM interface.
func (v *View) MarketDealProposal(ctx context.Context, dealID abi.DealID) (market.DealProposal, error) {
	marketState, err := v.LoadMarketState(ctx)
	if err != nil {
		return market.DealProposal{}, err
	}

	proposals, err := marketState.Proposals()
	if err != nil {
		return market.DealProposal{}, err
	}

	// map deals to pieceInfo
	proposal, bFound, err := proposals.Get(dealID)
	if err != nil {
		return market.DealProposal{}, err
	}

	if !bFound {
		return market.DealProposal{}, fmt.Errorf("deal %d not found", dealID)
	}
	return *proposal, nil
}

// NOTE: exposes on-chain structures directly for storage FSM and market module interfaces.
func (v *View) MarketDealState(ctx context.Context, dealID abi.DealID) (*market.DealState, bool, error) {
	marketState, err := v.LoadMarketState(ctx)
	if err != nil {
		return nil, false, err
	}

	deals, err := marketState.States()
	if err != nil {
		return nil, false, err
	}

	return deals.Get(dealID)
}

// NOTE: exposes on-chain structures directly for market interface.
// The callback receives a pointer to a transient object; take a copy or drop the reference outside the callback.
func (v *View) MarketDealStatesForEach(ctx context.Context, f func(id abi.DealID, state *market.DealState) error) error {
	marketState, err := v.LoadMarketState(ctx)
	if err != nil {
		return err
	}

	deals, err := marketState.States()
	if err != nil {
		return err
	}

	ff := func(id abi.DealID, ds market.DealState) error {
		return f(id, &ds)
	}
	return deals.ForEach(ff)
}

// StateVerifiedClientStatus returns the data cap for the given address.
// Returns nil if there is no entry in the data cap table for the
// address.
func (v *View) StateVerifiedClientStatus(ctx context.Context, addr addr.Address) (abi.StoragePower, error) {
	act, err := v.loadActor(ctx, verifreg.Address)
	if err != nil {
		return abi.NewStoragePower(0), err
	}

	state, err := verifreg.Load(adt.WrapStore(ctx, v.ipldStore), act)
	if err != nil {
		return abi.NewStoragePower(0), err
	}

	found, storagePower, err := state.VerifiedClientDataCap(addr)
	if err != nil {
		return abi.NewStoragePower(0), err
	}

	if !found {
		return abi.NewStoragePower(0), errors.New("address not found")
	}

	return storagePower, nil
}

// StateMarketStorageDeal returns information about the indicated deal
func (v *View) StateMarketStorageDeal(ctx context.Context, dealID abi.DealID) (*types.MarketDeal, error) {
	state, err := v.LoadMarketState(ctx)
	if err != nil {
		return nil, err
	}

	dealProposals, err := state.Proposals()
	if err != nil {
		return nil, err
	}

	dealProposal, found, err := dealProposals.Get(dealID)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, errors.New("deal proposal not found")
	}

	dealStates, err := state.States()
	if err != nil {
		return nil, err
	}

	dealState, found, err := dealStates.Get(dealID)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, errors.New("deal state not found")
	}

	return &types.MarketDeal{
		Proposal: *dealProposal,
		State:    *dealState,
	}, nil
}

// Returns the storage power actor's values for network total power.
func (v *View) PowerNetworkTotal(ctx context.Context) (*NetworkPower, error) {
	st, err := v.LoadPowerActor(ctx)
	if err != nil {
		return nil, err
	}

	tp, err := st.TotalPower()
	if err != nil {
		return nil, err
	}

	minPowerMinerCount, minerCount, err := st.MinerCounts()
	if err != nil {
		return nil, err
	}

	return &NetworkPower{
		RawBytePower:         tp.RawBytePower,
		QualityAdjustedPower: tp.QualityAdjPower,
		MinerCount:           int64(minerCount),
		MinPowerMinerCount:   int64(minPowerMinerCount),
	}, nil
}

// Returns the power of a miner's committed sectors.
func (v *View) MinerClaimedPower(ctx context.Context, miner addr.Address) (raw, qa abi.StoragePower, err error) {
	st, err := v.LoadPowerActor(ctx)
	if err != nil {
		return big.Zero(), big.Zero(), err
	}

	p, found, err := st.MinerPower(miner)
	if err != nil {
		return big.Zero(), big.Zero(), err
	}

	if !found {
		return big.Zero(), big.Zero(), errors.New("miner not found")
	}

	return p.RawBytePower, p.QualityAdjPower, nil
}

//MinerNominalPowerMeetsConsensusMinimum return whether miner meet consensus minmum power
func (v *View) MinerNominalPowerMeetsConsensusMinimum(ctx context.Context, addr addr.Address) (bool, error) {
	st, err := v.LoadPowerActor(ctx)
	if err != nil {
		return false, err
	}

	return st.MinerNominalPowerMeetsConsensusMinimum(addr)
}

// PaychActorParties returns the From and To addresses for the given payment channel
func (v *View) PaychActorParties(ctx context.Context, paychAddr addr.Address) (from, to addr.Address, err error) {
	a, err := v.loadActor(ctx, paychAddr)
	if err != nil {
		return addr.Undef, addr.Undef, err
	}

	state, err := paychActor.Load(adt.WrapStore(ctx, v.ipldStore), a)
	if err != nil {
		return addr.Undef, addr.Undef, err
	}

	from, err = state.From()
	if err != nil {
		return addr.Undef, addr.Undef, err
	}

	to, err = state.To()
	if err != nil {
		return addr.Undef, addr.Undef, err
	}

	return from, to, nil
}

// StateMinerProvingDeadline calculates the deadline at some epoch for a proving period
// and returns the deadline-related calculations.
func (v *View) StateMinerProvingDeadline(ctx context.Context, addr addr.Address, ts *types.TipSet) (*dline.Info, error) {
	mas, err := v.LoadMinerState(ctx, addr)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get proving dealline")
	}

	height := ts.Height()
	di, err := mas.DeadlineInfo(height)
	if err != nil {
		return nil, fmt.Errorf("failed to get deadline info: %v", err)
	}

	return di.NextNotElapsed(), nil
}

// StateSectorExpiration returns epoch at which given sector will expire
func (v *View) StateSectorExpiration(ctx context.Context, maddr addr.Address, sectorNumber abi.SectorNumber, key types.TipSetKey) (*lminer.SectorExpiration, error) {
	mas, err := v.LoadMinerState(ctx, maddr)
	if err != nil {
		return nil, err
	}
	return mas.GetSectorExpiration(sectorNumber)
}

// StateMinerAvailableBalance returns the portion of a miner's balance that can be withdrawn or spent
func (v *View) StateMinerAvailableBalance(ctx context.Context, maddr addr.Address, ts *types.TipSet) (big.Int, error) {
	resolvedAddr, err := v.InitResolveAddress(ctx, maddr)
	if err != nil {
		return big.Int{}, err
	}
	actor, err := v.loadActor(ctx, resolvedAddr)
	if err != nil {
		return big.Int{}, err
	}

	mas, err := lminer.Load(adt.WrapStore(context.TODO(), v.ipldStore), actor)
	if err != nil {
		return big.Int{}, fmt.Errorf("failed to load miner actor state: %v", err)
	}

	height := ts.Height()
	vested, err := mas.VestedFunds(height)
	if err != nil {
		return big.Int{}, err
	}

	abal, err := mas.AvailableBalance(actor.Balance)
	if err != nil {
		return big.Int{}, err
	}

	return big.Add(abal, vested), nil
}

// StateListMiners returns the addresses of every miner that has claimed power in the Power Actor
func (v *View) StateListMiners(ctx context.Context, tsk types.TipSetKey) ([]addr.Address, error) {
	powState, err := v.LoadPowerActor(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load power actor state: %v", err)
	}

	return powState.ListAllMiners()
}

// StateMinerPower returns the power of the indicated miner
func (v *View) StateMinerPower(ctx context.Context, maddr addr.Address, tsk types.TipSetKey) (power.Claim, power.Claim, bool, error) {
	pas, err := v.LoadPowerActor(ctx)
	if err != nil {
		return power.Claim{}, power.Claim{}, false, fmt.Errorf("(get sset) failed to load power actor state: %v", err)
	}

	tpow, err := pas.TotalPower()
	if err != nil {
		return power.Claim{}, power.Claim{}, false, err
	}

	var mpow power.Claim
	var minpow bool
	if maddr != addr.Undef {
		var found bool
		mpow, found, err = pas.MinerPower(maddr)
		if err != nil || !found {
			// TODO: return an error when not found?
			return power.Claim{}, tpow, false, err
		}

		minpow, err = pas.MinerNominalPowerMeetsConsensusMinimum(maddr)
		if err != nil {
			return power.Claim{}, power.Claim{}, false, err
		}
	}

	return mpow, tpow, minpow, nil
}

// StateMarketDeals returns information about every deal in the Storage Market
func (v *View) StateMarketDeals(ctx context.Context, tsk types.TipSetKey) (map[string]*types.MarketDeal, error) {
	out := map[string]*types.MarketDeal{}

	state, err := v.LoadMarketState(ctx)
	if err != nil {
		return nil, err
	}

	da, err := state.Proposals()
	if err != nil {
		return nil, err
	}

	sa, err := state.States()
	if err != nil {
		return nil, err
	}

	if err := da.ForEach(func(dealID abi.DealID, d market.DealProposal) error {
		s, found, err := sa.Get(dealID)
		if err != nil {
			return fmt.Errorf("failed to get state for deal in proposals array: %v", err)
		} else if !found {
			s = market.EmptyDealState()
		}
		out[strconv.FormatInt(int64(dealID), 10)] = &types.MarketDeal{
			Proposal: d,
			State:    *s,
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// StateMinerActiveSectors returns info about sectors that a given miner is actively proving.
func (v *View) StateMinerActiveSectors(ctx context.Context, maddr addr.Address, tsk types.TipSetKey) ([]*miner.SectorOnChainInfo, error) {
	mas, err := v.LoadMinerState(ctx, maddr)
	if err != nil {
		return nil, fmt.Errorf("failed to load miner actor state: %v", err)
	}
	activeSectors, err := lminer.AllPartSectors(mas, lminer.Partition.ActiveSectors)
	if err != nil {
		return nil, fmt.Errorf("merge partition active sets: %v", err)
	}
	return mas.LoadSectors(&activeSectors)
}

//GetFilLocked return all locked fil amount
func (v *View) GetFilLocked(ctx context.Context, st vmstate.Tree) (abi.TokenAmount, error) {
	filMarketLocked, err := getFilMarketLocked(ctx, v.ipldStore, st)
	if err != nil {
		return big.Zero(), fmt.Errorf("failed to get filMarketLocked: %v", err)
	}

	powerState, err := v.LoadPowerActor(ctx)
	if err != nil {
		return big.Zero(), fmt.Errorf("failed to get filPowerLocked: %v", err)
	}

	filPowerLocked, err := powerState.TotalLocked()
	if err != nil {
		return big.Zero(), fmt.Errorf("failed to get filPowerLocked: %v", err)
	}

	return big.Add(filMarketLocked, filPowerLocked), nil
}

//LoadActor load actor from tree
func (v *View) LoadActor(ctx context.Context, address addr.Address) (*types.Actor, error) {
	return v.loadActor(ctx, address)
}

// ResolveToKeyAddress is similar to `vm.ResolveToKeyAddr` but does not allow `Actor` type of addresses.
// Uses the `TipSet` `ts` to generate the VM state.
func (v *View) ResolveToKeyAddr(ctx context.Context, address addr.Address) (addr.Address, error) {
	if address.Protocol() == addr.BLS || address.Protocol() == addr.SECP256K1 {
		return address, nil
	}

	act, err := v.LoadActor(context.TODO(), address)
	if err != nil {
		return addr.Undef, fmt.Errorf("failed to find actor: %s", address)
	}

	aast, err := account.Load(adt.WrapStore(context.TODO(), v.ipldStore), act)
	if err != nil {
		return addr.Undef, fmt.Errorf("failed to get account actor state for %s: %v", address, err)
	}

	return aast.PubkeyAddress()
}

func (v *View) LoadInitState(ctx context.Context) (notinit.State, error) {
	actr, err := v.loadActor(ctx, notinit.Address)
	if err != nil {
		return nil, err
	}

	return notinit.Load(adt.WrapStore(ctx, v.ipldStore), actr)
}

//LoadPaychState get pay channel state for actor
func (v *View) LoadPaychState(ctx context.Context, actor *types.Actor) (paychActor.State, error) {
	return paychActor.Load(adt.WrapStore(context.TODO(), v.ipldStore), actor)
}

//LoadMinerState return miner state
func (v *View) LoadMinerState(ctx context.Context, maddr addr.Address) (lminer.State, error) {
	resolvedAddr, err := v.InitResolveAddress(ctx, maddr)
	if err != nil {
		return nil, err
	}
	actr, err := v.loadActor(ctx, resolvedAddr)
	if err != nil {
		return nil, err
	}

	return lminer.Load(adt.WrapStore(context.TODO(), v.ipldStore), actr)
}

func (v *View) LoadPowerActor(ctx context.Context) (power.State, error) {
	actr, err := v.loadActor(ctx, power.Address)
	if err != nil {
		return nil, err
	}

	return power.Load(adt.WrapStore(ctx, v.ipldStore), actr)
}

func (v *View) LoadVerifregActor(ctx context.Context) (verifreg.State, error) {
	actr, err := v.loadActor(ctx, verifreg.Address)
	if err != nil {
		return nil, err
	}

	return verifreg.Load(adt.WrapStore(ctx, v.ipldStore), actr)
}

// nolint
func (v *View) LoadRewardState(ctx context.Context) (reward.State, error) {
	actr, err := v.loadActor(ctx, reward.Address)
	if err != nil {
		return nil, err
	}

	return reward.Load(adt.WrapStore(ctx, v.ipldStore), actr)
}

// nolint
func (v *View) LoadPowerState(ctx context.Context) (power.State, error) {
	actr, err := v.loadActor(ctx, power.Address)
	if err != nil {
		return nil, err
	}

	return power.Load(adt.WrapStore(ctx, v.ipldStore), actr)
}

func (v *View) LoadMarketState(ctx context.Context) (market.State, error) {
	actr, err := v.loadActor(ctx, market.Address)
	if err != nil {
		return nil, err
	}

	return market.Load(adt.WrapStore(ctx, v.ipldStore), actr)
}

// nolint
func (v *View) LoadAccountState(ctx context.Context, a addr.Address) (account.State, error) {
	resolvedAddr, err := v.InitResolveAddress(ctx, a)
	if err != nil {
		return nil, err
	}
	actr, err := v.loadActor(ctx, resolvedAddr)
	if err != nil {
		return nil, err
	}

	return account.Load(adt.WrapStore(context.TODO(), v.ipldStore), actr)
}

//loadActor load actor of address in db
func (v *View) loadActor(ctx context.Context, address addr.Address) (*types.Actor, error) {
	tree, err := vmstate.LoadState(ctx, v.ipldStore, v.root)
	if err != nil {
		return nil, err
	}
	actor, found, err := tree.GetActor(ctx, address)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.Wrapf(types.ErrActorNotFound, "address is :%s", address)
	}

	return actor, err
}

func getFilMarketLocked(ctx context.Context, ipldStore cbor.IpldStore, st vmstate.Tree) (abi.TokenAmount, error) {
	mactor, found, err := st.GetActor(ctx, market.Address)
	if !found || err != nil {
		return big.Zero(), fmt.Errorf("failed to load market actor: %v", err)
	}

	mst, err := market.Load(adt.WrapStore(ctx, ipldStore), mactor)
	if err != nil {
		return big.Zero(), fmt.Errorf("failed to load market state: %v", err)
	}

	return mst.TotalLocked()
}

// LookupID retrieves the ID address of the given address
func (v *View) LookupID(ctx context.Context, address addr.Address) (addr.Address, error) {
	sTree, err := vmstate.LoadState(ctx, v.ipldStore, v.root)
	if err != nil {
		return addr.Address{}, err
	}

	return sTree.LookupID(address)
}

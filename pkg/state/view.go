package state

import (
	"context"
	"strconv"

	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/dline"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	xerrors "github.com/pkg/errors"

	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/pkg/specactors/adt"
	"github.com/filecoin-project/venus/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/account"
	notinit "github.com/filecoin-project/venus/pkg/specactors/builtin/init"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/market"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"
	paychActor "github.com/filecoin-project/venus/pkg/specactors/builtin/paych"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/power"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/reward"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/verifreg"
	vmstate "github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
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
	return initState.NetworkName()
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
	rAddr, found, err := initState.ResolveAddress(a)
	if err != nil {
		return addr.Undef, err
	}

	if !found {
		return addr.Undef, xerrors.Errorf("not found resolve address")
	}

	return rAddr, nil
}

// ResolveToKeyAddr returns the public key type of address (`BLS`/`SECP256K1`) of an account actor identified by `addr`.
func (v *View) GetMinerWorkerRaw(ctx context.Context, maddr addr.Address) (addr.Address, error) {
	minerState, err := v.loadMinerState(ctx, maddr)
	if err != nil {
		return addr.Undef, err
	}

	minerInfo, err := minerState.Info()
	if err != nil {
		return addr.Undef, err
	}
	return v.ResolveToKeyAddr(ctx, minerInfo.Worker)
}

func (v *View) MinerInfo(ctx context.Context, maddr addr.Address, nv network.Version) (*miner.MinerInfo, error) {
	minerState, err := v.loadMinerState(ctx, maddr)
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
	minerState, err := v.loadMinerState(ctx, maddr)
	if err != nil {
		return nil, err
	}

	info, err := minerState.GetSector(sectorNum)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func (v *View) GetSectorsForWinningPoSt(ctx context.Context, nv network.Version, pv ffiwrapper.Verifier, st cid.Cid, maddr addr.Address, rand abi.PoStRandomness) ([]builtin.SectorInfo, error) {
	mas, err := v.LoadMinerState(ctx, maddr)
	if err != nil {
		return nil, xerrors.Errorf("failed to load miner actor state: %s", err)
	}

	var provingSectors bitfield.BitField
	if nv < network.Version7 {
		allSectors, err := miner.AllPartSectors(mas, miner.Partition.AllSectors)
		if err != nil {
			return nil, xerrors.Errorf("get all sectors: %v", err)
		}

		faultySectors, err := miner.AllPartSectors(mas, miner.Partition.FaultySectors)
		if err != nil {
			return nil, xerrors.Errorf("get faulty sectors: %v", err)
		}

		provingSectors, err = bitfield.SubtractBitField(allSectors, faultySectors)
		if err != nil {
			return nil, xerrors.Errorf("calc proving sectors: %v", err)
		}
	} else {
		provingSectors, err = miner.AllPartSectors(mas, miner.Partition.ActiveSectors)
		if err != nil {
			return nil, xerrors.Errorf("get active sectors sectors: %v", err)
		}
	}

	numProvSect, err := provingSectors.Count()
	if err != nil {
		return nil, xerrors.Errorf("failed to count bits: %s", err)
	}

	// TODO(review): is this right? feels fishy to me
	if numProvSect == 0 {
		return nil, nil
	}

	info, err := mas.Info()
	if err != nil {
		return nil, xerrors.Errorf("getting miner info: %s", err)
	}

	mid, err := addr.IDFromAddress(maddr)
	if err != nil {
		return nil, xerrors.Errorf("getting miner ID: %s", err)
	}

	proofType, err := miner.WinningPoStProofTypeFromWindowPoStProofType(nv, info.WindowPoStProofType)
	if err != nil {
		return nil, xerrors.Errorf("determining winning post proof type: %v", err)
	}

	ids, err := pv.GenerateWinningPoStSectorChallenge(ctx, proofType, abi.ActorID(mid), rand, numProvSect)
	if err != nil {
		return nil, xerrors.Errorf("generating winning post challenges: %s", err)
	}

	iter, err := provingSectors.BitIterator()
	if err != nil {
		return nil, xerrors.Errorf("iterating over proving sectors: %s", err)
	}

	// Select winning sectors by _index_ in the all-sectors bitfield.
	selectedSectors := bitfield.New()
	prev := uint64(0)
	for _, n := range ids {
		sno, err := iter.Nth(n - prev)
		if err != nil {
			return nil, xerrors.Errorf("iterating over proving sectors: %s", err)
		}
		selectedSectors.Set(sno)
		prev = n
	}

	sectors, err := mas.LoadSectors(&selectedSectors)
	if err != nil {
		return nil, xerrors.Errorf("loading proving sectors: %s", err)
	}

	out := make([]builtin.SectorInfo, len(sectors))
	for i, sinfo := range sectors {
		out[i] = builtin.SectorInfo{
			SealProof:    sinfo.SealProof,
			SectorNumber: sinfo.SectorNumber,
			SealedCID:    sinfo.SealedCID,
		}
	}

	return out, nil
}

func (v *View) GetPartsProving(ctx context.Context, maddr addr.Address) ([]bitfield.BitField, error) {
	minerState, err := v.loadMinerState(ctx, maddr)
	if err != nil {
		return nil, err
	}

	var partsProving []bitfield.BitField

	if err := minerState.ForEachDeadline(func(idx uint64, dl miner.Deadline) error {
		if err := dl.ForEachPartition(func(idx uint64, part miner.Partition) error {
			allSectors, err := part.AllSectors()
			if err != nil {
				return err
			}
			faultySectors, err := part.FaultySectors()
			if err != nil {
				return err
			}
			p, err := bitfield.SubtractBitField(allSectors, faultySectors)
			if err != nil {
				return xerrors.Errorf("subtract faults from partition sectors: %v", err)
			}

			partsProving = append(partsProving, p)

			return nil
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return partsProving, nil
}

func (v *View) PreCommitInfo(ctx context.Context, maddr addr.Address, sid abi.SectorNumber) (*miner.SectorPreCommitOnChainInfo, error) {
	mas, err := v.loadMinerState(ctx, maddr)
	if err != nil {
		return nil, xerrors.Errorf("(get sset) failed to load miner actor: %v", err)
	}

	return mas.GetPrecommittedSector(sid)
}

func (v *View) StateSectorPartition(ctx context.Context, maddr addr.Address, sectorNumber abi.SectorNumber) (*miner.SectorLocation, error) {
	mas, err := v.loadMinerState(ctx, maddr)
	if err != nil {
		return nil, xerrors.Errorf("(get sset) failed to load miner actor: %v", err)
	}

	return mas.FindSector(sectorNumber)
}

// MinerDeadlineInfo returns information relevant to the current proving deadline
func (v *View) MinerDeadlineInfo(ctx context.Context, maddr addr.Address, epoch abi.ChainEpoch) (index uint64, open, close, challenge abi.ChainEpoch, _ error) {
	minerState, err := v.loadMinerState(ctx, maddr)
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
	_, err := v.loadMinerState(ctx, maddr)
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
	minerState, err := v.loadMinerState(ctx, maddr)
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
	marketState, err := v.loadMarketState(ctx)
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

// MarketComputeDataCommitment takes deal ids and uses associated commPs to compute commD for a sector that contains the deals
func (v *View) MarketComputeDataCommitment(ctx context.Context, registeredProof abi.RegisteredSealProof, dealIDs []abi.DealID) (cid.Cid, error) {
	marketState, err := v.loadMarketState(ctx)
	if err != nil {
		return cid.Undef, err
	}

	// todo review
	proposals, err := marketState.Proposals()
	if err != nil {
		return cid.Undef, err
	}

	// map deals to pieceInfo
	pieceInfos := make([]abi.PieceInfo, len(dealIDs))
	for i, id := range dealIDs {
		proposal, bFound, err := proposals.Get(id)
		if err != nil {
			return cid.Undef, err
		}

		if !bFound {
			return cid.Undef, xerrors.Errorf("deal %d not found", id)
		}

		pieceInfos[i].PieceCID = proposal.PieceCID
		pieceInfos[i].Size = proposal.PieceSize
	}
	return ffiwrapper.GenerateUnsealedCID(registeredProof, pieceInfos)
}

// NOTE: exposes on-chain structures directly for storage FSM interface.
func (v *View) MarketDealProposal(ctx context.Context, dealID abi.DealID) (market.DealProposal, error) {
	marketState, err := v.loadMarketState(ctx)
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
		return market.DealProposal{}, xerrors.Errorf("deal %d not found", dealID)
	}
	return *proposal, nil
}

// NOTE: exposes on-chain structures directly for storage FSM and market module interfaces.
func (v *View) MarketDealState(ctx context.Context, dealID abi.DealID) (*market.DealState, bool, error) {
	marketState, err := v.loadMarketState(ctx)
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
	marketState, err := v.loadMarketState(ctx)
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
	if err := deals.ForEach(ff); err != nil {
		return err
	}
	return nil
}

// StateDealProviderCollateralBounds returns the min and max collateral a storage provider
// can issue. It takes the deal size and verified status as parameters.
func (v *View) MarketDealProviderCollateralBounds(ctx context.Context, size abi.PaddedPieceSize, verified bool, height abi.ChainEpoch) (DealCollateralBounds, error) {
	panic("not impl")
}

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
		return abi.NewStoragePower(0), xerrors.New("address not found")
	}

	return storagePower, nil
}

func (v *View) StateMarketStorageDeal(ctx context.Context, dealID abi.DealID) (*MarketDeal, error) {
	state, err := v.loadMarketState(ctx)
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
		return nil, xerrors.New("deal proposal not found")
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
		return nil, xerrors.New("deal state not found")
	}

	return &MarketDeal{
		Proposal: *dealProposal,
		State:    *dealState,
	}, nil
}

// Returns the storage power actor's values for network total power.
func (v *View) PowerNetworkTotal(ctx context.Context) (*NetworkPower, error) {
	st, err := v.loadPowerActor(ctx)
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

func (v *View) GetPowerRaw(ctx context.Context, maddr addr.Address) (power.Claim, power.Claim, bool, error) {
	act, err := v.loadActor(ctx, power.Address)
	if err != nil {
		return power.Claim{}, power.Claim{}, false, xerrors.Errorf("(get sset) failed to load power actor state: %v", err)
	}

	pas, err := power.Load(adt.WrapStore(ctx, v.ipldStore), act)
	if err != nil {
		return power.Claim{}, power.Claim{}, false, err
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
			return power.Claim{}, power.Claim{}, false, err
		}

		minpow, err = pas.MinerNominalPowerMeetsConsensusMinimum(maddr)
		if err != nil {
			return power.Claim{}, power.Claim{}, false, err
		}
	}

	return mpow, tpow, minpow, nil
}

// Returns the power of a miner's committed sectors.
func (v *View) MinerClaimedPower(ctx context.Context, miner addr.Address) (raw, qa abi.StoragePower, err error) {
	st, err := v.loadPowerActor(ctx)
	if err != nil {
		return big.Zero(), big.Zero(), err
	}

	p, found, err := st.MinerPower(miner)
	if err != nil {
		return big.Zero(), big.Zero(), err
	}

	if !found {
		return big.Zero(), big.Zero(), xerrors.New("miner not found")
	}

	return p.RawBytePower, p.QualityAdjPower, nil
}

func (v *View) MinerNominalPowerMeetsConsensusMinimum(ctx context.Context, addr addr.Address) (bool, error) {
	st, err := v.loadPowerActor(ctx)
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

func (v *View) StateMinerProvingDeadline(ctx context.Context, addr addr.Address, ts *types.TipSet) (*dline.Info, error) {
	mas, err := v.loadMinerState(ctx, addr)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to get proving dealline")
	}

	height := ts.Height()
	di, err := mas.DeadlineInfo(height)
	if err != nil {
		return nil, xerrors.Errorf("failed to get deadline info: %v", err)
	}

	return di.NextNotElapsed(), nil
}

func (v *View) StateMinerDeadlineForIdx(ctx context.Context, addr addr.Address, dlIdx uint64, key types.TipSetKey) (miner.Deadline, error) {
	mas, err := v.loadMinerState(ctx, addr)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to get proving dealline")
	}

	return mas.LoadDeadline(dlIdx)
}

func (v *View) StateMinerSectors(ctx context.Context, addr addr.Address, filter *bitfield.BitField, key types.TipSetKey) ([]*ChainSectorInfo, error) {
	mas, err := v.loadMinerState(ctx, addr)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to get proving dealline")
	}

	siset, err := mas.LoadSectors(filter)
	if err != nil {
		return nil, err
	}

	sset := make([]*ChainSectorInfo, len(siset))
	for i, val := range siset {
		sset[i] = &ChainSectorInfo{
			Info: *val,
			ID:   val.SectorNumber,
		}
	}

	return sset, nil
}

func (v *View) StateSectorExpiration(ctx context.Context, maddr addr.Address, sectorNumber abi.SectorNumber, key types.TipSetKey) (*miner.SectorExpiration, error) {
	mas, err := v.LoadMinerState(ctx, maddr)
	if err != nil {
		return nil, err
	}
	return mas.GetSectorExpiration(sectorNumber)
}

func (v *View) StateMinerAvailableBalance(ctx context.Context, maddr addr.Address, ts *types.TipSet) (big.Int, error) {
	resolvedAddr, err := v.InitResolveAddress(ctx, maddr)
	if err != nil {
		return big.Int{}, err
	}
	actor, err := v.loadActor(ctx, resolvedAddr)
	if err != nil {
		return big.Int{}, err
	}

	mas, err := miner.Load(adt.WrapStore(context.TODO(), v.ipldStore), actor)
	if err != nil {
		return big.Int{}, xerrors.Errorf("failed to load miner actor state: %v", err)
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

func (v *View) StateListMiners(ctx context.Context, tsk types.TipSetKey) ([]addr.Address, error) {
	powState, err := v.loadPowerActor(ctx)
	if err != nil {
		return nil, xerrors.Errorf("failed to load power actor state: %v", err)
	}

	return powState.ListAllMiners()
}

func (v *View) StateMinerPower(ctx context.Context, maddr addr.Address, tsk types.TipSetKey) (power.Claim, power.Claim, bool, error) {
	pas, err := v.loadPowerActor(ctx)
	if err != nil {
		return power.Claim{}, power.Claim{}, false, xerrors.Errorf("(get sset) failed to load power actor state: %v", err)
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
			return power.Claim{}, power.Claim{}, false, err
		}

		minpow, err = pas.MinerNominalPowerMeetsConsensusMinimum(maddr)
		if err != nil {
			return power.Claim{}, power.Claim{}, false, err
		}
	}

	return mpow, tpow, minpow, nil
}

func (v *View) StateMarketDeals(ctx context.Context, tsk types.TipSetKey) (map[string]MarketDeal, error) {
	out := map[string]MarketDeal{}

	state, err := v.loadMarketState(ctx)
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
			return xerrors.Errorf("failed to get state for deal in proposals array: %v", err)
		} else if !found {
			s = market.EmptyDealState()
		}
		out[strconv.FormatInt(int64(dealID), 10)] = MarketDeal{
			Proposal: d,
			State:    *s,
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func (v *View) StateMinerActiveSectors(ctx context.Context, maddr addr.Address, tsk types.TipSetKey) ([]*miner.SectorOnChainInfo, error) {
	mas, err := v.loadMinerState(ctx, maddr)
	if err != nil {
		return nil, xerrors.Errorf("failed to load miner actor state: %v", err)
	}
	activeSectors, err := miner.AllPartSectors(mas, miner.Partition.ActiveSectors)
	if err != nil {
		return nil, xerrors.Errorf("merge partition active sets: %v", err)
	}
	return mas.LoadSectors(&activeSectors)
}

func (v *View) GetFilLocked(ctx context.Context, st vmstate.Tree) (abi.TokenAmount, error) {
	filMarketLocked, err := getFilMarketLocked(ctx, v.ipldStore, st)
	if err != nil {
		return big.Zero(), xerrors.Errorf("failed to get filMarketLocked: %v", err)
	}

	powerState, err := v.loadPowerActor(ctx)
	if err != nil {
		return big.Zero(), xerrors.Errorf("failed to get filPowerLocked: %v", err)
	}

	filPowerLocked, err := powerState.TotalLocked()
	if err != nil {
		return big.Zero(), xerrors.Errorf("failed to get filPowerLocked: %v", err)
	}

	return big.Add(filMarketLocked, filPowerLocked), nil
}

func (v *View) LoadActor(ctx context.Context, address addr.Address) (*types.Actor, error) {
	return v.loadActor(ctx, address)
}

func (v *View) ResolveToKeyAddr(ctx context.Context, address addr.Address) (addr.Address, error) {
	if address.Protocol() == addr.BLS || address.Protocol() == addr.SECP256K1 {
		return address, nil
	}

	act, err := v.LoadActor(context.TODO(), address)
	if err != nil {
		return addr.Undef, xerrors.Errorf("failed to find actor: %s", address)
	}

	aast, err := account.Load(adt.WrapStore(context.TODO(), v.ipldStore), act)
	if err != nil {
		return addr.Undef, xerrors.Errorf("failed to get account actor state for %s: %v", address, err)
	}

	return aast.PubkeyAddress()
}

func (v *View) loadInitActor(ctx context.Context) (notinit.State, error) {
	actr, err := v.loadActor(ctx, notinit.Address)
	if err != nil {
		return nil, err
	}

	return notinit.Load(adt.WrapStore(ctx, v.ipldStore), actr)
}

func (v *View) LoadPaychState(ctx context.Context, actr *types.Actor) (paychActor.State, error) {
	return v.loadPaychState(ctx, actr)
}

func (v *View) loadPaychState(ctx context.Context, actr *types.Actor) (paychActor.State, error) {
	return paychActor.Load(adt.WrapStore(context.TODO(), v.ipldStore), actr)
}

func (v *View) LoadMinerState(ctx context.Context, maddr addr.Address) (miner.State, error) {
	return v.loadMinerState(ctx, maddr)
}

func (v *View) LoadMarketState(ctx context.Context) (market.State, error) {
	return v.loadMarketState(ctx)
}

func (v *View) loadMinerState(ctx context.Context, maddr addr.Address) (miner.State, error) {
	resolvedAddr, err := v.InitResolveAddress(ctx, maddr)
	if err != nil {
		return nil, err
	}
	actr, err := v.loadActor(ctx, resolvedAddr)
	if err != nil {
		return nil, err
	}

	return miner.Load(adt.WrapStore(context.TODO(), v.ipldStore), actr)
}

func (v *View) loadPowerActor(ctx context.Context) (power.State, error) {
	actr, err := v.loadActor(ctx, power.Address)
	if err != nil {
		return nil, err
	}

	return power.Load(adt.WrapStore(ctx, v.ipldStore), actr)
}

// nolint
func (v *View) loadRewardState(ctx context.Context) (reward.State, error) {
	actr, err := v.loadActor(ctx, reward.Address)
	if err != nil {
		return nil, err
	}

	return reward.Load(adt.WrapStore(ctx, v.ipldStore), actr)
}

func (v *View) loadMarketState(ctx context.Context) (market.State, error) {
	actr, err := v.loadActor(ctx, market.Address)
	if err != nil {
		return nil, err
	}

	return market.Load(adt.WrapStore(ctx, v.ipldStore), actr)
}

// nolint
func (v *View) loadAccountActor(ctx context.Context, a addr.Address) (account.State, error) {
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
		return nil, xerrors.Wrapf(types.ErrActorNotFound, "address is :%s", address)
	}

	return actor, err
}

func getFilMarketLocked(ctx context.Context, ipldStore cbor.IpldStore, st vmstate.Tree) (abi.TokenAmount, error) {
	mactor, found, err := st.GetActor(ctx, market.Address)
	if !found || err != nil {
		return big.Zero(), xerrors.Errorf("failed to load market actor: %v", err)
	}

	mst, err := market.Load(adt.WrapStore(ctx, ipldStore), mactor)
	if err != nil {
		return big.Zero(), xerrors.Errorf("failed to load market state: %v", err)
	}

	return mst.TotalLocked()
}

func (v *View) LookupID(ctx context.Context, address addr.Address) (addr.Address, error) {
	sTree, err := vmstate.LoadState(ctx, v.ipldStore, v.root)
	if err != nil {
		return addr.Address{}, err
	}

	return sTree.LookupID(address)
}

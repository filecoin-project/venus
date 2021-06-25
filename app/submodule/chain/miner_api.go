package chain

import (
	"context"
	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/app/submodule/apitypes"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/dline"
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/market"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/power"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/reward"
	pstate "github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/types"
)

var _ apiface.IMinerState = &minerStateAPI{}

type minerStateAPI struct {
	*ChainSubmodule
}

//NewMinerStateAPI create miner state api
func NewMinerStateAPI(chain *ChainSubmodule) apiface.IMinerState {
	return &minerStateAPI{ChainSubmodule: chain}
}

// StateMinerSectorAllocated checks if a sector is allocated
func (msa *minerStateAPI) StateMinerSectorAllocated(ctx context.Context, maddr address.Address, s abi.SectorNumber, tsk types.TipSetKey) (bool, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return false, xerrors.Errorf("failed to get tipset %v", err)
	}

	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return false, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, maddr)
	if err != nil {
		return false, xerrors.Errorf("failed to load miner actor state: %v", err)
	}
	return mas.IsAllocated(s)
}

// StateSectorPreCommitInfo returns the PreCommit info for the specified miner's sector
func (msa *minerStateAPI) StateSectorPreCommitInfo(ctx context.Context, maddr address.Address, n abi.SectorNumber, tsk types.TipSetKey) (miner.SectorPreCommitOnChainInfo, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return miner.SectorPreCommitOnChainInfo{}, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return miner.SectorPreCommitOnChainInfo{}, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	pci, err := view.SectorPreCommitInfo(ctx, maddr, n)
	if err != nil {
		return miner.SectorPreCommitOnChainInfo{}, err
	} else if pci == nil {
		return miner.SectorPreCommitOnChainInfo{}, xerrors.Errorf("precommit info is not exists")
	}
	return *pci, nil
}

// StateSectorGetInfo returns the on-chain info for the specified miner's sector. Returns null in case the sector info isn't found
// NOTE: returned info.Expiration may not be accurate in some cases, use StateSectorExpiration to get accurate
// expiration epoch
func (msa *minerStateAPI) StateSectorGetInfo(ctx context.Context, maddr address.Address, n abi.SectorNumber, tsk types.TipSetKey) (*miner.SectorOnChainInfo, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	return view.MinerSectorInfo(ctx, maddr, n)
}

// StateSectorPartition finds deadline/partition with the specified sector
func (msa *minerStateAPI) StateSectorPartition(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tsk types.TipSetKey) (*miner.SectorLocation, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	return view.StateSectorPartition(ctx, maddr, sectorNumber)
}

// StateMinerSectorSize get miner sector size
func (msa *minerStateAPI) StateMinerSectorSize(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (abi.SectorSize, error) {
	// TODO: update storage-fsm to just StateMinerSectorAllocated
	mi, err := msa.StateMinerInfo(ctx, maddr, tsk)
	if err != nil {
		return 0, err
	}
	return mi.SectorSize, nil
}

// StateMinerInfo returns info about the indicated miner
func (msa *minerStateAPI) StateMinerInfo(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (miner.MinerInfo, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return miner.MinerInfo{}, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return miner.MinerInfo{}, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	nv := msa.Fork.GetNtwkVersion(ctx, ts.Height())
	minfo, err := view.MinerInfo(ctx, maddr, nv)
	if err != nil {
		return miner.MinerInfo{}, err
	}
	return *minfo, nil
}

// StateMinerWorkerAddress get miner worker address
func (msa *minerStateAPI) StateMinerWorkerAddress(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (address.Address, error) {
	// TODO: update storage-fsm to just StateMinerInfo
	mi, err := msa.StateMinerInfo(ctx, maddr, tsk)
	if err != nil {
		return address.Undef, err
	}
	return mi.Worker, nil
}

// StateMinerRecoveries returns a bitfield indicating the recovering sectors of the given miner
func (msa *minerStateAPI) StateMinerRecoveries(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (bitfield.BitField, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, maddr)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("failed to load miner actor state: %v", err)
	}

	return miner.AllPartSectors(mas, miner.Partition.RecoveringSectors)
}

// StateMinerFaults returns a bitfield indicating the faulty sectors of the given miner
func (msa *minerStateAPI) StateMinerFaults(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (bitfield.BitField, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, maddr)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("failed to load miner actor state: %v", err)
	}

	return miner.AllPartSectors(mas, miner.Partition.FaultySectors)
}

// StateMinerProvingDeadline calculates the deadline at some epoch for a proving period
// and returns the deadline-related calculations.
func (msa *minerStateAPI) StateMinerProvingDeadline(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (*dline.Info, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, maddr)
	if err != nil {
		return nil, xerrors.Errorf("failed to load miner actor state: %v", err)
	}
	di, err := mas.DeadlineInfo(ts.Height())
	if err != nil {
		return nil, xerrors.Errorf("failed to get deadline info: %v", err)
	}

	return di.NextNotElapsed(), nil
}

// StateMinerPartitions returns all partitions in the specified deadline
func (msa *minerStateAPI) StateMinerPartitions(ctx context.Context, maddr address.Address, dlIdx uint64, tsk types.TipSetKey) ([]apitypes.Partition, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, maddr)
	if err != nil {
		return nil, xerrors.Errorf("failed to load miner actor state: %v", err)
	}

	dl, err := mas.LoadDeadline(dlIdx)
	if err != nil {
		return nil, xerrors.Errorf("failed to load the deadline: %v", err)
	}

	var out []apitypes.Partition
	err = dl.ForEachPartition(func(_ uint64, part miner.Partition) error {
		allSectors, err := part.AllSectors()
		if err != nil {
			return xerrors.Errorf("getting AllSectors: %v", err)
		}

		faultySectors, err := part.FaultySectors()
		if err != nil {
			return xerrors.Errorf("getting FaultySectors: %v", err)
		}

		recoveringSectors, err := part.RecoveringSectors()
		if err != nil {
			return xerrors.Errorf("getting RecoveringSectors: %v", err)
		}

		liveSectors, err := part.LiveSectors()
		if err != nil {
			return xerrors.Errorf("getting LiveSectors: %v", err)
		}

		activeSectors, err := part.ActiveSectors()
		if err != nil {
			return xerrors.Errorf("getting ActiveSectors: %v", err)
		}

		out = append(out, apitypes.Partition{
			AllSectors:        allSectors,
			FaultySectors:     faultySectors,
			RecoveringSectors: recoveringSectors,
			LiveSectors:       liveSectors,
			ActiveSectors:     activeSectors,
		})
		return nil
	})

	return out, err
}

// StateMinerDeadlines returns all the proving deadlines for the given miner
func (msa *minerStateAPI) StateMinerDeadlines(ctx context.Context, maddr address.Address, tsk types.TipSetKey) ([]apitypes.Deadline, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, maddr)
	if err != nil {
		return nil, xerrors.Errorf("failed to load miner actor state: %v", err)
	}

	deadlines, err := mas.NumDeadlines()
	if err != nil {
		return nil, xerrors.Errorf("getting deadline count: %v", err)
	}

	out := make([]apitypes.Deadline, deadlines)
	if err := mas.ForEachDeadline(func(i uint64, dl miner.Deadline) error {
		ps, err := dl.PartitionsPoSted()
		if err != nil {
			return err
		}

		l, err := dl.DisputableProofCount()
		if err != nil {
			return err
		}

		out[i] = apitypes.Deadline{
			PostSubmissions:      ps,
			DisputableProofCount: l,
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// StateMinerSectors returns info about the given miner's sectors. If the filter bitfield is nil, all sectors are included.
func (msa *minerStateAPI) StateMinerSectors(ctx context.Context, maddr address.Address, sectorNos *bitfield.BitField, tsk types.TipSetKey) ([]*miner.SectorOnChainInfo, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, maddr)
	if err != nil {
		return nil, xerrors.Errorf("failed to load miner actor state: %v", err)
	}

	return mas.LoadSectors(sectorNos)
}

// StateMarketStorageDeal returns information about the indicated deal
func (msa *minerStateAPI) StateMarketStorageDeal(ctx context.Context, dealID abi.DealID, tsk types.TipSetKey) (*apitypes.MarketDeal, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	mas, err := view.LoadMarketState(ctx)
	if err != nil {
		return nil, xerrors.Errorf("failed to load miner actor state: %v", err)
	}

	proposals, err := mas.Proposals()
	if err != nil {
		return nil, err
	}

	proposal, found, err := proposals.Get(dealID)

	if err != nil {
		return nil, err
	} else if !found {
		return nil, xerrors.Errorf("deal %d not found", dealID)
	}

	states, err := mas.States()
	if err != nil {
		return nil, err
	}

	st, found, err := states.Get(dealID)
	if err != nil {
		return nil, err
	}

	if !found {
		st = market.EmptyDealState()
	}

	return &apitypes.MarketDeal{
		Proposal: *proposal,
		State:    *st,
	}, nil
}

var initialPledgeNum = big.NewInt(110)
var initialPledgeDen = big.NewInt(100)

// StateMinerInitialPledgeCollateral returns the precommit deposit for the specified miner's sector
func (msa *minerStateAPI) StateMinerPreCommitDepositForPower(ctx context.Context, maddr address.Address, pci miner.SectorPreCommitInfo, tsk types.TipSetKey) (big.Int, error) {
	store := msa.ChainReader.Store(ctx)
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return big.Int{}, err
	}

	sTree, err := tree.LoadState(ctx, store, ts.At(0).ParentStateRoot)
	if err != nil {
		return big.Int{}, err
	}

	ssize, err := pci.SealProof.SectorSize()
	if err != nil {
		return big.Int{}, xerrors.Errorf("failed to get resolve size: %v", err)
	}

	var sectorWeight abi.StoragePower
	if act, found, err := sTree.GetActor(ctx, market.Address); err != nil || !found {
		return big.Int{}, xerrors.Errorf("loading market actor %s: %v", maddr, err)
	} else if s, err := market.Load(store, act); err != nil {
		return big.Int{}, xerrors.Errorf("loading market actor state %s: %v", maddr, err)
	} else if w, vw, err := s.VerifyDealsForActivation(maddr, pci.DealIDs, ts.Height(), pci.Expiration); err != nil {
		return big.Int{}, xerrors.Errorf("verifying deals for activation: %v", err)
	} else {
		// NB: not exactly accurate, but should always lead us to *over* estimate, not under
		duration := pci.Expiration - ts.Height()
		sectorWeight = builtin.QAPowerForWeight(ssize, duration, w, vw)
	}

	var powerSmoothed builtin.FilterEstimate
	if act, found, err := sTree.GetActor(ctx, power.Address); err != nil || !found {
		return big.Int{}, xerrors.Errorf("loading power actor: %v", err)
	} else if s, err := power.Load(store, act); err != nil {
		return big.Int{}, xerrors.Errorf("loading power actor state: %v", err)
	} else if p, err := s.TotalPowerSmoothed(); err != nil {
		return big.Int{}, xerrors.Errorf("failed to determine total power: %v", err)
	} else {
		powerSmoothed = p
	}

	rewardActor, found, err := sTree.GetActor(ctx, reward.Address)
	if err != nil || !found {
		return big.Int{}, xerrors.Errorf("loading miner actor: %v", err)
	}

	rewardState, err := reward.Load(store, rewardActor)
	if err != nil {
		return big.Int{}, xerrors.Errorf("loading reward actor state: %v", err)
	}

	deposit, err := rewardState.PreCommitDepositForPower(powerSmoothed, sectorWeight)
	if err != nil {
		return big.Zero(), xerrors.Errorf("calculating precommit deposit: %v", err)
	}

	return big.Div(big.Mul(deposit, initialPledgeNum), initialPledgeDen), nil
}

// StateMinerInitialPledgeCollateral returns the initial pledge collateral for the specified miner's sector
func (msa *minerStateAPI) StateMinerInitialPledgeCollateral(ctx context.Context, maddr address.Address, pci miner.SectorPreCommitInfo, tsk types.TipSetKey) (big.Int, error) {
	// TODO: this repeats a lot of the previous function. Fix that.
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return big.Int{}, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	store := msa.ChainReader.Store(ctx)
	state, err := tree.LoadState(ctx, store, ts.At(0).ParentStateRoot)
	if err != nil {
		return big.Int{}, xerrors.Errorf("loading state %s: %v", tsk, err)
	}

	ssize, err := pci.SealProof.SectorSize()
	if err != nil {
		return big.Int{}, xerrors.Errorf("failed to get resolve size: %v", err)
	}

	var sectorWeight abi.StoragePower
	if act, found, err := state.GetActor(ctx, market.Address); err != nil || !found {
		return big.Int{}, xerrors.Errorf("loading miner actor %s: %v", maddr, err)
	} else if s, err := market.Load(store, act); err != nil {
		return big.Int{}, xerrors.Errorf("loading market actor state %s: %v", maddr, err)
	} else if w, vw, err := s.VerifyDealsForActivation(maddr, pci.DealIDs, ts.Height(), pci.Expiration); err != nil {
		return big.Int{}, xerrors.Errorf("verifying deals for activation: %v", err)
	} else {
		// NB: not exactly accurate, but should always lead us to *over* estimate, not under
		duration := pci.Expiration - ts.Height()
		sectorWeight = builtin.QAPowerForWeight(ssize, duration, w, vw)
	}

	var (
		powerSmoothed    builtin.FilterEstimate
		pledgeCollateral abi.TokenAmount
	)
	if act, found, err := state.GetActor(ctx, power.Address); err != nil || !found {
		return big.Int{}, xerrors.Errorf("loading miner actor: %v", err)
	} else if s, err := power.Load(store, act); err != nil {
		return big.Int{}, xerrors.Errorf("loading power actor state: %v", err)
	} else if p, err := s.TotalPowerSmoothed(); err != nil {
		return big.Int{}, xerrors.Errorf("failed to determine total power: %v", err)
	} else if c, err := s.TotalLocked(); err != nil {
		return big.Int{}, xerrors.Errorf("failed to determine pledge collateral: %v", err)
	} else {
		powerSmoothed = p
		pledgeCollateral = c
	}

	rewardActor, found, err := state.GetActor(ctx, reward.Address)
	if err != nil || !found {
		return big.Int{}, xerrors.Errorf("loading miner actor: %v", err)
	}

	rewardState, err := reward.Load(store, rewardActor)
	if err != nil {
		return big.Int{}, xerrors.Errorf("loading reward actor state: %v", err)
	}

	circSupply, err := msa.StateVMCirculatingSupplyInternal(ctx, ts.Key())
	if err != nil {
		return big.Zero(), xerrors.Errorf("getting circulating supply: %v", err)
	}

	initialPledge, err := rewardState.InitialPledgeForPower(
		sectorWeight,
		pledgeCollateral,
		&powerSmoothed,
		circSupply.FilCirculating,
	)
	if err != nil {
		return big.Zero(), xerrors.Errorf("calculating initial pledge: %v", err)
	}

	return big.Div(big.Mul(initialPledge, initialPledgeNum), initialPledgeDen), nil
}

// StateVMCirculatingSupplyInternal returns an approximation of the circulating supply of Filecoin at the given tipset.
// This is the value reported by the runtime interface to actors code.
func (msa *minerStateAPI) StateVMCirculatingSupplyInternal(ctx context.Context, tsk types.TipSetKey) (chain.CirculatingSupply, error) {
	store := msa.ChainReader.Store(ctx)
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return chain.CirculatingSupply{}, err
	}

	root, err := msa.ChainReader.GetTipSetStateRoot(ts)
	if err != nil {
		return chain.CirculatingSupply{}, err
	}

	sTree, err := tree.LoadState(ctx, store, root)
	if err != nil {
		return chain.CirculatingSupply{}, err
	}

	return msa.ChainReader.GetCirculatingSupplyDetailed(ctx, ts.Height(), sTree)
}

// StateCirculatingSupply returns the exact circulating supply of Filecoin at the given tipset.
// This is not used anywhere in the protocol itself, and is only for external consumption.
func (msa *minerStateAPI) StateCirculatingSupply(ctx context.Context, tsk types.TipSetKey) (abi.TokenAmount, error) {
	return msa.ChainReader.StateCirculatingSupply(ctx, tsk)
}

// StateMarketDeals returns information about every deal in the Storage Market
func (msa *minerStateAPI) StateMarketDeals(ctx context.Context, tsk types.TipSetKey) (map[string]pstate.MarketDeal, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	return view.StateMarketDeals(ctx, tsk)
}

// StateMinerActiveSectors returns info about sectors that a given miner is actively proving.
func (msa *minerStateAPI) StateMinerActiveSectors(ctx context.Context, maddr address.Address, tsk types.TipSetKey) ([]*miner.SectorOnChainInfo, error) { // TODO: only used in cli
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	return view.StateMinerActiveSectors(ctx, maddr, tsk)
}

// StateLookupID retrieves the ID address of the given address
func (msa *minerStateAPI) StateLookupID(ctx context.Context, addr address.Address, tsk types.TipSetKey) (address.Address, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return address.Undef, xerrors.Errorf("failed to get tipset %v", err)
	}

	state, err := msa.ChainReader.GetTipSetState(ctx, ts)
	if err != nil {
		return address.Undef, xerrors.Errorf("load state tree: %v", err)
	}

	return state.LookupID(addr)
}

// StateListMiners returns the addresses of every miner that has claimed power in the Power Actor
func (msa *minerStateAPI) StateListMiners(ctx context.Context, tsk types.TipSetKey) ([]address.Address, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	return view.StateListMiners(ctx, tsk)
}

// StateListActors returns the addresses of every actor in the state
func (msa *minerStateAPI) StateListActors(ctx context.Context, tsk types.TipSetKey) ([]address.Address, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	st, err := msa.ChainReader.GetTipSetState(ctx, ts)
	if err != nil {
		return nil, xerrors.Errorf("loading state: %v", err)
	}

	var out []address.Address
	err = st.ForEach(func(addr tree.ActorKey, act *types.Actor) error {
		out = append(out, addr)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

// StateMinerPower returns the power of the indicated miner
func (msa *minerStateAPI) StateMinerPower(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*power.MinerPower, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}

	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	return view.StateMinerPower(ctx, addr, tsk)
}

// StateMinerAvailableBalance returns the portion of a miner's balance that can be withdrawn or spent
func (msa *minerStateAPI) StateMinerAvailableBalance(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (big.Int, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return big.Int{}, xerrors.Wrapf(err, "failed to get tipset for %s", tsk.String())
	}

	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return big.Int{}, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	return view.StateMinerAvailableBalance(ctx, maddr, ts)
}

// StateSectorExpiration returns epoch at which given sector will expire
func (msa *minerStateAPI) StateSectorExpiration(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tsk types.TipSetKey) (*miner.SectorExpiration, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}

	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	return view.StateSectorExpiration(ctx, maddr, sectorNumber, tsk)
}

// StateMinerSectorCount returns the number of sectors in a miner's sector set and proving set
func (msa *minerStateAPI) StateMinerSectorCount(ctx context.Context, addr address.Address, tsk types.TipSetKey) (apitypes.MinerSectors, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return apitypes.MinerSectors{}, xerrors.Errorf("failed to get tipset %v", err)
	}

	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return apitypes.MinerSectors{}, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, addr)
	if err != nil {
		return apitypes.MinerSectors{}, err
	}
	var activeCount, liveCount, faultyCount uint64
	if err := mas.ForEachDeadline(func(_ uint64, dl miner.Deadline) error {
		return dl.ForEachPartition(func(_ uint64, part miner.Partition) error {
			if active, err := part.ActiveSectors(); err != nil {
				return err
			} else if count, err := active.Count(); err != nil {
				return err
			} else {
				activeCount += count
			}
			if live, err := part.LiveSectors(); err != nil {
				return err
			} else if count, err := live.Count(); err != nil {
				return err
			} else {
				liveCount += count
			}
			if faulty, err := part.FaultySectors(); err != nil {
				return err
			} else if count, err := faulty.Count(); err != nil {
				return err
			} else {
				faultyCount += count
			}
			return nil
		})
	}); err != nil {
		return apitypes.MinerSectors{}, err
	}
	return apitypes.MinerSectors{Live: liveCount, Active: activeCount, Faulty: faultyCount}, nil
}

// StateMarketBalance looks up the Escrow and Locked balances of the given address in the Storage Market
func (msa *minerStateAPI) StateMarketBalance(ctx context.Context, addr address.Address, tsk types.TipSetKey) (apitypes.MarketBalance, error) {
	ts, err := msa.ChainReader.GetTipSet(tsk)
	if err != nil {
		return apitypes.MarketBalanceNil, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}
	view, err := msa.ChainReader.ParentStateView(ts)
	if err != nil {
		return apitypes.MarketBalanceNil, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	mstate, err := view.LoadMarketState(ctx)
	if err != nil {
		return apitypes.MarketBalanceNil, err
	}

	addr, err = view.LookupID(ctx, addr)
	if err != nil {
		return apitypes.MarketBalanceNil, err
	}

	var out apitypes.MarketBalance

	et, err := mstate.EscrowTable()
	if err != nil {
		return apitypes.MarketBalanceNil, err
	}
	out.Escrow, err = et.Get(addr)
	if err != nil {
		return apitypes.MarketBalanceNil, xerrors.Errorf("getting escrow balance: %v", err)
	}

	lt, err := mstate.LockedTable()
	if err != nil {
		return apitypes.MarketBalanceNil, err
	}
	out.Locked, err = lt.Get(addr)
	if err != nil {
		return apitypes.MarketBalanceNil, xerrors.Errorf("getting locked balance: %v", err)
	}

	return out, nil

}

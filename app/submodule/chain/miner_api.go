package chain

import (
	"context"

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
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm/state"
)

type IMinerState interface {
	StateMinerSectorAllocated(ctx context.Context, maddr address.Address, s abi.SectorNumber, tsk types.TipSetKey) (bool, error)
	StateSectorPreCommitInfo(ctx context.Context, maddr address.Address, n abi.SectorNumber, tsk types.TipSetKey) (miner.SectorPreCommitOnChainInfo, error)
	StateSectorGetInfo(ctx context.Context, maddr address.Address, n abi.SectorNumber, tsk types.TipSetKey) (*miner.SectorOnChainInfo, error)
	StateSectorPartition(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tsk types.TipSetKey) (*miner.SectorLocation, error)
	StateMinerSectorSize(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (abi.SectorSize, error)
	StateMinerInfo(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (miner.MinerInfo, error)
	StateMinerWorkerAddress(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (address.Address, error)
	StateMinerRecoveries(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (bitfield.BitField, error)
	StateMinerFaults(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (bitfield.BitField, error)
	StateMinerProvingDeadline(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (*dline.Info, error)
	StateMinerPartitions(ctx context.Context, maddr address.Address, dlIdx uint64, tsk types.TipSetKey) ([]Partition, error)
	StateMinerDeadlines(ctx context.Context, maddr address.Address, tsk types.TipSetKey) ([]Deadline, error)
	StateMinerSectors(ctx context.Context, maddr address.Address, sectorNos *bitfield.BitField, tsk types.TipSetKey) ([]*miner.SectorOnChainInfo, error)
	StateMarketStorageDeal(ctx context.Context, dealID abi.DealID, tsk types.TipSetKey) (*MarketDeal, error)
	StateMinerPreCommitDepositForPower(ctx context.Context, maddr address.Address, pci miner.SectorPreCommitInfo, tsk types.TipSetKey) (big.Int, error)
	StateMinerInitialPledgeCollateral(ctx context.Context, maddr address.Address, pci miner.SectorPreCommitInfo, tsk types.TipSetKey) (big.Int, error)
	StateVMCirculatingSupplyInternal(ctx context.Context, tsk types.TipSetKey) (chain.CirculatingSupply, error)
	StateCirculatingSupply(ctx context.Context, tsk types.TipSetKey) (abi.TokenAmount, error)
	StateMarketDeals(ctx context.Context, tsk types.TipSetKey) (map[string]pstate.MarketDeal, error)
	StateMinerActiveSectors(ctx context.Context, maddr address.Address, tsk types.TipSetKey) ([]*miner.SectorOnChainInfo, error)
	StateLookupID(ctx context.Context, addr address.Address, tsk types.TipSetKey) (address.Address, error)
	StateListMiners(ctx context.Context, tsk types.TipSetKey) ([]address.Address, error)
	StateListActors(ctx context.Context, tsk types.TipSetKey) ([]address.Address, error)
	StateMinerPower(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*power.MinerPower, error)
	StateMinerAvailableBalance(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (big.Int, error)
	StateSectorExpiration(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tsk types.TipSetKey) (*miner.SectorExpiration, error)
	StateMinerSectorCount(ctx context.Context, addr address.Address, tsk types.TipSetKey) (MinerSectors, error)
	StateMarketBalance(ctx context.Context, addr address.Address, tsk types.TipSetKey) (MarketBalance, error)
}

type MinerStateAPI struct {
	chain *ChainSubmodule
}

func NewMinerStateAPI(chain *ChainSubmodule) MinerStateAPI {
	return MinerStateAPI{chain: chain}
}

func (minerStateAPI *MinerStateAPI) StateMinerSectorAllocated(ctx context.Context, maddr address.Address, s abi.SectorNumber, tsk types.TipSetKey) (bool, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return false, xerrors.Errorf("failed to get tipset %v", err)
	}

	view, err := minerStateAPI.chain.State.ParentStateView(ts)
	if err != nil {
		return false, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, maddr)
	if err != nil {
		return false, xerrors.Errorf("failed to load miner actor state: %v", err)
	}
	return mas.IsAllocated(s)
}

func (minerStateAPI *MinerStateAPI) StateSectorPreCommitInfo(ctx context.Context, maddr address.Address, n abi.SectorNumber, tsk types.TipSetKey) (miner.SectorPreCommitOnChainInfo, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return miner.SectorPreCommitOnChainInfo{}, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := minerStateAPI.chain.State.ParentStateView(ts)
	if err != nil {
		return miner.SectorPreCommitOnChainInfo{}, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	pci, err := view.PreCommitInfo(ctx, maddr, n)
	if err != nil {
		return miner.SectorPreCommitOnChainInfo{}, err
	} else if pci == nil {
		return miner.SectorPreCommitOnChainInfo{}, xerrors.Errorf("precommit info is not exists")
	}
	return *pci, nil
}

func (minerStateAPI *MinerStateAPI) StateSectorGetInfo(ctx context.Context, maddr address.Address, n abi.SectorNumber, tsk types.TipSetKey) (*miner.SectorOnChainInfo, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := minerStateAPI.chain.State.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	return view.MinerSectorInfo(ctx, maddr, n)
}

func (minerStateAPI *MinerStateAPI) StateSectorPartition(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tsk types.TipSetKey) (*miner.SectorLocation, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := minerStateAPI.chain.State.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	return view.StateSectorPartition(ctx, maddr, sectorNumber)
}

func (minerStateAPI *MinerStateAPI) StateMinerSectorSize(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (abi.SectorSize, error) {
	// TODO: update storage-fsm to just StateMinerSectorAllocated
	mi, err := minerStateAPI.StateMinerInfo(ctx, maddr, tsk)
	if err != nil {
		return 0, err
	}
	return mi.SectorSize, nil
}

func (minerStateAPI *MinerStateAPI) StateMinerInfo(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (miner.MinerInfo, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return miner.MinerInfo{}, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	view, err := minerStateAPI.chain.State.ParentStateView(ts)
	if err != nil {
		return miner.MinerInfo{}, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	nv := minerStateAPI.chain.Fork.GetNtwkVersion(ctx, ts.Height())
	minfo, err := view.MinerInfo(ctx, maddr, nv)
	if err != nil {
		return miner.MinerInfo{}, err
	}
	return *minfo, nil
}

func (minerStateAPI *MinerStateAPI) StateMinerWorkerAddress(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (address.Address, error) {
	// TODO: update storage-fsm to just StateMinerInfo
	mi, err := minerStateAPI.StateMinerInfo(ctx, maddr, tsk)
	if err != nil {
		return address.Undef, err
	}
	return mi.Worker, nil
}

func (minerStateAPI *MinerStateAPI) StateMinerRecoveries(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (bitfield.BitField, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := minerStateAPI.chain.State.ParentStateView(ts)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, maddr)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("failed to load miner actor state: %v", err)
	}

	return miner.AllPartSectors(mas, miner.Partition.RecoveringSectors)
}

func (minerStateAPI *MinerStateAPI) StateMinerFaults(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (bitfield.BitField, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := minerStateAPI.chain.State.ParentStateView(ts)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, maddr)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("failed to load miner actor state: %v", err)
	}

	return miner.AllPartSectors(mas, miner.Partition.FaultySectors)
}

func (minerStateAPI *MinerStateAPI) StateMinerProvingDeadline(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (*dline.Info, error) {
	ts, err := minerStateAPI.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	view, err := minerStateAPI.chain.State.ParentStateView(ts)
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

func (minerStateAPI *MinerStateAPI) StateMinerPartitions(ctx context.Context, maddr address.Address, dlIdx uint64, tsk types.TipSetKey) ([]Partition, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := minerStateAPI.chain.State.ParentStateView(ts)
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

	var out []Partition
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

		out = append(out, Partition{
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

func (minerStateAPI *MinerStateAPI) StateMinerDeadlines(ctx context.Context, maddr address.Address, tsk types.TipSetKey) ([]Deadline, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := minerStateAPI.chain.State.ParentStateView(ts)
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

	out := make([]Deadline, deadlines)
	if err := mas.ForEachDeadline(func(i uint64, dl miner.Deadline) error {
		ps, err := dl.PartitionsPoSted()
		if err != nil {
			return err
		}

		out[i] = Deadline{
			PostSubmissions: ps,
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func (minerStateAPI *MinerStateAPI) StateMinerSectors(ctx context.Context, maddr address.Address, sectorNos *bitfield.BitField, tsk types.TipSetKey) ([]*miner.SectorOnChainInfo, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := minerStateAPI.chain.State.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, maddr)
	if err != nil {
		return nil, xerrors.Errorf("failed to load miner actor state: %v", err)
	}

	return mas.LoadSectors(sectorNos)
}

func (minerStateAPI *MinerStateAPI) StateMarketStorageDeal(ctx context.Context, dealID abi.DealID, tsk types.TipSetKey) (*MarketDeal, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := minerStateAPI.chain.State.ParentStateView(ts)
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

	return &MarketDeal{
		Proposal: *proposal,
		State:    *st,
	}, nil
}

var initialPledgeNum = big.NewInt(110)
var initialPledgeDen = big.NewInt(100)

func (minerStateAPI *MinerStateAPI) StateMinerPreCommitDepositForPower(ctx context.Context, maddr address.Address, pci miner.SectorPreCommitInfo, tsk types.TipSetKey) (big.Int, error) {
	store := minerStateAPI.chain.State.Store(ctx)
	ts, err := minerStateAPI.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return big.Int{}, err
	}

	sTree, err := state.LoadState(ctx, store, ts.At(0).ParentStateRoot)
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

func (minerStateAPI *MinerStateAPI) StateMinerInitialPledgeCollateral(ctx context.Context, maddr address.Address, pci miner.SectorPreCommitInfo, tsk types.TipSetKey) (big.Int, error) {
	// TODO: this repeats a lot of the previous function. Fix that.
	ts, err := minerStateAPI.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return big.Int{}, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	store := minerStateAPI.chain.State.Store(ctx)
	state, err := state.LoadState(ctx, store, ts.At(0).ParentStateRoot)
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

	circSupply, err := minerStateAPI.StateVMCirculatingSupplyInternal(ctx, ts.Key())
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

func (minerStateAPI *MinerStateAPI) StateVMCirculatingSupplyInternal(ctx context.Context, tsk types.TipSetKey) (chain.CirculatingSupply, error) {
	store := minerStateAPI.chain.State.Store(ctx)
	ts, err := minerStateAPI.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return chain.CirculatingSupply{}, err
	}

	root, err := minerStateAPI.chain.State.GetTipSetStateRoot(ctx, ts)
	if err != nil {
		return chain.CirculatingSupply{}, err
	}

	sTree, err := state.LoadState(ctx, store, root)
	if err != nil {
		return chain.CirculatingSupply{}, err
	}

	return minerStateAPI.chain.ChainReader.GetCirculatingSupplyDetailed(ctx, ts.Height(), sTree)
}

func (minerStateAPI *MinerStateAPI) StateCirculatingSupply(ctx context.Context, tsk types.TipSetKey) (abi.TokenAmount, error) {
	return minerStateAPI.chain.ChainReader.StateCirculatingSupply(ctx, tsk)
}

func (minerStateAPI *MinerStateAPI) StateMarketDeals(ctx context.Context, tsk types.TipSetKey) (map[string]pstate.MarketDeal, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := minerStateAPI.chain.State.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	return view.StateMarketDeals(ctx, tsk)
}

func (minerStateAPI *MinerStateAPI) StateMinerActiveSectors(ctx context.Context, maddr address.Address, tsk types.TipSetKey) ([]*miner.SectorOnChainInfo, error) { // TODO: only used in cli
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := minerStateAPI.chain.State.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	return view.StateMinerActiveSectors(ctx, maddr, tsk)
}

func (minerStateAPI *MinerStateAPI) StateLookupID(ctx context.Context, addr address.Address, tsk types.TipSetKey) (address.Address, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return address.Undef, xerrors.Errorf("failed to get tipset %v", err)
	}

	state, err := minerStateAPI.chain.State.GetTipSetState(ctx, ts)
	if err != nil {
		return address.Undef, xerrors.Errorf("load state tree: %v", err)
	}

	return state.LookupID(addr)
}

func (minerStateAPI *MinerStateAPI) StateListMiners(ctx context.Context, tsk types.TipSetKey) ([]address.Address, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	view, err := minerStateAPI.chain.State.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	return view.StateListMiners(ctx, tsk)
}

func (minerStateAPI *MinerStateAPI) StateListActors(ctx context.Context, tsk types.TipSetKey) ([]address.Address, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}
	st, err := minerStateAPI.chain.ChainReader.GetTipSetState(ctx, ts)
	if err != nil {
		return nil, xerrors.Errorf("loading state: %v", err)
	}

	var out []address.Address
	err = st.ForEach(func(addr state.ActorKey, act *types.Actor) error {
		out = append(out, addr)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (minerStateAPI *MinerStateAPI) StateMinerPower(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*power.MinerPower, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}

	view, err := minerStateAPI.chain.State.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	return view.StateMinerPower(ctx, addr, tsk)
}

func (minerStateAPI *MinerStateAPI) StateMinerAvailableBalance(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (big.Int, error) {
	ts, err := minerStateAPI.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return big.Int{}, xerrors.Wrapf(err, "failed to get tipset for %s", tsk.String())
	}

	view, err := minerStateAPI.chain.State.ParentStateView(ts)
	if err != nil {
		return big.Int{}, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	return view.StateMinerAvailableBalance(ctx, maddr, ts)
}

func (minerStateAPI *MinerStateAPI) StateSectorExpiration(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tsk types.TipSetKey) (*miner.SectorExpiration, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset %v", err)
	}

	view, err := minerStateAPI.chain.State.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	return view.StateSectorExpiration(ctx, maddr, sectorNumber, tsk)
}

func (minerStateAPI *MinerStateAPI) StateMinerSectorCount(ctx context.Context, addr address.Address, tsk types.TipSetKey) (MinerSectors, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return MinerSectors{}, xerrors.Errorf("failed to get tipset %v", err)
	}

	view, err := minerStateAPI.chain.State.ParentStateView(ts)
	if err != nil {
		return MinerSectors{}, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, addr)
	if err != nil {
		return MinerSectors{}, err
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
		return MinerSectors{}, err
	}
	return MinerSectors{Live: liveCount, Active: activeCount, Faulty: faultyCount}, nil
}

func (minerStateAPI *MinerStateAPI) StateMarketBalance(ctx context.Context, addr address.Address, tsk types.TipSetKey) (MarketBalance, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return MarketBalance{}, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}
	view, err := minerStateAPI.chain.State.ParentStateView(ts)
	if err != nil {
		return MarketBalance{}, xerrors.Errorf("loading view %s: %v", tsk, err)
	}

	mstate, err := view.LoadMarketState(ctx)
	if err != nil {
		return MarketBalance{}, err
	}

	addr, err = view.LookupID(ctx, addr)
	if err != nil {
		return MarketBalance{}, err
	}

	var out MarketBalance

	et, err := mstate.EscrowTable()
	if err != nil {
		return MarketBalance{}, err
	}
	out.Escrow, err = et.Get(addr)
	if err != nil {
		return MarketBalance{}, xerrors.Errorf("getting escrow balance: %v", err)
	}

	lt, err := mstate.LockedTable()
	if err != nil {
		return MarketBalance{}, err
	}
	out.Locked, err = lt.Get(addr)
	if err != nil {
		return MarketBalance{}, xerrors.Errorf("getting locked balance: %v", err)
	}

	return out, nil

}

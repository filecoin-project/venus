package chain

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/dline"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/market"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/power"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/reward"
	"github.com/filecoin-project/venus/pkg/vm/state"
	xerrors "github.com/pkg/errors"
)

type MinerStateAPI struct {
	chain *ChainSubmodule
}

func NewMinerStateAPI(chain *ChainSubmodule) MinerStateAPI {
	return MinerStateAPI{chain: chain}
}

func (minerStateAPI *MinerStateAPI) StateMinerSectorAllocated(ctx context.Context, maddr address.Address, s abi.SectorNumber, tsk block.TipSetKey) (bool, error) {
	view, err := minerStateAPI.chain.State.ParentStateView(tsk)
	if err != nil {
		return false, xerrors.Errorf("loading tipset %s: %w", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, maddr)
	if err != nil {
		return false, xerrors.Errorf("failed to load miner actor state: %w", err)
	}
	return mas.IsAllocated(s)
}

func (minerStateAPI *MinerStateAPI) StateSectorPreCommitInfo(ctx context.Context, maddr address.Address, n abi.SectorNumber, tsk block.TipSetKey) (miner.SectorPreCommitOnChainInfo, error) {
	view, err := minerStateAPI.chain.State.ParentStateView(tsk)
	if err != nil {
		return miner.SectorPreCommitOnChainInfo{}, xerrors.Errorf("loading tipset %s: %w", tsk, err)
	}

	pci, err := view.PreCommitInfo(ctx, maddr, n)
	if err != nil {
		return miner.SectorPreCommitOnChainInfo{}, err
	} else if pci == nil {
		return miner.SectorPreCommitOnChainInfo{}, xerrors.Errorf("precommit info is not exists")
	}
	return *pci, nil
}

func (minerStateAPI *MinerStateAPI) StateSectorGetInfo(ctx context.Context, maddr address.Address, n abi.SectorNumber, tsk block.TipSetKey) (*miner.SectorOnChainInfo, error) {
	view, err := minerStateAPI.chain.State.ParentStateView(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %w", tsk, err)
	}

	return view.MinerSectorInfo(ctx, maddr, n)
}

func (minerStateAPI *MinerStateAPI) StateSectorPartition(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tsk block.TipSetKey) (*miner.SectorLocation, error) {
	view, err := minerStateAPI.chain.State.ParentStateView(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %w", tsk, err)
	}

	return view.StateSectorPartition(ctx, maddr, sectorNumber)
}

func (minerStateAPI *MinerStateAPI) StateMinerSectorSize(ctx context.Context, maddr address.Address, tsk block.TipSetKey) (abi.SectorSize, error) {
	// TODO: update storage-fsm to just StateMinerSectorAllocated
	mi, err := minerStateAPI.StateMinerInfo(ctx, maddr, tsk)
	if err != nil {
		return 0, err
	}
	return mi.SectorSize, nil
}

func (minerStateAPI *MinerStateAPI) StateMinerInfo(ctx context.Context, maddr address.Address, tsk block.TipSetKey) (miner.MinerInfo, error) {
	ts, err := minerStateAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return miner.MinerInfo{}, xerrors.Errorf("loading tipset %s: %w", tsk, err)
	}

	view, err := minerStateAPI.chain.State.ParentStateView(tsk)
	if err != nil {
		return miner.MinerInfo{}, xerrors.Errorf("loading view %s: %w", tsk, err)
	}

	nv := minerStateAPI.chain.Fork.GetNtwkVersion(ctx, ts.EnsureHeight())
	minfo, err := view.MinerInfo(ctx, maddr, nv)
	if err != nil {
		return miner.MinerInfo{}, err
	}
	return *minfo, nil
}

func (minerStateAPI *MinerStateAPI) StateMinerWorkerAddress(ctx context.Context, maddr address.Address, tsk block.TipSetKey) (address.Address, error) {
	// TODO: update storage-fsm to just StateMinerInfo
	mi, err := minerStateAPI.StateMinerInfo(ctx, maddr, tsk)
	if err != nil {
		return address.Undef, err
	}
	return mi.Worker, nil
}

func (minerStateAPI *MinerStateAPI) StateMinerRecoveries(ctx context.Context, maddr address.Address, tsk block.TipSetKey) (bitfield.BitField, error) {
	view, err := minerStateAPI.chain.State.ParentStateView(tsk)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("loading view %s: %w", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, maddr)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("failed to load miner actor state: %w", err)
	}

	return miner.AllPartSectors(mas, miner.Partition.RecoveringSectors)
}

func (minerStateAPI *MinerStateAPI) StateMinerFaults(ctx context.Context, maddr address.Address, tsk block.TipSetKey) (bitfield.BitField, error) {
	view, err := minerStateAPI.chain.State.ParentStateView(tsk)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("loading view %s: %w", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, maddr)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("failed to load miner actor state: %w", err)
	}

	return miner.AllPartSectors(mas, miner.Partition.FaultySectors)
}

func (minerStateAPI *MinerStateAPI) StateMinerProvingDeadline(ctx context.Context, maddr address.Address, tsk block.TipSetKey) (*dline.Info, error) {
	ts, err := minerStateAPI.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %w", tsk, err)
	}

	view, err := minerStateAPI.chain.State.ParentStateView(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading view %s: %w", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, maddr)
	if err != nil {
		return nil, xerrors.Errorf("failed to load miner actor state: %w", err)
	}
	di, err := mas.DeadlineInfo(ts.EnsureHeight())
	if err != nil {
		return nil, xerrors.Errorf("failed to get deadline info: %w", err)
	}

	return di.NextNotElapsed(), nil
}

func (minerStateAPI *MinerStateAPI) StateMinerPartitions(ctx context.Context, maddr address.Address, dlIdx uint64, tsk block.TipSetKey) ([]Partition, error) {
	view, err := minerStateAPI.chain.State.ParentStateView(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading view %s: %w", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, maddr)
	if err != nil {
		return nil, xerrors.Errorf("failed to load miner actor state: %w", err)
	}

	dl, err := mas.LoadDeadline(dlIdx)
	if err != nil {
		return nil, xerrors.Errorf("failed to load the deadline: %w", err)
	}

	var out []Partition
	err = dl.ForEachPartition(func(_ uint64, part miner.Partition) error {
		allSectors, err := part.AllSectors()
		if err != nil {
			return xerrors.Errorf("getting AllSectors: %w", err)
		}

		faultySectors, err := part.FaultySectors()
		if err != nil {
			return xerrors.Errorf("getting FaultySectors: %w", err)
		}

		recoveringSectors, err := part.RecoveringSectors()
		if err != nil {
			return xerrors.Errorf("getting RecoveringSectors: %w", err)
		}

		liveSectors, err := part.LiveSectors()
		if err != nil {
			return xerrors.Errorf("getting LiveSectors: %w", err)
		}

		activeSectors, err := part.ActiveSectors()
		if err != nil {
			return xerrors.Errorf("getting ActiveSectors: %w", err)
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

func (minerStateAPI *MinerStateAPI) StateMinerDeadlines(ctx context.Context, maddr address.Address, tsk block.TipSetKey) ([]Deadline, error) {
	view, err := minerStateAPI.chain.State.ParentStateView(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading view %s: %w", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, maddr)
	if err != nil {
		return nil, xerrors.Errorf("failed to load miner actor state: %w", err)
	}

	deadlines, err := mas.NumDeadlines()
	if err != nil {
		return nil, xerrors.Errorf("getting deadline count: %w", err)
	}

	out := make([]Deadline, deadlines)
	if err := mas.ForEachDeadline(func(i uint64, dl miner.Deadline) error {
		ps, err := dl.PostSubmissions()
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

func (minerStateAPI *MinerStateAPI) StateMinerSectors(ctx context.Context, maddr address.Address, sectorNos *bitfield.BitField, tsk block.TipSetKey) ([]*miner.SectorOnChainInfo, error) {
	view, err := minerStateAPI.chain.State.ParentStateView(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading view %s: %w", tsk, err)
	}

	mas, err := view.LoadMinerState(ctx, maddr)
	if err != nil {
		return nil, xerrors.Errorf("failed to load miner actor state: %w", err)
	}

	return mas.LoadSectors(sectorNos)
}

func (minerStateAPI *MinerStateAPI) StateMarketStorageDeal(ctx context.Context, dealID abi.DealID, tsk block.TipSetKey) (*MarketDeal, error) {
	view, err := minerStateAPI.chain.State.ParentStateView(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading view %s: %w", tsk, err)
	}

	mas, err := view.LoadMarketState(ctx)
	if err != nil {
		return nil, xerrors.Errorf("failed to load miner actor state: %w", err)
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

func (minerStateAPI *MinerStateAPI) StateMinerPreCommitDepositForPower(ctx context.Context, maddr address.Address, pci miner.SectorPreCommitInfo, tsk block.TipSetKey) (big.Int, error) {
	store := minerStateAPI.chain.State.Store(ctx)
	ts, err := minerStateAPI.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return big.Int{}, err
	}

	root, err := minerStateAPI.chain.State.GetTipSetStateRoot(ctx, tsk)
	if err != nil {
		return big.Int{}, err
	}

	sTree, err := state.LoadState(ctx, store, root)
	if err != nil {
		return big.Int{}, err
	}

	ssize, err := pci.SealProof.SectorSize()
	if err != nil {
		return big.Int{}, xerrors.Errorf("failed to get resolve size: %w", err)
	}

	var sectorWeight abi.StoragePower
	if act, found, err := sTree.GetActor(ctx, market.Address); err != nil || !found {
		return big.Int{}, xerrors.Errorf("loading market actor %s: %w", maddr, err)
	} else if s, err := market.Load(store, act); err != nil {
		return big.Int{}, xerrors.Errorf("loading market actor state %s: %w", maddr, err)
	} else if w, vw, err := s.VerifyDealsForActivation(maddr, pci.DealIDs, ts.EnsureHeight(), pci.Expiration); err != nil {
		return big.Int{}, xerrors.Errorf("verifying deals for activation: %w", err)
	} else {
		// NB: not exactly accurate, but should always lead us to *over* estimate, not under
		duration := pci.Expiration - ts.EnsureHeight()
		sectorWeight = builtin.QAPowerForWeight(ssize, duration, w, vw)
	}

	var powerSmoothed builtin.FilterEstimate
	if act, found, err := sTree.GetActor(ctx, power.Address); err != nil || !found {
		return big.Int{}, xerrors.Errorf("loading power actor: %w", err)
	} else if s, err := power.Load(store, act); err != nil {
		return big.Int{}, xerrors.Errorf("loading power actor state: %w", err)
	} else if p, err := s.TotalPowerSmoothed(); err != nil {
		return big.Int{}, xerrors.Errorf("failed to determine total power: %w", err)
	} else {
		powerSmoothed = p
	}

	rewardActor, found, err := sTree.GetActor(ctx, reward.Address)
	if err != nil || !found {
		return big.Int{}, xerrors.Errorf("loading miner actor: %w", err)
	}

	rewardState, err := reward.Load(store, rewardActor)
	if err != nil {
		return big.Int{}, xerrors.Errorf("loading reward actor state: %w", err)
	}

	deposit, err := rewardState.PreCommitDepositForPower(powerSmoothed, sectorWeight)
	if err != nil {
		return big.Zero(), xerrors.Errorf("calculating precommit deposit: %w", err)
	}

	return big.Div(big.Mul(deposit, initialPledgeNum), initialPledgeDen), nil
}

func (minerStateAPI *MinerStateAPI) StateMinerInitialPledgeCollateral(ctx context.Context, maddr address.Address, pci miner.SectorPreCommitInfo, tsk block.TipSetKey) (big.Int, error) {
	// TODO: this repeats a lot of the previous function. Fix that.
	ts, err := minerStateAPI.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return big.Int{}, xerrors.Errorf("loading tipset %s: %w", tsk, err)
	}

	store := minerStateAPI.chain.State.Store(ctx)
	state, err := state.LoadState(ctx, store, ts.At(0).ParentStateRoot.Cid)
	if err != nil {
		return big.Int{}, xerrors.Errorf("loading state %s: %w", tsk, err)
	}

	ssize, err := pci.SealProof.SectorSize()
	if err != nil {
		return big.Int{}, xerrors.Errorf("failed to get resolve size: %w", err)
	}

	var sectorWeight abi.StoragePower
	if act, found, err := state.GetActor(ctx, market.Address); err != nil || !found {
		return big.Int{}, xerrors.Errorf("loading miner actor %s: %w", maddr, err)
	} else if s, err := market.Load(store, act); err != nil {
		return big.Int{}, xerrors.Errorf("loading market actor state %s: %w", maddr, err)
	} else if w, vw, err := s.VerifyDealsForActivation(maddr, pci.DealIDs, ts.EnsureHeight(), pci.Expiration); err != nil {
		return big.Int{}, xerrors.Errorf("verifying deals for activation: %w", err)
	} else {
		// NB: not exactly accurate, but should always lead us to *over* estimate, not under
		duration := pci.Expiration - ts.EnsureHeight()
		sectorWeight = builtin.QAPowerForWeight(ssize, duration, w, vw)
	}

	var (
		powerSmoothed    builtin.FilterEstimate
		pledgeCollateral abi.TokenAmount
	)
	if act, found, err := state.GetActor(ctx, power.Address); err != nil || !found {
		return big.Int{}, xerrors.Errorf("loading miner actor: %w", err)
	} else if s, err := power.Load(store, act); err != nil {
		return big.Int{}, xerrors.Errorf("loading power actor state: %w", err)
	} else if p, err := s.TotalPowerSmoothed(); err != nil {
		return big.Int{}, xerrors.Errorf("failed to determine total power: %w", err)
	} else if c, err := s.TotalLocked(); err != nil {
		return big.Int{}, xerrors.Errorf("failed to determine pledge collateral: %w", err)
	} else {
		powerSmoothed = p
		pledgeCollateral = c
	}

	rewardActor, found, err := state.GetActor(ctx, reward.Address)
	if err != nil || !found {
		return big.Int{}, xerrors.Errorf("loading miner actor: %w", err)
	}

	rewardState, err := reward.Load(store, rewardActor)
	if err != nil {
		return big.Int{}, xerrors.Errorf("loading reward actor state: %w", err)
	}

	circSupply, err := minerStateAPI.StateVMCirculatingSupplyInternal(ctx, ts.Key())
	if err != nil {
		return big.Zero(), xerrors.Errorf("getting circulating supply: %w", err)
	}

	initialPledge, err := rewardState.InitialPledgeForPower(
		sectorWeight,
		pledgeCollateral,
		&powerSmoothed,
		circSupply.FilCirculating,
	)
	if err != nil {
		return big.Zero(), xerrors.Errorf("calculating initial pledge: %w", err)
	}

	return big.Div(big.Mul(initialPledge, initialPledgeNum), initialPledgeDen), nil
}

func (minerStateAPI *MinerStateAPI) StateVMCirculatingSupplyInternal(ctx context.Context, tsk block.TipSetKey) (chain.CirculatingSupply, error) {
	store := minerStateAPI.chain.State.Store(ctx)
	ts, err := minerStateAPI.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return chain.CirculatingSupply{}, err
	}

	root, err := minerStateAPI.chain.State.GetTipSetStateRoot(ctx, tsk)
	if err != nil {
		return chain.CirculatingSupply{}, err
	}

	sTree, err := state.LoadState(ctx, store, root)
	if err != nil {
		return chain.CirculatingSupply{}, err
	}

	return minerStateAPI.chain.ChainReader.GetCirculatingSupplyDetailed(ctx, ts.EnsureHeight(), sTree)
}

func (minerStateAPI *MinerStateAPI) StateCirculatingSupply(ctx context.Context, tsk block.TipSetKey) (abi.TokenAmount, error) {
	return minerStateAPI.chain.ChainReader.StateCirculatingSupply(ctx, tsk)
}

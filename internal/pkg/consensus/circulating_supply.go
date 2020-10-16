package consensus

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	msig0 "github.com/filecoin-project/specs-actors/actors/builtin/multisig"
	"github.com/filecoin-project/specs-actors/actors/builtin/power"
	"github.com/filecoin-project/specs-actors/actors/builtin/reward"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/ipfs/go-cid"
	cbornode "github.com/ipfs/go-ipld-cbor"
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/build"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

type CirculatingSupply struct {
	FilVested      abi.TokenAmount
	FilMined       abi.TokenAmount
	FilBurnt       abi.TokenAmount
	FilLocked      abi.TokenAmount
	FilCirculating abi.TokenAmount
}

type genesisInfo struct {
	genesisMsigs []msig0.State
	// info about the Accounts in the genesis state
	genesisActors      []genesisActor
	genesisPledge      abi.TokenAmount
	genesisMarketFunds abi.TokenAmount
}

type genesisActor struct {
	addr    address.Address
	initBal abi.TokenAmount
}

func (sm *Expected) GetCirculatingSupplyDetailed(ctx context.Context, height abi.ChainEpoch, st state.Tree) (CirculatingSupply, error) {
	sm.genesisMsigLk.Lock()
	defer sm.genesisMsigLk.Unlock()
	if sm.preIgnitionGenInfos == nil {
		err := sm.setupPreIgnitionGenesisActorsTestnet(ctx)
		if err != nil {
			return CirculatingSupply{}, xerrors.Errorf("failed to setup pre-ignition genesis information: %v", err)
		}
	}
	if sm.postIgnitionGenInfos == nil {
		err := sm.setupPostIgnitionGenesisActors(ctx)
		if err != nil {
			return CirculatingSupply{}, xerrors.Errorf("failed to setup post-ignition genesis information: %v", err)
		}
	}

	filVested, err := sm.GetFilVested(ctx, height, st)
	if err != nil {
		return CirculatingSupply{}, xerrors.Errorf("failed to calculate filVested: %v", err)
	}

	filMined, err := GetFilMined(ctx, st)
	if err != nil {
		return CirculatingSupply{}, xerrors.Errorf("failed to calculate filMined: %v", err)
	}

	filBurnt, err := GetFilBurnt(ctx, st)
	if err != nil {
		return CirculatingSupply{}, xerrors.Errorf("failed to calculate filBurnt: %v", err)
	}

	filLocked, err := sm.GetFilLocked(ctx, st)
	if err != nil {
		return CirculatingSupply{}, xerrors.Errorf("failed to calculate filLocked: %v", err)
	}

	ret := big.Add(filVested, filMined)
	ret = big.Sub(ret, filBurnt)
	ret = big.Sub(ret, filLocked)

	if ret.LessThan(big.Zero()) {
		ret = big.Zero()
	}

	return CirculatingSupply{
		FilVested:      filVested,
		FilMined:       filMined,
		FilBurnt:       filBurnt,
		FilLocked:      filLocked,
		FilCirculating: ret,
	}, nil
}

func (c *Expected) processBlock(ctx context.Context, ts *block.TipSet) (cid.Cid, []types.MessageReceipt, error) {
	var secpMessages [][]*types.SignedMessage
	var blsMessages [][]*types.UnsignedMessage
	for i := 0; i < ts.Len(); i++ {
		blk := ts.At(i)
		secpMsgs, blsMsgs, err := c.messageStore.LoadMessages(ctx, blk.Messages.Cid)
		if err != nil {
			return cid.Undef, []types.MessageReceipt{}, xerrors.Wrapf(err, "syncing tip %s failed loading message list %s for block %s", ts.Key(), blk.Messages, blk.Cid())
		}

		blsMessages = append(blsMessages, blsMsgs)
		secpMessages = append(secpMessages, secpMsgs)
	}

	vms := vm.NewStorage(c.bstore)
	priorState, err := state.LoadState(ctx, vms, ts.At(0).StateRoot.Cid)
	if err != nil {
		return cid.Undef, []types.MessageReceipt{}, err
	}

	var newState state.Tree
	newState, receipts, err := c.runMessages(ctx, priorState, vms, ts, blsMessages, secpMessages)
	if err != nil {
		return cid.Undef, []types.MessageReceipt{}, err
	}
	err = vms.Flush()
	if err != nil {
		return cid.Undef, []types.MessageReceipt{}, err
	}

	root, err := newState.Flush(ctx)
	if err != nil {
		return cid.Undef, []types.MessageReceipt{}, err
	}
	return root, receipts, err
}

func (c *Expected) setupPreIgnitionGenesisActorsTestnet(ctx context.Context) error {
	gi := genesisInfo{}
	gb, err := c.chainState.GetGenesisBlock(ctx)
	if err != nil {
		return xerrors.Errorf("getting genesis block: %v", err)
	}
	gts, err := block.NewTipSet(gb)
	if err != nil {
		return xerrors.Errorf("getting genesis tipset: %v", err)
	}

	/*	st, _, err := c.processBlock(ctx, gts)
		if err != nil {
			return xerrors.Errorf("getting genesis tipset state: %v", err)
		}*/

	cst := cbornode.NewCborStore(c.bstore)
	sTree, err := state.LoadState(ctx, cst, gts.At(0).StateRoot.Cid)
	if err != nil {
		return xerrors.Errorf("loading state tree: %v", err)
	}

	gi.genesisMarketFunds, err = getFilMarketLocked(ctx, sTree)
	if err != nil {
		return xerrors.Errorf("setting up genesis market funds: %v", err)
	}

	gi.genesisPledge, err = getFilPowerLocked(ctx, sTree)
	if err != nil {
		return xerrors.Errorf("setting up genesis pledge: %v", err)
	}

	totalsByEpoch := make(map[abi.ChainEpoch]abi.TokenAmount)

	// 6 months
	sixMonths := abi.ChainEpoch(183 * builtin.EpochsInDay)
	totalsByEpoch[sixMonths] = big.NewInt(49_929_341)
	totalsByEpoch[sixMonths] = big.Add(totalsByEpoch[sixMonths], big.NewInt(32_787_700))

	// 1 year
	oneYear := abi.ChainEpoch(365 * builtin.EpochsInDay)
	totalsByEpoch[oneYear] = big.NewInt(22_421_712)

	// 2 years
	twoYears := abi.ChainEpoch(2 * 365 * builtin.EpochsInDay)
	totalsByEpoch[twoYears] = big.NewInt(7_223_364)

	// 3 years
	threeYears := abi.ChainEpoch(3 * 365 * builtin.EpochsInDay)
	totalsByEpoch[threeYears] = big.NewInt(87_637_883)

	// 6 years
	sixYears := abi.ChainEpoch(6 * 365 * builtin.EpochsInDay)
	totalsByEpoch[sixYears] = big.NewInt(100_000_000)
	totalsByEpoch[sixYears] = big.Add(totalsByEpoch[sixYears], big.NewInt(300_000_000))

	gi.genesisMsigs = make([]msig0.State, 0, len(totalsByEpoch))
	for k, v := range totalsByEpoch {
		ns := msig0.State{
			InitialBalance: v,
			UnlockDuration: k,
			PendingTxns:    cid.Undef,
		}
		gi.genesisMsigs = append(gi.genesisMsigs, ns)
	}

	c.preIgnitionGenInfos = &gi

	return nil
}

// sets up information about the actors in the genesis state, post the ignition fork
func (sm *Expected) setupPostIgnitionGenesisActors(ctx context.Context) error {

	gi := genesisInfo{}

	gb, err := sm.chainState.GetGenesisBlock(ctx)
	if err != nil {
		return xerrors.Errorf("getting genesis block: %v", err)
	}

	gts, err := block.NewTipSet(gb)
	if err != nil {
		return xerrors.Errorf("getting genesis tipset: %v", err)
	}

	/*st, _, err := sm.processBlock(ctx, gts)
	if err != nil {
		return xerrors.Errorf("getting genesis tipset state: %v", err)
	}*/

	cst := cbornode.NewCborStore(sm.bstore)
	sTree, err := state.LoadState(ctx, cst, gts.At(0).StateRoot.Cid)
	if err != nil {
		return xerrors.Errorf("loading state tree: %v", err)
	}

	// Unnecessary, should be removed
	gi.genesisMarketFunds, err = getFilMarketLocked(ctx, sTree)
	if err != nil {
		return xerrors.Errorf("setting up genesis market funds: %v", err)
	}

	// Unnecessary, should be removed
	gi.genesisPledge, err = getFilPowerLocked(ctx, sTree)
	if err != nil {
		return xerrors.Errorf("setting up genesis pledge: %v", err)
	}

	totalsByEpoch := make(map[abi.ChainEpoch]abi.TokenAmount)

	// 6 months
	sixMonths := abi.ChainEpoch(183 * builtin.EpochsInDay)
	totalsByEpoch[sixMonths] = big.NewInt(49_929_341)
	totalsByEpoch[sixMonths] = big.Add(totalsByEpoch[sixMonths], big.NewInt(32_787_700))

	// 1 year
	oneYear := abi.ChainEpoch(365 * builtin.EpochsInDay)
	totalsByEpoch[oneYear] = big.NewInt(22_421_712)

	// 2 years
	twoYears := abi.ChainEpoch(2 * 365 * builtin.EpochsInDay)
	totalsByEpoch[twoYears] = big.NewInt(7_223_364)

	// 3 years
	threeYears := abi.ChainEpoch(3 * 365 * builtin.EpochsInDay)
	totalsByEpoch[threeYears] = big.NewInt(87_637_883)

	// 6 years
	sixYears := abi.ChainEpoch(6 * 365 * builtin.EpochsInDay)
	totalsByEpoch[sixYears] = big.NewInt(100_000_000)
	totalsByEpoch[sixYears] = big.Add(totalsByEpoch[sixYears], big.NewInt(300_000_000))

	gi.genesisMsigs = make([]msig0.State, 0, len(totalsByEpoch))
	for k, v := range totalsByEpoch {
		ns := msig0.State{
			// In the pre-ignition logic, we incorrectly set this value in Fil, not attoFil, an off-by-10^18 error
			InitialBalance: big.Mul(v, big.NewInt(int64(build.FilecoinPrecision))),
			UnlockDuration: k,
			PendingTxns:    cid.Undef,
			// In the pre-ignition logic, the start epoch was 0. This changes in the fork logic of the Ignition upgrade itself.
			StartEpoch: 148888, //todo add by force should chnge after fork done
		}
		gi.genesisMsigs = append(gi.genesisMsigs, ns)
	}

	sm.postIgnitionGenInfos = &gi

	return nil
}

func GetFilMined(ctx context.Context, st state.Tree) (abi.TokenAmount, error) {
	ractor, found, err := st.GetActor(ctx, builtin.RewardActorAddr)
	if !found || err != nil {
		return big.Zero(), xerrors.Errorf("failed to load reward actor state: %v", err)
	}

	var rst reward.State
	if err := adt.WrapStore(ctx, st.GetStore()).Get(ctx, ractor.Head.Cid, &rst); err != nil {
		return big.Zero(), xerrors.Errorf("failed to load reward state: %v", err)
	}

	return rst.TotalMined, nil
}

func getFilMarketLocked(ctx context.Context, st state.Tree) (abi.TokenAmount, error) {
	act, found, err := st.GetActor(ctx, builtin.StorageMarketActorAddr)
	if !found || err != nil {
		return big.Zero(), xerrors.Errorf("failed to load market actor: %v", err)
	}

	var mst market.State
	if err := adt.WrapStore(ctx, st.GetStore()).Get(ctx, act.Head.Cid, &mst); err != nil {
		return big.Zero(), xerrors.Errorf("failed to load reward state: %v", err)
	}

	fml := big.Add(mst.TotalClientLockedCollateral, mst.TotalProviderLockedCollateral)
	fml = big.Add(fml, mst.TotalClientStorageFee)
	return fml, nil
}

func getFilPowerLocked(ctx context.Context, st state.Tree) (abi.TokenAmount, error) {
	act, found, err := st.GetActor(ctx, builtin.StoragePowerActorAddr)
	if !found || err != nil {
		return big.Zero(), xerrors.Errorf("failed to load power actor: %v", err)
	}
	var pst power.State
	if err := adt.WrapStore(ctx, st.GetStore()).Get(ctx, act.Head.Cid, &pst); err != nil {
		return big.Zero(), xerrors.Errorf("failed to load power state: %v", err)
	}

	return pst.TotalPledgeCollateral, nil
}

func GetFilBurnt(ctx context.Context, st state.Tree) (abi.TokenAmount, error) {
	burnt, found, err := st.GetActor(ctx, builtin.BurntFundsActorAddr)
	if !found || err != nil {
		return big.Zero(), xerrors.Errorf("failed to load burnt actor: %v", err)
	}

	return burnt.Balance, nil
}
func (sm *Expected) GetFilVested(ctx context.Context, height abi.ChainEpoch, st state.Tree) (abi.TokenAmount, error) {
	vf := big.Zero()
	if height <= 94000 { //todo add by force should chnge after fork done
		for _, v := range sm.preIgnitionGenInfos.genesisMsigs {
			au := big.Sub(v.InitialBalance, v.AmountLocked(height))
			vf = big.Add(vf, au)
		}
	} else {
		for _, v := range sm.postIgnitionGenInfos.genesisMsigs {
			// In the pre-ignition logic, we simply called AmountLocked(height), assuming startEpoch was 0.
			// The start epoch changed in the Ignition upgrade.
			au := big.Sub(v.InitialBalance, v.AmountLocked(height-v.StartEpoch))
			vf = big.Add(vf, au)
		}
	}

	// there should not be any such accounts in testnet (and also none in mainnet?)
	// continue to use preIgnitionGenInfos, nothing changed at the Ignition epoch
	for _, v := range sm.preIgnitionGenInfos.genesisActors {
		act, found, err := st.GetActor(ctx, v.addr)
		if !found || err != nil {
			return big.Zero(), xerrors.Errorf("failed to get actor: %v", err)
		}

		diff := big.Sub(v.initBal, act.Balance)
		if diff.GreaterThan(big.Zero()) {
			vf = big.Add(vf, diff)
		}
	}

	// continue to use preIgnitionGenInfos, nothing changed at the Ignition epoch
	vf = big.Add(vf, sm.preIgnitionGenInfos.genesisPledge)
	// continue to use preIgnitionGenInfos, nothing changed at the Ignition epoch
	vf = big.Add(vf, sm.preIgnitionGenInfos.genesisMarketFunds)

	return vf, nil
}

func (sm *Expected) GetFilLocked(ctx context.Context, st state.Tree) (abi.TokenAmount, error) {

	filMarketLocked, err := getFilMarketLocked(ctx, st)
	if err != nil {
		return big.Zero(), xerrors.Errorf("failed to get filMarketLocked: %v", err)
	}

	filPowerLocked, err := getFilPowerLocked(ctx, st)
	if err != nil {
		return big.Zero(), xerrors.Errorf("failed to get filPowerLocked: %v", err)
	}

	return big.Add(filMarketLocked, filPowerLocked), nil
}

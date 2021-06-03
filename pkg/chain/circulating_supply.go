package chain

import (
	"context"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/types"
	"sync"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	cbornode "github.com/ipfs/go-ipld-cbor"
	xerrors "github.com/pkg/errors"

	// Used for genesis.
	msig0 "github.com/filecoin-project/specs-actors/actors/builtin/multisig"

	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/specactors/adt"
	"github.com/filecoin-project/venus/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/market"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/power"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/reward"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
)

type genesisReader interface {
	GetGenesisBlock(ctx context.Context) (*types.BlockHeader, error)
}

type CirculatingSupply struct {
	FilVested      abi.TokenAmount
	FilMined       abi.TokenAmount
	FilBurnt       abi.TokenAmount
	FilLocked      abi.TokenAmount
	FilCirculating abi.TokenAmount
}

type CirculatingSupplyCalculator struct {
	bstore        blockstoreutil.Blockstore
	genesisReader genesisReader

	// info about the Accounts in the genesis state
	preIgnitionVesting  []msig0.State
	postIgnitionVesting []msig0.State
	postCalicoVesting   []msig0.State

	genesisPledge      abi.TokenAmount
	genesisMarketFunds abi.TokenAmount

	genesisMsigLk sync.Mutex
	upgradeConfig *config.ForkUpgradeConfig
}

func NewCirculatingSupplyCalculator(bstore blockstoreutil.Blockstore, genesisReader genesisReader, upgradeConfig *config.ForkUpgradeConfig) *CirculatingSupplyCalculator {
	return &CirculatingSupplyCalculator{bstore: bstore, genesisReader: genesisReader, upgradeConfig: upgradeConfig}
}

func (caculator *CirculatingSupplyCalculator) GetCirculatingSupplyDetailed(ctx context.Context, height abi.ChainEpoch, st tree.Tree) (CirculatingSupply, error) {
	caculator.genesisMsigLk.Lock()
	defer caculator.genesisMsigLk.Unlock()
	if caculator.preIgnitionVesting == nil || caculator.genesisPledge.IsZero() || caculator.genesisMarketFunds.IsZero() {
		err := caculator.setupGenesisVestingSchedule(ctx)
		if err != nil {
			return CirculatingSupply{}, xerrors.Errorf("failed to setup pre-ignition vesting schedule: %v", err)
		}
	}
	if caculator.postIgnitionVesting == nil {
		err := caculator.setupPostIgnitionVesting(ctx)
		if err != nil {
			return CirculatingSupply{}, xerrors.Errorf("failed to setup post-ignition vesting schedule: %v", err)
		}
	}
	if caculator.postCalicoVesting == nil {
		err := caculator.setupPostCalicoVesting(ctx)
		if err != nil {
			return CirculatingSupply{}, xerrors.Errorf("failed to setup post-calico vesting schedule: %v", err)
		}
	}

	filVested, err := caculator.GetFilVested(ctx, height, st)
	if err != nil {
		return CirculatingSupply{}, xerrors.Errorf("failed to calculate filVested: %v", err)
	}

	filReserveDisbursed := big.Zero()
	if height > caculator.upgradeConfig.UpgradeAssemblyHeight {
		filReserveDisbursed, err = caculator.GetFilReserveDisbursed(ctx, st)
		if err != nil {
			return CirculatingSupply{}, xerrors.Errorf("failed to calculate filReserveDisbursed: %v", err)
		}
	}

	filMined, err := GetFilMined(ctx, st)
	if err != nil {
		return CirculatingSupply{}, xerrors.Errorf("failed to calculate filMined: %v", err)
	}
	filBurnt, err := GetFilBurnt(ctx, st)
	if err != nil {
		return CirculatingSupply{}, xerrors.Errorf("failed to calculate filBurnt: %v", err)
	}
	filLocked, err := caculator.GetFilLocked(ctx, st)
	if err != nil {
		return CirculatingSupply{}, xerrors.Errorf("failed to calculate filLocked: %v", err)
	}
	ret := big.Add(filVested, filMined)
	ret = big.Add(ret, filReserveDisbursed)
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

/*func (c *Expected) processBlock(ctx context.Context, ts *block.TipSet) (cid.Cid, []types.MessageReceipt, error) {
	var secpMessages [][]*types.SignedMessage
	var blsMessages [][]*types.UnsignedMessage
	for i := 0; i < ts.Len(); i++ {
		blk := ts.At(i)
		secpMsgs, blsMsgs, err := c.messageStore.LoadMetaMessages(ctx, blk.Messages.Cid)
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
*/

// sets up information about the vesting schedule
func (caculator *CirculatingSupplyCalculator) setupGenesisVestingSchedule(ctx context.Context) error {

	gb, err := caculator.genesisReader.GetGenesisBlock(ctx)
	if err != nil {
		return xerrors.Errorf("getting genesis block: %v", err)
	}

	gts, err := types.NewTipSet(gb)
	if err != nil {
		return xerrors.Errorf("getting genesis tipset: %v", err)
	}

	cst := cbornode.NewCborStore(caculator.bstore)
	sTree, err := tree.LoadState(ctx, cst, gts.At(0).ParentStateRoot)
	if err != nil {
		return xerrors.Errorf("loading state tree: %v", err)
	}

	gmf, err := getFilMarketLocked(ctx, sTree)
	if err != nil {
		return xerrors.Errorf("setting up genesis market funds: %v", err)
	}

	gp, err := getFilPowerLocked(ctx, sTree)
	if err != nil {
		return xerrors.Errorf("setting up genesis pledge: %v", err)
	}

	caculator.genesisMarketFunds = gmf
	caculator.genesisPledge = gp

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

	caculator.preIgnitionVesting = make([]msig0.State, 0, len(totalsByEpoch))
	for k, v := range totalsByEpoch {
		ns := msig0.State{
			InitialBalance: v,
			UnlockDuration: k,
			PendingTxns:    cid.Undef,
		}
		caculator.preIgnitionVesting = append(caculator.preIgnitionVesting, ns)
	}

	return nil
}

// sets up information about the vesting schedule post the ignition upgrade
func (caculator *CirculatingSupplyCalculator) setupPostIgnitionVesting(ctx context.Context) error {

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

	caculator.postIgnitionVesting = make([]msig0.State, 0, len(totalsByEpoch))
	for k, v := range totalsByEpoch {
		ns := msig0.State{
			// In the pre-ignition logic, we incorrectly set this value in Fil, not attoFil, an off-by-10^18 error
			InitialBalance: big.Mul(v, big.NewInt(int64(constants.FilecoinPrecision))),
			UnlockDuration: k,
			PendingTxns:    cid.Undef,
			// In the pre-ignition logic, the start epoch was 0. This changes in the fork logic of the Ignition upgrade itself.
			StartEpoch: caculator.upgradeConfig.UpgradeLiftoffHeight,
		}
		caculator.postIgnitionVesting = append(caculator.postIgnitionVesting, ns)
	}

	return nil
}

// sets up information about the vesting schedule post the calico upgrade
func (caculator *CirculatingSupplyCalculator) setupPostCalicoVesting(ctx context.Context) error {

	totalsByEpoch := make(map[abi.ChainEpoch]abi.TokenAmount)

	// 0 days
	zeroDays := abi.ChainEpoch(0)
	totalsByEpoch[zeroDays] = big.NewInt(10_632_000)

	// 6 months
	sixMonths := abi.ChainEpoch(183 * builtin.EpochsInDay)
	totalsByEpoch[sixMonths] = big.NewInt(19_015_887)
	totalsByEpoch[sixMonths] = big.Add(totalsByEpoch[sixMonths], big.NewInt(32_787_700))

	// 1 year
	oneYear := abi.ChainEpoch(365 * builtin.EpochsInDay)
	totalsByEpoch[oneYear] = big.NewInt(22_421_712)
	totalsByEpoch[oneYear] = big.Add(totalsByEpoch[oneYear], big.NewInt(9_400_000))

	// 2 years
	twoYears := abi.ChainEpoch(2 * 365 * builtin.EpochsInDay)
	totalsByEpoch[twoYears] = big.NewInt(7_223_364)

	// 3 years
	threeYears := abi.ChainEpoch(3 * 365 * builtin.EpochsInDay)
	totalsByEpoch[threeYears] = big.NewInt(87_637_883)
	totalsByEpoch[threeYears] = big.Add(totalsByEpoch[threeYears], big.NewInt(898_958))

	// 6 years
	sixYears := abi.ChainEpoch(6 * 365 * builtin.EpochsInDay)
	totalsByEpoch[sixYears] = big.NewInt(100_000_000)
	totalsByEpoch[sixYears] = big.Add(totalsByEpoch[sixYears], big.NewInt(300_000_000))
	totalsByEpoch[sixYears] = big.Add(totalsByEpoch[sixYears], big.NewInt(9_805_053))

	caculator.postCalicoVesting = make([]msig0.State, 0, len(totalsByEpoch))
	for k, v := range totalsByEpoch {
		ns := msig0.State{
			InitialBalance: big.Mul(v, big.NewInt(int64(constants.FilecoinPrecision))),
			UnlockDuration: k,
			PendingTxns:    cid.Undef,
			StartEpoch:     caculator.upgradeConfig.UpgradeLiftoffHeight,
		}
		caculator.postCalicoVesting = append(caculator.postCalicoVesting, ns)
	}

	return nil
}

// GetVestedFunds returns all funds that have "left" actors that are in the genesis state:
// - For Multisigs, it counts the actual amounts that have vested at the given epoch
// - For Accounts, it counts max(currentBalance - genesisBalance, 0).
func (caculator *CirculatingSupplyCalculator) GetFilVested(ctx context.Context, height abi.ChainEpoch, st tree.Tree) (abi.TokenAmount, error) {
	vf := big.Zero()
	if height <= caculator.upgradeConfig.UpgradeIgnitionHeight {
		for _, v := range caculator.preIgnitionVesting {
			au := big.Sub(v.InitialBalance, v.AmountLocked(height))
			vf = big.Add(vf, au)
		}
	} else if height <= caculator.upgradeConfig.UpgradeCalicoHeight {
		for _, v := range caculator.postIgnitionVesting {
			// In the pre-ignition logic, we simply called AmountLocked(height), assuming startEpoch was 0.
			// The start epoch changed in the Ignition upgrade.
			au := big.Sub(v.InitialBalance, v.AmountLocked(height-v.StartEpoch))
			vf = big.Add(vf, au)
		}
	} else {
		for _, v := range caculator.postCalicoVesting {
			// In the pre-ignition logic, we simply called AmountLocked(height), assuming startEpoch was 0.
			// The start epoch changed in the Ignition upgrade.
			au := big.Sub(v.InitialBalance, v.AmountLocked(height-v.StartEpoch))
			vf = big.Add(vf, au)
		}
	}

	// After UpgradeAssemblyHeight these funds are accounted for in GetFilReserveDisbursed
	if height <= caculator.upgradeConfig.UpgradeAssemblyHeight {
		// continue to use preIgnitionGenInfos, nothing changed at the Ignition epoch
		vf = big.Add(vf, caculator.genesisPledge)
		// continue to use preIgnitionGenInfos, nothing changed at the Ignition epoch
		vf = big.Add(vf, caculator.genesisMarketFunds)
	}

	return vf, nil
}

func (caculator *CirculatingSupplyCalculator) GetFilReserveDisbursed(ctx context.Context, st tree.Tree) (abi.TokenAmount, error) {
	ract, found, err := st.GetActor(ctx, builtin.ReserveAddress)
	if !found || err != nil {
		return big.Zero(), xerrors.Errorf("failed to get reserve actor: %v", err)
	}

	// If money enters the reserve actor, this could lead to a negative term
	return big.Sub(big.NewFromGo(constants.InitialFilReserved), ract.Balance), nil
}

func GetFilMined(ctx context.Context, st tree.Tree) (abi.TokenAmount, error) {
	ractor, found, err := st.GetActor(ctx, reward.Address)
	if !found || err != nil {
		return big.Zero(), xerrors.Errorf("failed to load reward actor state: %v", err)
	}

	rst, err := reward.Load(adt.WrapStore(ctx, st.GetStore()), ractor)
	if err != nil {
		return big.Zero(), err
	}

	return rst.TotalStoragePowerReward()
}

func GetFilBurnt(ctx context.Context, st tree.Tree) (abi.TokenAmount, error) {
	burnt, found, err := st.GetActor(ctx, builtin.BurntFundsActorAddr)
	if !found || err != nil {
		return big.Zero(), xerrors.Errorf("failed to load burnt actor: %v", err)
	}

	return burnt.Balance, nil
}

func (caculator *CirculatingSupplyCalculator) GetFilLocked(ctx context.Context, st tree.Tree) (abi.TokenAmount, error) {

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

func getFilMarketLocked(ctx context.Context, st tree.Tree) (abi.TokenAmount, error) {
	act, found, err := st.GetActor(ctx, market.Address)
	if !found || err != nil {
		return big.Zero(), xerrors.Errorf("failed to load market actor: %v", err)
	}

	mst, err := market.Load(adt.WrapStore(ctx, st.GetStore()), act)
	if err != nil {
		return big.Zero(), xerrors.Errorf("failed to load market state: %v", err)
	}

	return mst.TotalLocked()
}

func getFilPowerLocked(ctx context.Context, st tree.Tree) (abi.TokenAmount, error) {
	pactor, found, err := st.GetActor(ctx, power.Address)
	if !found || err != nil {
		return big.Zero(), xerrors.Errorf("failed to load power actor: %v", err)
	}

	pst, err := power.Load(adt.WrapStore(ctx, st.GetStore()), pactor)
	if err != nil {
		return big.Zero(), xerrors.Errorf("failed to load power state: %v", err)
	}

	return pst.TotalLocked()
}

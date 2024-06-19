package chain

import (
	"context"
	"fmt"
	"sync"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	cbornode "github.com/ipfs/go-ipld-cbor"

	// Used for genesis.
	msig0 "github.com/filecoin-project/specs-actors/actors/builtin/multisig"

	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	_init "github.com/filecoin-project/venus/venus-shared/actors/builtin/init"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/market"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/multisig"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/power"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/reward"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/verifreg"
	blockstoreutil "github.com/filecoin-project/venus/venus-shared/blockstore"
)

type GetNetworkVersionFunc func(ctx context.Context, height abi.ChainEpoch) network.Version

type ICirculatingSupplyCalcualtor interface {
	GetCirculatingSupplyDetailed(ctx context.Context, height abi.ChainEpoch, st tree.Tree) (types.CirculatingSupply, error)
	GetFilVested(ctx context.Context, height abi.ChainEpoch) (abi.TokenAmount, error)
	GetCirculatingSupply(ctx context.Context, height abi.ChainEpoch, st tree.Tree) (abi.TokenAmount, error)
}

// CirculatingSupplyCalculator used to calculate the funds at a specific block height
type CirculatingSupplyCalculator struct {
	bstore      blockstoreutil.Blockstore
	genesisRoot cid.Cid

	// info about the Accounts in the genesis state
	preIgnitionVesting  []msig0.State
	postIgnitionVesting []msig0.State
	postCalicoVesting   []msig0.State

	genesisPledge abi.TokenAmount

	genesisMsigLk     sync.Mutex
	upgradeConfig     *config.ForkUpgradeConfig
	getNetworkVersion GetNetworkVersionFunc
}

// NewCirculatingSupplyCalculator create new  circulating supply calculator
func NewCirculatingSupplyCalculator(bstore blockstoreutil.Blockstore,
	genesisRoot cid.Cid,
	upgradeConfig *config.ForkUpgradeConfig,
	getNetworkVersion GetNetworkVersionFunc,
) *CirculatingSupplyCalculator {
	return &CirculatingSupplyCalculator{
		bstore:            bstore,
		genesisRoot:       genesisRoot,
		upgradeConfig:     upgradeConfig,
		getNetworkVersion: getNetworkVersion,
	}
}

// GetCirculatingSupplyDetailed query contract and calculate circulation status at specific height and tree state
func (caculator *CirculatingSupplyCalculator) GetCirculatingSupplyDetailed(ctx context.Context, height abi.ChainEpoch, st tree.Tree) (types.CirculatingSupply, error) {
	nv := caculator.getNetworkVersion(ctx, height)
	filVested, err := caculator.GetFilVested(ctx, height)
	if err != nil {
		return types.CirculatingSupply{}, fmt.Errorf("failed to calculate filVested: %v", err)
	}

	filReserveDisbursed := big.Zero()
	if height > caculator.upgradeConfig.UpgradeAssemblyHeight {
		filReserveDisbursed, err = caculator.GetFilReserveDisbursed(ctx, st)
		if err != nil {
			return types.CirculatingSupply{}, fmt.Errorf("failed to calculate filReserveDisbursed: %v", err)
		}
	}

	filMined, err := GetFilMined(ctx, st)
	if err != nil {
		return types.CirculatingSupply{}, fmt.Errorf("failed to calculate filMined: %v", err)
	}
	filBurnt, err := GetFilBurnt(ctx, st)
	if err != nil {
		return types.CirculatingSupply{}, fmt.Errorf("failed to calculate filBurnt: %v", err)
	}
	filLocked, err := caculator.GetFilLocked(ctx, st, nv)
	if err != nil {
		return types.CirculatingSupply{}, fmt.Errorf("failed to calculate filLocked: %v", err)
	}
	ret := big.Add(filVested, filMined)
	ret = big.Add(ret, filReserveDisbursed)
	ret = big.Sub(ret, filBurnt)
	ret = big.Sub(ret, filLocked)

	if ret.LessThan(big.Zero()) {
		ret = big.Zero()
	}

	return types.CirculatingSupply{
		FilVested:           filVested,
		FilMined:            filMined,
		FilBurnt:            filBurnt,
		FilLocked:           filLocked,
		FilCirculating:      ret,
		FilReserveDisbursed: filReserveDisbursed,
	}, nil
}

func (caculator *CirculatingSupplyCalculator) GetCirculatingSupply(ctx context.Context, height abi.ChainEpoch, st tree.Tree) (abi.TokenAmount, error) {
	adtStore := adt.WrapStore(ctx, cbor.NewCborStore(caculator.bstore))
	circ := big.Zero()
	unCirc := big.Zero()
	nv := caculator.getNetworkVersion(ctx, height)
	err := st.ForEach(func(a address.Address, actor *types.Actor) error {
		// this can be a lengthy operation, we need to cancel early when
		// the context is cancelled to avoid resource exhaustion
		select {
		case <-ctx.Done():
			// this will cause ForEach to return
			return ctx.Err()
		default:
		}

		switch {
		case actor.Balance.IsZero():
			// Do nothing for zero-balance actors
			break
		case a == _init.Address ||
			a == reward.Address ||
			a == verifreg.Address ||
			// The power actor itself should never receive funds
			a == power.Address ||
			a == builtin.SystemActorAddr ||
			a == builtin.CronActorAddr ||
			a == builtin.BurntFundsActorAddr ||
			a == builtin.SaftAddress ||
			a == builtin.ReserveAddress ||
			a == builtin.EthereumAddressManagerActorAddr:

			unCirc = big.Add(unCirc, actor.Balance)

		case a == market.Address:
			if nv >= network.Version23 {
				circ = big.Add(circ, actor.Balance)
			} else {
				mst, err := market.Load(adtStore, actor)
				if err != nil {
					return err
				}

				lb, err := mst.TotalLocked()
				if err != nil {
					return err
				}

				circ = big.Add(circ, big.Sub(actor.Balance, lb))
				unCirc = big.Add(unCirc, lb)
			}

		case builtin.IsAccountActor(actor.Code) ||
			builtin.IsPaymentChannelActor(actor.Code) ||
			builtin.IsEthAccountActor(actor.Code) ||
			builtin.IsEvmActor(actor.Code) ||
			builtin.IsPlaceholderActor(actor.Code):

			circ = big.Add(circ, actor.Balance)

		case builtin.IsStorageMinerActor(actor.Code):
			mst, err := miner.Load(adtStore, actor)
			if err != nil {
				return err
			}

			ab, err := mst.AvailableBalance(actor.Balance)

			if err == nil {
				circ = big.Add(circ, ab)
				unCirc = big.Add(unCirc, big.Sub(actor.Balance, ab))
			} else {
				// Assume any error is because the miner state is "broken" (lower actor balance than locked funds)
				// In this case, the actor's entire balance is considered "uncirculating"
				unCirc = big.Add(unCirc, actor.Balance)
			}

		case builtin.IsMultisigActor(actor.Code):
			mst, err := multisig.Load(adtStore, actor)
			if err != nil {
				return err
			}

			lb, err := mst.LockedBalance(height)
			if err != nil {
				return err
			}

			ab := big.Sub(actor.Balance, lb)
			circ = big.Add(circ, big.Max(ab, big.Zero()))
			unCirc = big.Add(unCirc, big.Min(actor.Balance, lb))
		default:
			return fmt.Errorf("unexpected actor: %s", a)
		}

		return nil
	})
	if err != nil {
		return abi.TokenAmount{}, err
	}

	total := big.Add(circ, unCirc)
	if !total.Equals(types.TotalFilecoinInt) {
		return abi.TokenAmount{}, fmt.Errorf("total filecoin didn't add to expected amount: %s != %s", total, types.TotalFilecoinInt)
	}

	return circ, nil
}

// sets up information about the vesting schedule
func (caculator *CirculatingSupplyCalculator) setupGenesisVestingSchedule(ctx context.Context) error {
	cst := cbornode.NewCborStore(caculator.bstore)
	sTree, err := tree.LoadState(ctx, cst, caculator.genesisRoot)
	if err != nil {
		return fmt.Errorf("loading state tree: %v", err)
	}

	gp, err := getFilPowerLocked(ctx, sTree)
	if err != nil {
		return fmt.Errorf("setting up genesis pledge: %v", err)
	}

	caculator.genesisMsigLk.Lock()
	defer caculator.genesisMsigLk.Unlock()
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
func (caculator *CirculatingSupplyCalculator) setupPostIgnitionVesting(_ context.Context) error {
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

	caculator.genesisMsigLk.Lock()
	defer caculator.genesisMsigLk.Unlock()
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
func (caculator *CirculatingSupplyCalculator) setupPostCalicoVesting(_ context.Context) error {
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

	caculator.genesisMsigLk.Lock()
	defer caculator.genesisMsigLk.Unlock()

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

// GetFilVested returns all funds that have "left" actors that are in the genesis state:
// - For Multisigs, it counts the actual amounts that have vested at the given epoch
// - For Accounts, it counts max(currentBalance - genesisBalance, 0).
func (caculator *CirculatingSupplyCalculator) GetFilVested(ctx context.Context, height abi.ChainEpoch) (abi.TokenAmount, error) {
	vf := big.Zero()

	// TODO: combine all this?
	if caculator.preIgnitionVesting == nil || caculator.genesisPledge.IsZero() {
		err := caculator.setupGenesisVestingSchedule(ctx)
		if err != nil {
			return vf, fmt.Errorf("failed to setup pre-ignition vesting schedule: %w", err)
		}
	}
	if caculator.postIgnitionVesting == nil {
		err := caculator.setupPostIgnitionVesting(ctx)
		if err != nil {
			return vf, fmt.Errorf("failed to setup post-ignition vesting schedule: %w", err)
		}
	}
	if caculator.postCalicoVesting == nil {
		err := caculator.setupPostCalicoVesting(ctx)
		if err != nil {
			return vf, fmt.Errorf("failed to setup post-calico vesting schedule: %w", err)
		}
	}

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
	}

	return vf, nil
}

func (caculator *CirculatingSupplyCalculator) GetFilReserveDisbursed(ctx context.Context, st tree.Tree) (abi.TokenAmount, error) {
	ract, found, err := st.GetActor(ctx, builtin.ReserveAddress)
	if !found || err != nil {
		return big.Zero(), fmt.Errorf("failed to get reserve actor: %v", err)
	}

	// If money enters the reserve actor, this could lead to a negative term
	return big.Sub(big.NewFromGo(constants.InitialFilReserved), ract.Balance), nil
}

// GetFilMined query reward contract to get amount of mined fil
func GetFilMined(ctx context.Context, st tree.Tree) (abi.TokenAmount, error) {
	ractor, found, err := st.GetActor(ctx, reward.Address)
	if !found || err != nil {
		return big.Zero(), fmt.Errorf("failed to load reward actor state: %v", err)
	}

	rst, err := reward.Load(adt.WrapStore(ctx, st.GetStore()), ractor)
	if err != nil {
		return big.Zero(), err
	}

	return rst.TotalStoragePowerReward()
}

// GetFilBurnt query burnt contract to get amount of burnt fil
func GetFilBurnt(ctx context.Context, st tree.Tree) (abi.TokenAmount, error) {
	burnt, found, err := st.GetActor(ctx, builtin.BurntFundsActorAddr)
	if !found || err != nil {
		return big.Zero(), fmt.Errorf("failed to load burnt actor: %v", err)
	}

	return burnt.Balance, nil
}

// GetFilLocked query the market contract and power contract to get the amount of locked fils
func (caculator *CirculatingSupplyCalculator) GetFilLocked(ctx context.Context, st tree.Tree, nv network.Version) (abi.TokenAmount, error) {
	if nv >= network.Version23 {
		return getFilPowerLocked(ctx, st)
	}

	filMarketLocked, err := getFilMarketLocked(ctx, st)
	if err != nil {
		return big.Zero(), fmt.Errorf("failed to get filMarketLocked: %v", err)
	}

	filPowerLocked, err := getFilPowerLocked(ctx, st)
	if err != nil {
		return big.Zero(), fmt.Errorf("failed to get filPowerLocked: %v", err)
	}

	return big.Add(filMarketLocked, filPowerLocked), nil
}

func getFilMarketLocked(ctx context.Context, st tree.Tree) (abi.TokenAmount, error) {
	act, found, err := st.GetActor(ctx, market.Address)
	if !found || err != nil {
		return big.Zero(), fmt.Errorf("failed to load market actor: %v", err)
	}

	mst, err := market.Load(adt.WrapStore(ctx, st.GetStore()), act)
	if err != nil {
		return big.Zero(), fmt.Errorf("failed to load market state: %v", err)
	}

	return mst.TotalLocked()
}

func getFilPowerLocked(ctx context.Context, st tree.Tree) (abi.TokenAmount, error) {
	pactor, found, err := st.GetActor(ctx, power.Address)
	if !found || err != nil {
		return big.Zero(), fmt.Errorf("failed to load power actor: %v", err)
	}

	pst, err := power.Load(adt.WrapStore(ctx, st.GetStore()), pactor)
	if err != nil {
		return big.Zero(), fmt.Errorf("failed to load power state: %v", err)
	}

	return pst.TotalLocked()
}

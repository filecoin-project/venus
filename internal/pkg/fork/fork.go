package fork

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"os"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/specs-actors/actors/migration/nv3"
	m2 "github.com/filecoin-project/specs-actors/v2/actors/migration"

	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
	miner0 "github.com/filecoin-project/specs-actors/actors/builtin/miner"
	multisig0 "github.com/filecoin-project/specs-actors/actors/builtin/multisig"
	power0 "github.com/filecoin-project/specs-actors/actors/builtin/power"
	adt0 "github.com/filecoin-project/specs-actors/actors/util/adt"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/enccid"
	bstore "github.com/filecoin-project/venus/internal/pkg/fork/blockstore"
	"github.com/filecoin-project/venus/internal/pkg/fork/bufbstore"
	"github.com/filecoin-project/venus/internal/pkg/params"
	"github.com/filecoin-project/venus/internal/pkg/specactors/adt"
	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin"
	init_ "github.com/filecoin-project/venus/internal/pkg/specactors/builtin/init"
	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin/multisig"
	"github.com/filecoin-project/venus/internal/pkg/specactors/policy"
	"github.com/filecoin-project/venus/internal/pkg/types"

	vmstate "github.com/filecoin-project/venus/internal/pkg/vm/state"
)

var log = logging.Logger("fork")

var (
	UpgradeSmokeHeight    = abi.ChainEpoch(-1)
	UpgradeBreezeHeight   = abi.ChainEpoch(0)
	UpgradeIgnitionHeight = abi.ChainEpoch(0)
	UpgradeLiftoffHeight  = abi.ChainEpoch(0)
	UpgradeActorsV2Height = abi.ChainEpoch(0)
	UpgradeRefuelHeight   = abi.ChainEpoch(0)
	UpgradeTapeHeight     = abi.ChainEpoch(0)
	UpgradeKumquatHeight  = abi.ChainEpoch(0)

	BreezeGasTampingDuration = abi.ChainEpoch(0)
)

func SetAddressNetwork(n address.Network) {
	address.CurrentNetwork = n
}

func init() {
	UpgradeBreezeHeight = 41280
	BreezeGasTampingDuration = 120
	UpgradeSmokeHeight = 51000
	UpgradeIgnitionHeight = 94000
	UpgradeRefuelHeight = 130800
	UpgradeActorsV2Height = 138720
	UpgradeTapeHeight = 140760
	// This signals our tentative epoch for mainnet launch. Can make it later, but not earlier.
	// Miners, clients, developers, custodians all need time to prepare.
	// We still have upgrades and state changes to do, but can happen after signaling timing here.
	UpgradeLiftoffHeight = 148888
	UpgradeKumquatHeight = 170000

	policy.SetConsensusMinerMinPower(abi.NewStoragePower(10 << 40))
	policy.SetSupportedProofTypes(
		abi.RegisteredSealProof_StackedDrg32GiBV1,
		abi.RegisteredSealProof_StackedDrg64GiBV1,
	)

	if os.Getenv("LOTUS_USE_TEST_ADDRESSES") != "1" {
		SetAddressNetwork(address.Mainnet)
	}

	if os.Getenv("LOTUS_DISABLE_V2_ACTOR_MIGRATION") == "1" {
		UpgradeActorsV2Height = math.MaxInt64
	}
}

// UpgradeFunc is a migration function run at every upgrade.
//
// - The oldState is the state produced by the upgrade epoch.
// - The returned newState is the new state that will be used by the next epoch.
// - The height is the upgrade epoch height (already executed).
// - The tipset is the tipset for the last non-null block before the upgrade. Do
//   not assume that ts.Height() is the upgrade height.
type UpgradeFunc func(ctx context.Context, sm *ChainFork, oldState cid.Cid, height abi.ChainEpoch, ts *block.TipSet) (newState cid.Cid, err error)

type Upgrade struct {
	Height    abi.ChainEpoch
	Network   network.Version
	Expensive bool
	Migration UpgradeFunc
}

type UpgradeSchedule []Upgrade

func DefaultUpgradeSchedule() UpgradeSchedule {
	var us UpgradeSchedule

	updates := []Upgrade{{
		Height:    UpgradeBreezeHeight,
		Network:   network.Version1,
		Migration: UpgradeFaucetBurnRecovery,
	}, {
		Height:    UpgradeSmokeHeight,
		Network:   network.Version2,
		Migration: nil,
	}, {
		Height:    UpgradeIgnitionHeight,
		Network:   network.Version3,
		Migration: UpgradeIgnition,
	}, {
		Height:    UpgradeRefuelHeight,
		Network:   network.Version3,
		Migration: UpgradeRefuel,
	}, {
		Height:    UpgradeActorsV2Height,
		Network:   network.Version4,
		Expensive: true,
		Migration: UpgradeActorsV2,
	}, {
		Height:  UpgradeTapeHeight,
		Network: network.Version5,
	}, {
		Height:    UpgradeLiftoffHeight,
		Network:   network.Version5,
		Migration: UpgradeLiftoff,
	}, {
		Height:    UpgradeKumquatHeight,
		Network:   network.Version6,
		Migration: nil,
	}}

	if UpgradeActorsV2Height == math.MaxInt64 { // disable actors upgrade
		updates = []Upgrade{{
			Height:    UpgradeBreezeHeight,
			Network:   network.Version1,
			Migration: UpgradeFaucetBurnRecovery,
		}, {
			Height:    UpgradeSmokeHeight,
			Network:   network.Version2,
			Migration: nil,
		}, {
			Height:    UpgradeIgnitionHeight,
			Network:   network.Version3,
			Migration: UpgradeIgnition,
		}, {
			Height:    UpgradeRefuelHeight,
			Network:   network.Version3,
			Migration: UpgradeRefuel,
		}, {
			Height:    UpgradeLiftoffHeight,
			Network:   network.Version3,
			Migration: UpgradeLiftoff,
		}}
	}

	for _, u := range updates {
		if u.Height < 0 {
			// upgrade disabled
			continue
		}
		us = append(us, u)
	}
	return us
}

func (us UpgradeSchedule) Validate() error {
	// Make sure we're not trying to upgrade to version 0.
	for _, u := range us {
		if u.Network <= 0 {
			return xerrors.Errorf("cannot upgrade to version <= 0: %d", u.Network)
		}
	}

	// Make sure all the upgrades make sense.
	for i := 1; i < len(us); i++ {
		prev := &us[i-1]
		curr := &us[i]
		if !(prev.Network <= curr.Network) {
			return xerrors.Errorf("cannot downgrade from version %d to version %d", prev.Network, curr.Network)
		}
		// Make sure the heights make sense.
		if prev.Height < 0 {
			// Previous upgrade was disabled.
			continue
		}
		if !(prev.Height < curr.Height) {
			return xerrors.Errorf("upgrade heights must be strictly increasing: upgrade %d was at height %d, followed by upgrade %d at height %d", i-1, prev.Height, i, curr.Height)
		}
	}
	return nil
}

type chainReader interface {
	Head() block.TipSetKey
	GetTipSet(tsKey block.TipSetKey) (*block.TipSet, error)
	GetTipSetByHeight(context.Context, *block.TipSet, abi.ChainEpoch, bool) (*block.TipSet, error)
	GetTipSetStateRoot(context.Context, block.TipSetKey) (cid.Cid, error)
	GetTipSetState(context.Context, block.TipSetKey) (vmstate.Tree, error)
	GetGenesisBlock(ctx context.Context) (*block.Block, error)
}
type IFork interface {
	HandleStateForks(ctx context.Context, root cid.Cid, height abi.ChainEpoch, ts *block.TipSet) (cid.Cid, error)
	GetNtwkVersion(ctx context.Context, height abi.ChainEpoch) network.Version
}

var _ = IFork((*ChainFork)(nil))

type ChainFork struct {
	cr        chainReader
	bs        blockstore.Blockstore
	ipldstore cbor.IpldStore

	// Determines the network version at any given epoch.
	networkVersions []versionSpec
	latestVersion   network.Version

	// Maps chain epochs to upgrade functions.
	stateMigrations map[abi.ChainEpoch]UpgradeFunc
	// A set of potentially expensive/time consuming upgrades. Explicit
	// calls for, e.g., gas estimation fail against this epoch with
	// ErrExpensiveFork.
	expensiveUpgrades map[abi.ChainEpoch]struct{}
}

type versionSpec struct {
	networkVersion network.Version
	atOrBelow      abi.ChainEpoch
}

func NewChainFork(cr chainReader, ipldstore cbor.IpldStore, bs blockstore.Blockstore) (*ChainFork, error) {
	// If we have upgrades, make sure they're in-order and make sense.
	us := DefaultUpgradeSchedule()
	if err := us.Validate(); err != nil {
		return nil, err
	}
	log.Infof("UpgradeSchedule: %v", us)

	stateMigrations := make(map[abi.ChainEpoch]UpgradeFunc, len(us))
	expensiveUpgrades := make(map[abi.ChainEpoch]struct{}, len(us))
	var networkVersions []versionSpec
	lastVersion := network.Version0
	if len(us) > 0 {
		// If we have any upgrades, process them and create a version schedule.
		for _, upgrade := range us {
			if upgrade.Migration != nil {
				stateMigrations[upgrade.Height] = upgrade.Migration
			}
			if upgrade.Expensive {
				expensiveUpgrades[upgrade.Height] = struct{}{}
			}
			networkVersions = append(networkVersions, versionSpec{
				networkVersion: lastVersion,
				atOrBelow:      upgrade.Height,
			})
			lastVersion = upgrade.Network
		}
	} else {
		// Otherwise, go directly to the latest version.
		lastVersion = params.NewestNetworkVersion
	}

	fork := &ChainFork{
		cr:        cr,
		bs:        bs,
		ipldstore: ipldstore,

		networkVersions: networkVersions,
		latestVersion:   lastVersion,

		stateMigrations:   stateMigrations,
		expensiveUpgrades: expensiveUpgrades,
	}
	return fork, nil
}

func (fork *ChainFork) StateTree(ctx context.Context, st cid.Cid) (*vmstate.State, error) {
	return vmstate.LoadState(ctx, fork.ipldstore, st)
}

func (fork *ChainFork) HandleStateForks(ctx context.Context, root cid.Cid, height abi.ChainEpoch, ts *block.TipSet) (cid.Cid, error) {
	retCid := root
	var err error
	migration, ok := fork.stateMigrations[height]
	if ok {
		retCid, err = migration(ctx, fork, root, height, ts)
		if err != nil {
			return cid.Undef, err
		}
	}

	return retCid, nil
}

func (sm *ChainFork) hasExpensiveFork(ctx context.Context, height abi.ChainEpoch) bool {
	_, ok := sm.expensiveUpgrades[height]
	return ok
}

func (sm *ChainFork) GetNtwkVersion(ctx context.Context, height abi.ChainEpoch) network.Version {
	// The epochs here are the _last_ epoch for every version, or -1 if the
	// version is disabled.
	for _, spec := range sm.networkVersions {
		if height <= spec.atOrBelow {
			return spec.networkVersion
		}
	}
	return sm.latestVersion
}

func doTransfer(tree vmstate.Tree, from, to address.Address, amt abi.TokenAmount) error {
	fromAct, found, err := tree.GetActor(context.TODO(), from)
	if err != nil {
		return xerrors.Errorf("failed to get 'from' actor for transfer: %v", err)
	}
	if !found {
		return xerrors.Errorf("did not find 'from' actor for transfer: %v", from.String())
	}

	fromAct.Balance = big.Sub(fromAct.Balance, amt)
	if fromAct.Balance.Sign() < 0 {
		return xerrors.Errorf("(sanity) deducted more funds from target account than it had (%s, %s)", from, types.FIL(amt))
	}

	if err := tree.SetActor(context.TODO(), from, fromAct); err != nil {
		return xerrors.Errorf("failed to persist from actor: %v", err)
	}

	toAct, found, err := tree.GetActor(context.TODO(), to)
	if err != nil {
		return xerrors.Errorf("failed to get 'to' actor for transfer: %v", err)
	}
	if !found {
		return xerrors.Errorf("did not find 'to' actor for transfer: %v", from.String())
	}

	toAct.Balance = big.Add(toAct.Balance, amt)

	if err := tree.SetActor(context.TODO(), to, toAct); err != nil {
		return xerrors.Errorf("failed to persist to actor: %v", err)
	}

	return nil
}

func (sm *ChainFork) ParentState(ts *block.TipSet) cid.Cid {
	if ts == nil {
		tts, err := sm.cr.GetTipSet(sm.cr.Head())
		if err == nil {
			return tts.Blocks()[0].StateRoot.Cid
		}
	} else {
		return ts.Blocks()[0].StateRoot.Cid
	}

	return cid.Undef
}

func UpgradeFaucetBurnRecovery(ctx context.Context, sm *ChainFork, root cid.Cid, epoch abi.ChainEpoch, ts *block.TipSet) (cid.Cid, error) {
	// Some initial parameters
	FundsForMiners := types.FromFil(1_000_000)
	LookbackEpoch := abi.ChainEpoch(32000)
	AccountCap := types.FromFil(0)
	BaseMinerBalance := types.FromFil(20)
	DesiredReimbursementBalance := types.FromFil(5_000_000)

	isSystemAccount := func(addr address.Address) (bool, error) {
		id, err := address.IDFromAddress(addr)
		if err != nil {
			return false, xerrors.Errorf("id address: %v", err)
		}

		if id < 1000 {
			return true, nil
		}
		return false, nil
	}

	minerFundsAlloc := func(pow, tpow abi.StoragePower) abi.TokenAmount {
		return big.Div(big.Mul(pow, FundsForMiners), tpow)
	}

	// Grab lookback state for account checks
	lbts, err := sm.cr.GetTipSetByHeight(ctx, ts, LookbackEpoch, false)
	if err != nil {
		return cid.Undef, xerrors.Errorf("failed to get tipset at lookback height: %v", err)
	}

	lbtree, err := sm.cr.GetTipSetState(ctx, lbts.EnsureParents())
	//lbtree, err := sm.StateTree(ctx, sm.ParentState(lbts))
	if err != nil {
		return cid.Undef, xerrors.Errorf("loading state tree failed: %v", err)
	}
	//xxxxx, _ := lbtree.Flush(context.Background())
	//fmt.Println("lbtree root: ", xxxxx)

	tree, err := sm.StateTree(ctx, root)
	if err != nil {
		return cid.Undef, xerrors.Errorf("getting state tree: %v", err)
	}

	type transfer struct {
		From address.Address
		To   address.Address
		Amt  abi.TokenAmount
	}

	// todo not needed
	var transfers []transfer
	//subcalls := make([]types.ExecutionTrace, 0)
	//transferCb := func(trace types.ExecutionTrace) {
	//	subcalls = append(subcalls, trace)
	//}

	// Take all excess funds away, put them into the reserve account
	err = tree.ForEach(func(addr address.Address, act *types.Actor) error {
		switch act.Code.Cid {
		case builtin0.AccountActorCodeID, builtin0.MultisigActorCodeID, builtin0.PaymentChannelActorCodeID:
			sysAcc, err := isSystemAccount(addr)
			if err != nil {
				return xerrors.Errorf("checking system account: %v", err)
			}

			if !sysAcc {
				transfers = append(transfers, transfer{
					From: addr,
					To:   builtin.ReserveAddress,
					Amt:  act.Balance,
				})
			}
		case builtin0.StorageMinerActorCodeID:
			var st miner0.State
			if err := sm.ipldstore.Get(ctx, act.Head.Cid, &st); err != nil {
				return xerrors.Errorf("failed to load miner state: %v", err)
			}

			var available abi.TokenAmount
			{
				defer func() {
					if err := recover(); err != nil {
						log.Warnf("Get available balance failed (%s, %s, %s): %s", addr, act.Head, act.Balance, err)
					}
					available = abi.NewTokenAmount(0)
				}()
				// this panics if the miner doesnt have enough funds to cover their locked pledge
				available = st.GetAvailableBalance(act.Balance)
			}

			if !available.IsZero() {
				transfers = append(transfers, transfer{
					From: addr,
					To:   builtin.ReserveAddress,
					Amt:  available,
				})
			}
		}
		return nil
	})
	if err != nil {
		return cid.Undef, xerrors.Errorf("foreach over state tree failed: %v", err)
	}

	// Execute transfers from previous step
	//fmt.Printf("num:%v, transfers:%v\n", len(transfers), transfers)
	for _, t := range transfers {
		if err := doTransfer(tree, t.From, t.To, t.Amt); err != nil {
			return cid.Undef, xerrors.Errorf("transfer %s %s->%s failed: %v", t.Amt, t.From, t.To, err)
		}
	}

	// pull up power table to give miners back some funds proportional to their power
	var ps power0.State
	powAct, find, err := tree.GetActor(ctx, builtin0.StoragePowerActorAddr)
	if err != nil {
		return cid.Undef, xerrors.Errorf("failed to load power actor: %v", err)
	}

	if !find {
		return cid.Undef, xerrors.New("did not find power actor")
	}

	if err := sm.ipldstore.Get(ctx, powAct.Head.Cid, &ps); err != nil {
		return cid.Undef, xerrors.Errorf("failed to get power actor state: %v", err)
	}

	totalPower := ps.TotalBytesCommitted

	var transfersBack []transfer
	// Now, we return some funds to places where they are needed
	err = tree.ForEach(func(addr address.Address, act *types.Actor) error {
		lbact, _, err := lbtree.GetActor(ctx, addr)
		if err != nil {
			return xerrors.Errorf("failed to get actor in lookback state")
		}

		prevBalance := abi.NewTokenAmount(0)
		if lbact != nil {
			prevBalance = lbact.Balance
		}

		switch act.Code.Cid {
		case builtin0.AccountActorCodeID, builtin0.MultisigActorCodeID, builtin0.PaymentChannelActorCodeID:
			nbalance := big.Min(prevBalance, AccountCap)
			if nbalance.Sign() != 0 {
				transfersBack = append(transfersBack, transfer{
					From: builtin.ReserveAddress,
					To:   addr,
					Amt:  nbalance,
				})
			}
		case builtin0.StorageMinerActorCodeID:
			var st miner0.State
			if err := sm.ipldstore.Get(ctx, act.Head.Cid, &st); err != nil {
				return xerrors.Errorf("failed to load miner state: %v", err)
			}

			var minfo miner0.MinerInfo
			if err := sm.ipldstore.Get(ctx, st.Info, &minfo); err != nil {
				return xerrors.Errorf("failed to get miner info: %v", err)
			}

			sectorsArr, err := adt0.AsArray(adt.WrapStore(ctx, sm.ipldstore), st.Sectors)
			if err != nil {
				return xerrors.Errorf("failed to load sectors array: %v", err)
			}

			slen := sectorsArr.Length()

			power := big.Mul(big.NewInt(int64(slen)), big.NewInt(int64(minfo.SectorSize)))

			mfunds := minerFundsAlloc(power, totalPower)
			transfersBack = append(transfersBack, transfer{
				From: builtin.ReserveAddress,
				To:   minfo.Worker,
				Amt:  mfunds,
			})

			// Now make sure to give each miner who had power at the lookback some FIL
			lbact, found, err := lbtree.GetActor(ctx, addr)
			if err == nil {
				if found {
					var lbst miner0.State
					if err := sm.ipldstore.Get(ctx, lbact.Head.Cid, &lbst); err != nil {
						return xerrors.Errorf("failed to load miner state: %v", err)
					}

					lbsectors, err := adt0.AsArray(adt.WrapStore(ctx, sm.ipldstore), lbst.Sectors)
					if err != nil {
						return xerrors.Errorf("failed to load lb sectors array: %v", err)
					}

					if lbsectors.Length() > 0 {
						transfersBack = append(transfersBack, transfer{
							From: builtin.ReserveAddress,
							To:   minfo.Worker,
							Amt:  BaseMinerBalance,
						})
					}
				} else {
					log.Warnf("did not find actor: %s", addr.String())
				}
			} else {
				log.Warnf("failed to get miner in lookback state: %s", err)
			}
		}
		return nil
	})
	if err != nil {
		return cid.Undef, xerrors.Errorf("foreach over state tree failed: %v", err)
	}

	//fmt.Printf("num:%v, transfersBack:%v\n", len(transfersBack), transfersBack)
	for _, t := range transfersBack {
		if err := doTransfer(tree, t.From, t.To, t.Amt); err != nil {
			return cid.Undef, xerrors.Errorf("transfer %s %s->%s failed: %v", t.Amt, t.From, t.To, err)
		}
	}

	// transfer all burnt funds back to the reserve account
	burntAct, find, err := tree.GetActor(ctx, builtin0.BurntFundsActorAddr)
	if err != nil {
		return cid.Undef, xerrors.Errorf("failed to load burnt funds actor: %v", err)
	}
	if !find {
		return cid.Undef, xerrors.New("did not find burnt funds actor")
	}
	if err := doTransfer(tree, builtin0.BurntFundsActorAddr, builtin.ReserveAddress, burntAct.Balance); err != nil {
		return cid.Undef, xerrors.Errorf("failed to unburn funds: %v", err)
	}

	// Top up the reimbursement service
	reimbAddr, err := address.NewFromString("t0111")
	if err != nil {
		return cid.Undef, xerrors.Errorf("failed to parse reimbursement service address")
	}

	reimb, find, err := tree.GetActor(ctx, reimbAddr)
	if err != nil {
		return cid.Undef, xerrors.Errorf("failed to load reimbursement account actor: %v", err)
	}
	if !find {
		return cid.Undef, xerrors.New("did not find reimbursement actor")
	}

	difference := big.Sub(DesiredReimbursementBalance, reimb.Balance)
	if err := doTransfer(tree, builtin.ReserveAddress, reimbAddr, difference); err != nil {
		return cid.Undef, xerrors.Errorf("failed to top up reimbursement account: %v", err)
	}

	// Now, a final sanity check to make sure the balances all check out
	total := abi.NewTokenAmount(0)
	err = tree.ForEach(func(addr address.Address, act *types.Actor) error {
		total = big.Add(total, act.Balance)
		return nil
	})
	if err != nil {
		return cid.Undef, xerrors.Errorf("checking final state balance failed: %v", err)
	}

	exp := types.FromFil(params.FilBase)
	if !exp.Equals(total) {
		return cid.Undef, xerrors.Errorf("resultant state tree account balance was not correct: %s", total)
	}

	return tree.Flush(ctx)
}

func setNetworkName(ctx context.Context, store adt.Store, tree *vmstate.State, name string) error {
	ia, find, err := tree.GetActor(ctx, builtin0.InitActorAddr)
	if err != nil {
		return xerrors.Errorf("getting init actor: %w", err)
	}
	if !find {
		return xerrors.New("did not find init actor")
	}

	initState, err := init_.Load(store, ia)
	if err != nil {
		return xerrors.Errorf("reading init state: %w", err)
	}

	if err := initState.SetNetworkName(name); err != nil {
		return xerrors.Errorf("setting network name: %w", err)
	}

	c, err := store.Put(ctx, initState)
	if err != nil {
		return xerrors.Errorf("writing new init state: %w", err)
	}
	ia.Head = enccid.NewCid(c)

	if err := tree.SetActor(ctx, builtin0.InitActorAddr, ia); err != nil {
		return xerrors.Errorf("setting init actor: %w", err)
	}

	return nil
}

// TODO: After the Liftoff epoch, refactor this to use resetMultisigVesting
func resetGenesisMsigs0(ctx context.Context, sm *ChainFork, store adt0.Store, tree *vmstate.State, startEpoch abi.ChainEpoch) error {
	gb, err := sm.cr.GetGenesisBlock(ctx)
	if err != nil {
		return xerrors.Errorf("getting genesis block: %w", err)
	}

	gts, err := block.NewTipSet(gb)
	if err != nil {
		return xerrors.Errorf("getting genesis tipset: %v", err)
	}

	genesisTree, err := sm.StateTree(ctx, gts.Blocks()[0].StateRoot.Cid)
	if err != nil {
		return xerrors.Errorf("loading state tree: %w", err)
	}

	err = genesisTree.ForEach(func(addr address.Address, genesisActor *types.Actor) error {
		if genesisActor.Code.Cid == builtin0.MultisigActorCodeID {
			currActor, find, err := tree.GetActor(ctx, addr)
			if err != nil {
				return xerrors.Errorf("loading actor: %w", err)
			}
			if !find {
				return xerrors.Errorf("did not find actor: %s", addr.String())
			}

			var currState multisig0.State
			if err := store.Get(ctx, currActor.Head.Cid, &currState); err != nil {
				return xerrors.Errorf("reading multisig state: %w", err)
			}

			currState.StartEpoch = startEpoch

			head, err := store.Put(ctx, &currState)
			if err != nil {
				return xerrors.Errorf("writing new multisig state: %w", err)
			}
			currActor.Head = enccid.NewCid(head)

			if err := tree.SetActor(ctx, addr, currActor); err != nil {
				return xerrors.Errorf("setting multisig actor: %w", err)
			}
		}
		return nil
	})

	if err != nil {
		return xerrors.Errorf("iterating over genesis actors: %w", err)
	}

	return nil
}

func makeKeyAddr(splitAddr address.Address, count uint64) (address.Address, error) {
	var b bytes.Buffer
	if err := splitAddr.MarshalCBOR(&b); err != nil {
		return address.Undef, xerrors.Errorf("marshalling split address: %w", err)
	}

	if err := binary.Write(&b, binary.BigEndian, count); err != nil {
		return address.Undef, xerrors.Errorf("writing count into a buffer: %w", err)
	}

	if err := binary.Write(&b, binary.BigEndian, []byte("Ignition upgrade")); err != nil {
		return address.Undef, xerrors.Errorf("writing fork name into a buffer: %w", err)
	}

	addr, err := address.NewActorAddress(b.Bytes())
	if err != nil {
		return address.Undef, xerrors.Errorf("create actor address: %w", err)
	}

	return addr, nil
}

func splitGenesisMultisig0(ctx context.Context, addr address.Address, store adt0.Store, tree *vmstate.State, portions uint64, epoch abi.ChainEpoch) error {
	if portions < 1 {
		return xerrors.Errorf("cannot split into 0 portions")
	}

	mact, find, err := tree.GetActor(ctx, addr)
	if err != nil {
		return xerrors.Errorf("getting msig actor: %w", err)
	}
	if !find {
		return xerrors.Errorf("did not find actor: %s", addr.String())
	}

	mst, err := multisig.Load(store, mact)
	if err != nil {
		return xerrors.Errorf("getting msig state: %w", err)
	}

	signers, err := mst.Signers()
	if err != nil {
		return xerrors.Errorf("getting msig signers: %w", err)
	}

	thresh, err := mst.Threshold()
	if err != nil {
		return xerrors.Errorf("getting msig threshold: %w", err)
	}

	ibal, err := mst.InitialBalance()
	if err != nil {
		return xerrors.Errorf("getting msig initial balance: %w", err)
	}

	se, err := mst.StartEpoch()
	if err != nil {
		return xerrors.Errorf("getting msig start epoch: %w", err)
	}

	ud, err := mst.UnlockDuration()
	if err != nil {
		return xerrors.Errorf("getting msig unlock duration: %w", err)
	}

	pending, err := adt0.MakeEmptyMap(store).Root()
	if err != nil {
		return xerrors.Errorf("failed to create empty map: %w", err)
	}

	newIbal := big.Div(ibal, big.NewInt(int64(portions)))
	newState := &multisig0.State{
		Signers:               signers,
		NumApprovalsThreshold: thresh,
		NextTxnID:             0,
		InitialBalance:        newIbal,
		StartEpoch:            se,
		UnlockDuration:        ud,
		PendingTxns:           pending,
	}

	scid, err := store.Put(ctx, newState)
	if err != nil {
		return xerrors.Errorf("storing new state: %w", err)
	}

	newActor := types.Actor{
		Code:       enccid.NewCid(builtin0.MultisigActorCodeID),
		Head:       enccid.NewCid(scid),
		CallSeqNum: 0,
		Balance:    big.Zero(),
	}

	i := uint64(0)
	for i < portions {
		keyAddr, err := makeKeyAddr(addr, i)
		if err != nil {
			return xerrors.Errorf("creating key address: %w", err)
		}

		idAddr, err := tree.RegisterNewAddress(keyAddr)
		if err != nil {
			return xerrors.Errorf("registering new address: %w", err)
		}

		err = tree.SetActor(ctx, idAddr, &newActor)
		if err != nil {
			return xerrors.Errorf("setting new msig actor state: %w", err)
		}

		if err := doTransfer(tree, addr, idAddr, newIbal); err != nil {
			return xerrors.Errorf("transferring split msig balance: %w", err)
		}

		i++
	}

	return nil
}

func resetMultisigVesting0(ctx context.Context, store adt0.Store, tree *vmstate.State, addr address.Address, startEpoch abi.ChainEpoch, duration abi.ChainEpoch, balance abi.TokenAmount) error {
	act, find, err := tree.GetActor(ctx, addr)
	if err != nil {
		return xerrors.Errorf("getting actor: %w", err)
	}
	if !find {
		return xerrors.Errorf("did not find actor: %s", addr.String())
	}

	if !builtin.IsMultisigActor(act.Code.Cid) {
		return xerrors.Errorf("actor wasn't msig: %w", err)
	}

	var msigState multisig0.State
	if err := store.Get(ctx, act.Head.Cid, &msigState); err != nil {
		return xerrors.Errorf("reading multisig state: %w", err)
	}

	msigState.StartEpoch = startEpoch
	msigState.UnlockDuration = duration
	msigState.InitialBalance = balance

	head, err := store.Put(ctx, &msigState)
	if err != nil {
		return xerrors.Errorf("writing new multisig state: %w", err)
	}
	act.Head = enccid.NewCid(head)

	if err := tree.SetActor(ctx, addr, act); err != nil {
		return xerrors.Errorf("setting multisig actor: %w", err)
	}

	return nil
}

func UpgradeIgnition(ctx context.Context, sm *ChainFork, root cid.Cid, epoch abi.ChainEpoch, ts *block.TipSet) (cid.Cid, error) {
	store := adt.WrapStore(ctx, sm.ipldstore)

	if UpgradeLiftoffHeight <= epoch {
		return cid.Undef, xerrors.Errorf("liftoff height must be beyond ignition height")
	}

	nst, err := nv3.MigrateStateTree(ctx, store, root, epoch)
	if err != nil {
		return cid.Undef, xerrors.Errorf("migrating actors state: %v", err)
	}

	tree, err := sm.StateTree(ctx, nst)
	if err != nil {
		return cid.Undef, xerrors.Errorf("getting state tree: %v", err)
	}

	err = setNetworkName(ctx, store, tree, "ignition")
	if err != nil {
		return cid.Undef, xerrors.Errorf("setting network name: %v", err)
	}

	split1, err := address.NewFromString("t0115")
	if err != nil {
		return cid.Undef, xerrors.Errorf("first split address: %v", err)
	}

	split2, err := address.NewFromString("t0116")
	if err != nil {
		return cid.Undef, xerrors.Errorf("second split address: %v", err)
	}

	err = resetGenesisMsigs0(ctx, sm, store, tree, UpgradeLiftoffHeight)
	if err != nil {
		return cid.Undef, xerrors.Errorf("resetting genesis msig start epochs: %v", err)
	}

	err = splitGenesisMultisig0(ctx, split1, store, tree, 50, epoch)
	if err != nil {
		return cid.Undef, xerrors.Errorf("splitting first msig: %v", err)
	}

	err = splitGenesisMultisig0(ctx, split2, store, tree, 50, epoch)
	if err != nil {
		return cid.Undef, xerrors.Errorf("splitting second msig: %v", err)
	}

	err = nv3.CheckStateTree(ctx, store, nst, epoch, builtin0.TotalFilecoin)
	if err != nil {
		return cid.Undef, xerrors.Errorf("sanity check after ignition upgrade failed: %v", err)
	}

	return tree.Flush(ctx)
}

func UpgradeRefuel(ctx context.Context, sm *ChainFork, root cid.Cid, epoch abi.ChainEpoch, ts *block.TipSet) (cid.Cid, error) {
	store := adt.WrapStore(ctx, sm.ipldstore)
	tree, err := sm.StateTree(ctx, root)
	if err != nil {
		return cid.Undef, xerrors.Errorf("getting state tree: %v", err)
	}

	err = resetMultisigVesting0(ctx, store, tree, builtin.SaftAddress, 0, 0, big.Zero())
	if err != nil {
		return cid.Undef, xerrors.Errorf("tweaking msig vesting: %v", err)
	}

	err = resetMultisigVesting0(ctx, store, tree, builtin.ReserveAddress, 0, 0, big.Zero())
	if err != nil {
		return cid.Undef, xerrors.Errorf("tweaking msig vesting: %v", err)
	}

	err = resetMultisigVesting0(ctx, store, tree, builtin.RootVerifierAddress, 0, 0, big.Zero())
	if err != nil {
		return cid.Undef, xerrors.Errorf("tweaking msig vesting: %v", err)
	}

	return tree.Flush(ctx)
}

func ActorStore(ctx context.Context, bs blockstore.Blockstore) adt.Store {
	return adt.WrapStore(ctx, cbor.NewCborStore(bs))
}

func UpgradeActorsV2(ctx context.Context, sm *ChainFork, root cid.Cid, epoch abi.ChainEpoch, ts *block.TipSet) (cid.Cid, error) {
	buf := bufbstore.NewTieredBstore(sm.bs, bstore.NewTemporarySync())
	store := ActorStore(ctx, buf)

	info, err := store.Put(ctx, new(vmstate.StateInfo0))
	if err != nil {
		return cid.Undef, xerrors.Errorf("failed to create new state info for actors v2: %v", err)
	}

	newHamtRoot, err := m2.MigrateStateTree(ctx, store, root, epoch, m2.DefaultConfig())
	if err != nil {
		return cid.Undef, xerrors.Errorf("upgrading to actors v2: %v", err)
	}

	newRoot, err := store.Put(ctx, &vmstate.StateRoot{
		Version: vmstate.StateTreeVersion1,
		Actors:  enccid.NewCid(newHamtRoot),
		Info:    enccid.NewCid(info),
	})
	if err != nil {
		return cid.Undef, xerrors.Errorf("failed to persist new state root: %v", err)
	}

	// perform some basic sanity checks to make sure everything still works.
	if newSm, err := vmstate.LoadState(ctx, store, newRoot); err != nil {
		return cid.Undef, xerrors.Errorf("state tree sanity load failed: %v", err)
	} else if newRoot2, err := newSm.Flush(ctx); err != nil {
		return cid.Undef, xerrors.Errorf("state tree sanity flush failed: %v", err)
	} else if newRoot2 != newRoot {
		return cid.Undef, xerrors.Errorf("state-root mismatch: %s != %s", newRoot, newRoot2)
	} else if _, _, err := newSm.GetActor(ctx, builtin0.InitActorAddr); err != nil {
		return cid.Undef, xerrors.Errorf("failed to load init actor after upgrade: %v", err)
	}

	// Todo need to be implemented?
	//{
	//	from := buf
	//	to := buf.Read()
	//
	//	if err := vm.Copy(ctx, from, to, newRoot); err != nil {
	//		return cid.Undef, xerrors.Errorf("copying migrated tree: %v", err)
	//	}
	//}

	return newRoot, nil
}

func UpgradeLiftoff(ctx context.Context, sm *ChainFork, root cid.Cid, epoch abi.ChainEpoch, ts *block.TipSet) (cid.Cid, error) {
	tree, err := sm.StateTree(ctx, root)
	if err != nil {
		return cid.Undef, xerrors.Errorf("getting state tree: %v", err)
	}

	err = setNetworkName(ctx, adt.WrapStore(ctx, sm.ipldstore), tree, "mainnet")
	if err != nil {
		return cid.Undef, xerrors.Errorf("setting network name: %v", err)
	}

	return tree.Flush(ctx)
}

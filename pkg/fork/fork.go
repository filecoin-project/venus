package fork

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/docker/go-units"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	actorstypes "github.com/filecoin-project/go-state-types/actors"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/manifest"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/go-state-types/rt"
	gstStore "github.com/filecoin-project/go-state-types/store"
	blockstore "github.com/ipfs/boxo/blockstore"
	ipfsblock "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	dstore "github.com/ipfs/go-datastore"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multicodec"
	cbg "github.com/whyrusleeping/cbor-gen"
	"go.opencensus.io/trace"

	nv18 "github.com/filecoin-project/go-state-types/builtin/v10/migration"
	nv19 "github.com/filecoin-project/go-state-types/builtin/v11/migration"
	nv21 "github.com/filecoin-project/go-state-types/builtin/v12/migration"
	nv17 "github.com/filecoin-project/go-state-types/builtin/v9/migration"
	"github.com/filecoin-project/go-state-types/migration"
	"github.com/filecoin-project/specs-actors/actors/migration/nv3"
	"github.com/filecoin-project/specs-actors/v2/actors/migration/nv4"
	"github.com/filecoin-project/specs-actors/v2/actors/migration/nv7"
	"github.com/filecoin-project/specs-actors/v3/actors/migration/nv10"
	"github.com/filecoin-project/specs-actors/v4/actors/migration/nv12"
	"github.com/filecoin-project/specs-actors/v5/actors/migration/nv13"
	"github.com/filecoin-project/specs-actors/v6/actors/migration/nv14"
	"github.com/filecoin-project/specs-actors/v7/actors/migration/nv15"
	"github.com/filecoin-project/specs-actors/v8/actors/migration/nv16"

	init11 "github.com/filecoin-project/go-state-types/builtin/v11/init"
	system11 "github.com/filecoin-project/go-state-types/builtin/v11/system"
	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
	miner0 "github.com/filecoin-project/specs-actors/actors/builtin/miner"
	multisig0 "github.com/filecoin-project/specs-actors/actors/builtin/multisig"
	power0 "github.com/filecoin-project/specs-actors/actors/builtin/power"
	adt0 "github.com/filecoin-project/specs-actors/actors/util/adt"

	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/repo"
	vmstate "github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	init_ "github.com/filecoin-project/venus/venus-shared/actors/builtin/init"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/multisig"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/system"
	blockstoreutil "github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/types"
)

//go:embed FVMLiftoff.txt
var fvmLiftoffBanner string

var log = logging.Logger("fork")

var ErrExpensiveFork = errors.New("refusing explicit call due to state fork at epoch")

var (
	MigrationMaxWorkerCount    int
	EnvMigrationMaxWorkerCount = "VENUS_MIGRATION_MAX_WORKER_COUNT"
)

func init() {
	// the default calculation used for migration worker count
	MigrationMaxWorkerCount = runtime.NumCPU()
	// check if an alternative value was request by environment
	if mwcs := os.Getenv(EnvMigrationMaxWorkerCount); mwcs != "" {
		mwc, err := strconv.ParseInt(mwcs, 10, 32)
		if err != nil {
			log.Warnf("invalid value for %s (%s) defaulting to %d: %s", EnvMigrationMaxWorkerCount, mwcs, MigrationMaxWorkerCount, err)
			return
		}
		// use value from environment
		log.Infof("migration worker cound set from %s (%d)", EnvMigrationMaxWorkerCount, mwc)
		MigrationMaxWorkerCount = int(mwc)
		return
	}
	log.Infof("migration worker count: %d", MigrationMaxWorkerCount)
}

// MigrationCache can be used to cache information used by a migration. This is primarily useful to
// "pre-compute" some migration state ahead of time, and make it accessible in the migration itself.
type MigrationCache interface {
	Write(key string, value cid.Cid) error
	Read(key string) (bool, cid.Cid, error)
	Load(key string, loadFunc func() (cid.Cid, error)) (cid.Cid, error)
}

// MigrationFunc is a migration function run at every upgrade.
//
//   - The cache is a per-upgrade cache, pre-populated by pre-migrations.
//   - The oldState is the state produced by the upgrade epoch.
//   - The returned newState is the new state that will be used by the next epoch.
//   - The height is the upgrade epoch height (already executed).
//   - The tipset is the tipset for the last non-null block before the upgrade. Do
//     not assume that ts.Height() is the upgrade height.
type MigrationFunc func(
	ctx context.Context,
	cache MigrationCache,
	oldState cid.Cid,
	height abi.ChainEpoch,
	ts *types.TipSet,
) (newState cid.Cid, err error)

// PreMigrationFunc is a function run _before_ a network upgrade to pre-compute part of the network
// upgrade and speed it up.
type PreMigrationFunc func(
	ctx context.Context,
	cache MigrationCache,
	oldState cid.Cid,
	height abi.ChainEpoch,
	ts *types.TipSet,
) error

// PreMigration describes a pre-migration step to prepare for a network state upgrade. Pre-migrations
// are optimizations, are not guaranteed to run, and may be canceled and/or run multiple times.
type PreMigration struct {
	// PreMigration is the pre-migration function to run at the specified time. This function is
	// run asynchronously and must abort promptly when canceled.
	PreMigration PreMigrationFunc

	// StartWithin specifies that this pre-migration should be started at most StartWithin
	// epochs before the upgrade.
	StartWithin abi.ChainEpoch

	// DontStartWithin specifies that this pre-migration should not be started DontStartWithin
	// epochs before the final upgrade epoch.
	//
	// This should be set such that the pre-migration is likely to complete before StopWithin.
	DontStartWithin abi.ChainEpoch

	// StopWithin specifies that this pre-migration should be stopped StopWithin epochs of the
	// final upgrade epoch.
	StopWithin abi.ChainEpoch
}

type Upgrade struct {
	Height    abi.ChainEpoch
	Network   network.Version
	Expensive bool
	Migration MigrationFunc

	// PreMigrations specifies a set of pre-migration functions to run at the indicated epochs.
	// These functions should fill the given cache with information that can speed up the
	// eventual full migration at the upgrade epoch.
	PreMigrations []PreMigration
}

type UpgradeSchedule []Upgrade

type migrationLogger struct{}

func (ml migrationLogger) Log(level rt.LogLevel, msg string, args ...interface{}) {
	switch level {
	case rt.DEBUG:
		log.Debugf(msg, args...)
	case rt.INFO:
		log.Infof(msg, args...)
	case rt.WARN:
		log.Warnf(msg, args...)
	case rt.ERROR:
		log.Errorf(msg, args...)
	}
}

func DefaultUpgradeSchedule(cf *ChainFork, upgradeHeight *config.ForkUpgradeConfig) UpgradeSchedule {
	var us UpgradeSchedule

	updates := []Upgrade{
		{
			Height:    upgradeHeight.UpgradeBreezeHeight,
			Network:   network.Version1,
			Migration: cf.UpgradeFaucetBurnRecovery,
		},
		{
			Height:    upgradeHeight.UpgradeSmokeHeight,
			Network:   network.Version2,
			Migration: nil,
		},
		{
			Height:    upgradeHeight.UpgradeIgnitionHeight,
			Network:   network.Version3,
			Migration: cf.UpgradeIgnition,
		},
		{
			Height:    upgradeHeight.UpgradeRefuelHeight,
			Network:   network.Version3,
			Migration: cf.UpgradeRefuel,
		},
		{
			Height:    upgradeHeight.UpgradeAssemblyHeight,
			Network:   network.Version4,
			Expensive: true,
			Migration: cf.UpgradeActorsV2,
		},
		{
			Height:    upgradeHeight.UpgradeTapeHeight,
			Network:   network.Version5,
			Migration: nil,
		},
		{
			Height:    upgradeHeight.UpgradeLiftoffHeight,
			Network:   network.Version5,
			Migration: cf.UpgradeLiftoff,
		},
		{
			Height:    upgradeHeight.UpgradeKumquatHeight,
			Network:   network.Version6,
			Migration: nil,
		},
		//{
		//		Height:    upgradeHeight.UpgradePriceListOopsHeight,
		//		Network:   network.Version6AndAHalf,
		//		Migration: nil,
		//},
		{
			Height:    upgradeHeight.UpgradeCalicoHeight,
			Network:   network.Version7,
			Migration: cf.UpgradeCalico,
		},
		{
			Height:    upgradeHeight.UpgradePersianHeight,
			Network:   network.Version8,
			Migration: nil,
		},
		{
			Height:    upgradeHeight.UpgradeOrangeHeight,
			Network:   network.Version9,
			Migration: nil,
		},
		{
			Height:    upgradeHeight.UpgradeTrustHeight,
			Network:   network.Version10,
			Migration: cf.UpgradeActorsV3,
			PreMigrations: []PreMigration{{
				PreMigration:    cf.PreUpgradeActorsV3,
				StartWithin:     120,
				DontStartWithin: 60,
				StopWithin:      35,
			}, {
				PreMigration:    cf.PreUpgradeActorsV3,
				StartWithin:     30,
				DontStartWithin: 15,
				StopWithin:      5,
			}},
			Expensive: true,
		},
		{
			Height:    upgradeHeight.UpgradeNorwegianHeight,
			Network:   network.Version11,
			Migration: nil,
		},
		{
			Height:    upgradeHeight.UpgradeTurboHeight,
			Network:   network.Version12,
			Migration: cf.UpgradeActorsV4,
			PreMigrations: []PreMigration{{
				PreMigration:    cf.PreUpgradeActorsV4,
				StartWithin:     120,
				DontStartWithin: 60,
				StopWithin:      35,
			}, {
				PreMigration:    cf.PreUpgradeActorsV4,
				StartWithin:     30,
				DontStartWithin: 15,
				StopWithin:      5,
			}},
			Expensive: true,
		},
		{
			Height:    upgradeHeight.UpgradeHyperdriveHeight,
			Network:   network.Version13,
			Migration: cf.UpgradeActorsV5,
			PreMigrations: []PreMigration{{
				PreMigration:    cf.PreUpgradeActorsV5,
				StartWithin:     120,
				DontStartWithin: 60,
				StopWithin:      35,
			}, {
				PreMigration:    cf.PreUpgradeActorsV5,
				StartWithin:     30,
				DontStartWithin: 15,
				StopWithin:      5,
			}},
			Expensive: true,
		},
		{
			Height:    upgradeHeight.UpgradeChocolateHeight,
			Network:   network.Version14,
			Migration: cf.UpgradeActorsV6,
			PreMigrations: []PreMigration{{
				PreMigration:    cf.PreUpgradeActorsV6,
				StartWithin:     120,
				DontStartWithin: 60,
				StopWithin:      35,
			}, {
				PreMigration:    cf.PreUpgradeActorsV6,
				StartWithin:     30,
				DontStartWithin: 15,
				StopWithin:      5,
			}},
			Expensive: true,
		},
		{
			Height:    upgradeHeight.UpgradeOhSnapHeight,
			Network:   network.Version15,
			Migration: cf.UpgradeActorsV7,
			PreMigrations: []PreMigration{{
				PreMigration:    cf.PreUpgradeActorsV7,
				StartWithin:     180,
				DontStartWithin: 60,
				StopWithin:      5,
			}},
			Expensive: true,
		},
		{
			Height:    upgradeHeight.UpgradeSkyrHeight,
			Network:   network.Version16,
			Migration: cf.UpgradeActorsV8,
			PreMigrations: []PreMigration{{
				PreMigration:    cf.PreUpgradeActorsV8,
				StartWithin:     180,
				DontStartWithin: 60,
				StopWithin:      5,
			}},
			Expensive: true,
		},
		{
			Height:    upgradeHeight.UpgradeSharkHeight,
			Network:   network.Version17,
			Migration: cf.UpgradeActorsV9,
			PreMigrations: []PreMigration{{
				PreMigration:    cf.PreUpgradeActorsV9,
				StartWithin:     240,
				DontStartWithin: 60,
				StopWithin:      20,
			}, {
				PreMigration:    cf.PreUpgradeActorsV9,
				StartWithin:     15,
				DontStartWithin: 10,
				StopWithin:      5,
			}},
			Expensive: true,
		}, {
			Height:    upgradeHeight.UpgradeHyggeHeight,
			Network:   network.Version18,
			Migration: cf.UpgradeActorsV10,
			PreMigrations: []PreMigration{{
				PreMigration:    cf.PreUpgradeActorsV10,
				StartWithin:     60,
				DontStartWithin: 10,
				StopWithin:      5,
			}},
			Expensive: true,
		}, {
			Height:    upgradeHeight.UpgradeLightningHeight,
			Network:   network.Version19,
			Migration: cf.UpgradeActorsV11,
			PreMigrations: []PreMigration{{
				PreMigration:    cf.PreUpgradeActorsV11,
				StartWithin:     120,
				DontStartWithin: 15,
				StopWithin:      10,
			}},
			Expensive: true,
		}, {
			Height:    upgradeHeight.UpgradeThunderHeight,
			Network:   network.Version20,
			Migration: nil,
		}, {
			Height:    upgradeHeight.UpgradeWatermelonHeight,
			Network:   network.Version21,
			Migration: cf.UpgradeActorsV12,
			PreMigrations: []PreMigration{{
				PreMigration:    cf.PreUpgradeActorsV12,
				StartWithin:     180,
				DontStartWithin: 15,
				StopWithin:      10,
			}},
			Expensive: true,
		}, {
			Height:    upgradeHeight.UpgradeWatermelonFixHeight,
			Network:   network.Version21,
			Migration: cf.buildUpgradeActorsV12MinerFix(calibnetv12BuggyMinerCID1, calibnetv12BuggyManifestCID2),
		},
		{
			Height:    upgradeHeight.UpgradeWatermelonFix2Height,
			Network:   network.Version21,
			Migration: cf.buildUpgradeActorsV12MinerFix(calibnetv12BuggyMinerCID2, calibnetv12CorrectManifestCID1),
		},
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
	// Make sure each upgrade is valid.
	for _, u := range us {
		if u.Network <= 0 {
			return fmt.Errorf("cannot upgrade to version <= 0: %d", u.Network)
		}

		for _, m := range u.PreMigrations {
			if m.StartWithin <= 0 {
				return fmt.Errorf("pre-migration must specify a positive start-within epoch")
			}

			if m.DontStartWithin < 0 || m.StopWithin < 0 {
				return fmt.Errorf("pre-migration must specify non-negative epochs")
			}

			if m.StartWithin <= m.StopWithin {
				return fmt.Errorf("pre-migration start-within must come before stop-within")
			}

			// If we have a dont-start-within.
			if m.DontStartWithin != 0 {
				if m.DontStartWithin < m.StopWithin {
					return fmt.Errorf("pre-migration dont-start-within must come before stop-within")
				}
				if m.StartWithin <= m.DontStartWithin {
					return fmt.Errorf("pre-migration start-within must come after dont-start-within")
				}
			}
		}
		if !sort.SliceIsSorted(u.PreMigrations, func(i, j int) bool {
			return u.PreMigrations[i].StartWithin > u.PreMigrations[j].StartWithin //nolint:scopelint,gosec
		}) {
			return fmt.Errorf("pre-migrations must be sorted by start epoch")
		}
	}

	// Make sure the upgrade order makes sense.
	for i := 1; i < len(us); i++ {
		prev := &us[i-1]
		curr := &us[i]
		if !(prev.Network <= curr.Network) {
			return fmt.Errorf("cannot downgrade from version %d to version %d", prev.Network, curr.Network)
		}
		// Make sure the heights make sense.
		if prev.Height < 0 {
			// Previous upgrade was disabled.
			continue
		}
		if !(prev.Height < curr.Height) {
			return fmt.Errorf("upgrade heights must be strictly increasing: upgrade %d was at height %d, followed by upgrade %d at height %d", i-1, prev.Height, i, curr.Height)
		}
	}
	return nil
}

type chainReader interface {
	GetHead() *types.TipSet
	GetTipSet(context.Context, types.TipSetKey) (*types.TipSet, error)
	GetTipSetByHeight(context.Context, *types.TipSet, abi.ChainEpoch, bool) (*types.TipSet, error)
	GetTipSetState(context.Context, *types.TipSet) (vmstate.Tree, error)
	GetGenesisBlock(context.Context) (*types.BlockHeader, error)
	GetLookbackTipSetForRound(ctx context.Context, ts *types.TipSet, round abi.ChainEpoch, version network.Version) (*types.TipSet, cid.Cid, error)
	SubHeadChanges(context.Context) chan []*types.HeadChange
}

type IFork interface {
	HandleStateForks(ctx context.Context, root cid.Cid, height abi.ChainEpoch, ts *types.TipSet) (cid.Cid, error)
	GetNetworkVersion(ctx context.Context, height abi.ChainEpoch) network.Version
	HasExpensiveFork(ctx context.Context, height abi.ChainEpoch) bool
	HasExpensiveForkBetween(parent, height abi.ChainEpoch) bool
	GetForkUpgrade() *config.ForkUpgradeConfig
	Start(ctx context.Context) error
}

var _ = IFork((*ChainFork)(nil))

type versionSpec struct {
	networkVersion network.Version
	atOrBelow      abi.ChainEpoch
}

type Migration struct {
	upgrade              MigrationFunc
	preMigrations        []PreMigration
	cache                *nv16.MemMigrationCache
	migrationResultCache *migrationResultCache
}

type migrationResultCache struct {
	ds        dstore.Batching
	keyPrefix string
}

func (m *migrationResultCache) keyForMigration(root cid.Cid) dstore.Key {
	str := fmt.Sprintf("%s/%s", m.keyPrefix, root)
	return dstore.NewKey(str)
}

func (m *migrationResultCache) Get(ctx context.Context, root cid.Cid) (cid.Cid, bool, error) {
	k := m.keyForMigration(root)

	bs, err := m.ds.Get(ctx, k)
	if ipld.IsNotFound(err) {
		return cid.Undef, false, nil
	} else if err != nil {
		return cid.Undef, false, fmt.Errorf("error loading migration result: %w", err)
	}

	c, err := cid.Parse(bs)
	if err != nil {
		return cid.Undef, false, fmt.Errorf("error parsing migration result: %w", err)
	}

	return c, true, nil
}

func (m *migrationResultCache) Store(ctx context.Context, root cid.Cid, resultCid cid.Cid) error {
	k := m.keyForMigration(root)
	if err := m.ds.Put(ctx, k, resultCid.Bytes()); err != nil {
		return err
	}

	return nil
}

type ChainFork struct {
	cr        chainReader
	bs        blockstoreutil.Blockstore
	ipldstore cbor.IpldStore

	// Determines the network version at any given epoch.
	networkVersions []versionSpec
	latestVersion   network.Version

	// Maps chain epochs to upgrade functions.
	stateMigrations map[abi.ChainEpoch]*Migration
	// A set of potentially expensive/time consuming upgrades. Explicit
	// calls for, e.g., gas estimation fail against this epoch with
	// ErrExpensiveFork.
	expensiveUpgrades map[abi.ChainEpoch]struct{}

	// upgrade param
	networkType types.NetworkType
	forkUpgrade *config.ForkUpgradeConfig

	metadataDs repo.Datastore
}

func NewChainFork(ctx context.Context,
	cr chainReader,
	ipldstore cbor.IpldStore,
	bs blockstoreutil.Blockstore,
	networkParams *config.NetworkParamsConfig,
	metadataDs dstore.Batching,
) (*ChainFork, error) {
	fork := &ChainFork{
		cr:          cr,
		bs:          bs,
		ipldstore:   ipldstore,
		networkType: networkParams.NetworkType,
		forkUpgrade: networkParams.ForkUpgradeParam,
		metadataDs:  metadataDs,
	}

	// If we have upgrades, make sure they're in-order and make sense.
	us := DefaultUpgradeSchedule(fork, networkParams.ForkUpgradeParam)
	if err := us.Validate(); err != nil {
		return nil, err
	}

	stateMigrations := make(map[abi.ChainEpoch]*Migration, len(us))
	expensiveUpgrades := make(map[abi.ChainEpoch]struct{}, len(us))
	var networkVersions []versionSpec
	lastVersion := networkParams.GenesisNetworkVersion
	if len(us) > 0 {
		// If we have any upgrades, process them and create a version schedule.
		for _, upgrade := range us {
			if upgrade.Migration != nil || upgrade.PreMigrations != nil {
				migration := &Migration{
					upgrade:       upgrade.Migration,
					preMigrations: upgrade.PreMigrations,
					cache:         nv16.NewMemMigrationCache(),
					migrationResultCache: &migrationResultCache{
						keyPrefix: fmt.Sprintf("/migration-cache/nv%d", upgrade.Network),
						ds:        metadataDs,
					},
				}
				stateMigrations[upgrade.Height] = migration
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
	}

	fork.networkVersions = networkVersions
	fork.latestVersion = lastVersion
	fork.stateMigrations = stateMigrations
	fork.expensiveUpgrades = expensiveUpgrades

	return fork, nil
}

func (c *ChainFork) Start(ctx context.Context) error {
	log.Info("preMigrationWorker start ...")
	go c.preMigrationWorker(ctx)

	return nil
}

func (c *ChainFork) StateTree(ctx context.Context, st cid.Cid) (*vmstate.State, error) {
	return vmstate.LoadState(ctx, c.ipldstore, st)
}

func (c *ChainFork) HandleStateForks(ctx context.Context, root cid.Cid, height abi.ChainEpoch, ts *types.TipSet) (cid.Cid, error) {
	retCid := root
	u := c.stateMigrations[height]
	if u != nil && u.upgrade != nil {
		if height != c.forkUpgrade.UpgradeWatermelonFixHeight {
			migCid, ok, err := u.migrationResultCache.Get(ctx, root)
			if err == nil && ok && !constants.NoMigrationResultCache {
				log.Infow("CACHED migration", "height", height, "from", root, "to", migCid)
				return migCid, nil
			} else if !errors.Is(err, dstore.ErrNotFound) {
				log.Errorw("failed to lookup previous migration result", "err", err)
			} else {
				log.Debug("no cached migration found, migrating from scratch")
			}
		}

		var err error
		startTime := time.Now()
		log.Warnw("STARTING migration", "height", height, "from", root)
		// Yes, we clone the cache, even for the final upgrade epoch. Why? Reverts. We may
		// have to migrate multiple times.
		tmpCache := u.cache.Clone()
		retCid, err = u.upgrade(ctx, tmpCache, root, height, ts)
		if err != nil {
			log.Errorw("FAILED migration", "height", height, "from", root, "error", err)
			return cid.Undef, err
		}
		// Yes, we update the cache, even for the final upgrade epoch. Why? Reverts. This
		// can save us a _lot_ of time because very few actors will have changed if we
		// do a small revert then need to re-run the migration.
		u.cache.Update(tmpCache)
		log.Warnw("COMPLETED migration",
			"height", height,
			"from", root,
			"to", retCid,
			"duration", time.Since(startTime),
		)

		// Only set if migration ran, we do not want a root => root mapping
		if err := u.migrationResultCache.Store(ctx, root, retCid); err != nil {
			log.Errorw("failed to store migration result", "err", err)
		}
	}

	return retCid, nil
}

func (c *ChainFork) HasExpensiveFork(ctx context.Context, height abi.ChainEpoch) bool {
	_, ok := c.expensiveUpgrades[height]
	return ok
}

// Returns true executing tipsets between the specified heights would trigger an expensive
// migration. NOTE: migrations occurring _at_ the target height are not included, as they're
// executed _after_ the target height.
func (c *ChainFork) HasExpensiveForkBetween(parent, height abi.ChainEpoch) bool {
	for h := parent; h < height; h++ {
		if _, ok := c.expensiveUpgrades[h]; ok {
			return true
		}
	}
	return false
}

func (c *ChainFork) GetNetworkVersion(ctx context.Context, height abi.ChainEpoch) network.Version {
	// The epochs here are the _last_ epoch for every version, or -1 if the
	// version is disabled.
	for _, spec := range c.networkVersions {
		if height <= spec.atOrBelow {
			return spec.networkVersion
		}
	}
	return c.latestVersion
}

func runPreMigration(ctx context.Context, fn PreMigrationFunc, cache *nv16.MemMigrationCache, ts *types.TipSet) {
	height := ts.Height()
	parent := ts.Blocks()[0].ParentStateRoot

	startTime := time.Now()

	log.Warn("STARTING pre-migration")
	// Clone the cache so we don't actually _update_ it
	// till we're done. Otherwise, if we fail, the next
	// migration to use the cache may assume that
	// certain blocks exist, even if they don't.
	tmpCache := cache.Clone()
	err := fn(ctx, tmpCache, parent, height, ts)
	if err != nil {
		log.Errorw("FAILED pre-migration", "error", err)
		return
	}
	// Finally, if everything worked, update the cache.
	cache.Update(tmpCache)
	log.Warnw("COMPLETED pre-migration", "duration", time.Since(startTime))
}

func (c *ChainFork) preMigrationWorker(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type op struct {
		after    abi.ChainEpoch
		notAfter abi.ChainEpoch
		run      func(ts *types.TipSet)
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	// Turn each pre-migration into an operation in a schedule.
	var schedule []op
	for upgradeEpoch, migration := range c.stateMigrations {
		cache := migration.cache
		for _, prem := range migration.preMigrations {
			preCtx, preCancel := context.WithCancel(ctx)
			migrationFunc := prem.PreMigration

			afterEpoch := upgradeEpoch - prem.StartWithin
			notAfterEpoch := upgradeEpoch - prem.DontStartWithin
			stopEpoch := upgradeEpoch - prem.StopWithin
			// We can't start after we stop.
			if notAfterEpoch > stopEpoch {
				notAfterEpoch = stopEpoch - 1
			}

			// Add an op to start a pre-migration.
			schedule = append(schedule, op{
				after:    afterEpoch,
				notAfter: notAfterEpoch,

				// TODO: are these values correct?
				run: func(ts *types.TipSet) {
					wg.Add(1)
					go func() {
						defer wg.Done()
						runPreMigration(preCtx, migrationFunc, cache, ts)
					}()
				},
			})

			// Add an op to cancel the pre-migration if it's still running.
			schedule = append(schedule, op{
				after:    stopEpoch,
				notAfter: -1,
				run:      func(ts *types.TipSet) { preCancel() },
			})
		}
	}

	// Then sort by epoch.
	sort.Slice(schedule, func(i, j int) bool {
		return schedule[i].after < schedule[j].after
	})

	// Finally, when the head changes, see if there's anything we need to do.
	//
	// We're intentionally ignoring reorgs as they don't matter for our purposes.
	for change := range c.cr.SubHeadChanges(ctx) {
		for _, head := range change {
			for len(schedule) > 0 {
				op := &schedule[0]
				if head.Val.Height() < op.after {
					break
				}

				// If we haven't passed the pre-migration height...
				if op.notAfter < 0 || head.Val.Height() < op.notAfter {
					op.run(head.Val)
				}
				schedule = schedule[1:]
			}
		}
	}
}

func doTransfer(tree vmstate.Tree, from, to address.Address, amt abi.TokenAmount) error {
	fromAct, found, err := tree.GetActor(context.TODO(), from)
	if err != nil {
		return fmt.Errorf("failed to get 'from' actor for transfer: %v", err)
	}
	if !found {
		return fmt.Errorf("did not find 'from' actor for transfer: %v", from.String())
	}

	fromAct.Balance = big.Sub(fromAct.Balance, amt)
	if fromAct.Balance.Sign() < 0 {
		return fmt.Errorf("(sanity) deducted more funds from target account than it had (%s, %s)", from, types.FIL(amt))
	}

	if err := tree.SetActor(context.TODO(), from, fromAct); err != nil {
		return fmt.Errorf("failed to persist from actor: %v", err)
	}

	toAct, found, err := tree.GetActor(context.TODO(), to)
	if err != nil {
		return fmt.Errorf("failed to get 'to' actor for transfer: %v", err)
	}
	if !found {
		return fmt.Errorf("did not find 'to' actor for transfer: %v", from.String())
	}

	toAct.Balance = big.Add(toAct.Balance, amt)

	if err := tree.SetActor(context.TODO(), to, toAct); err != nil {
		return fmt.Errorf("failed to persist to actor: %v", err)
	}

	return nil
}

func (c *ChainFork) ParentState(ts *types.TipSet) cid.Cid {
	if ts == nil {
		tts := c.cr.GetHead()
		return tts.Blocks()[0].ParentStateRoot
	}
	return ts.Blocks()[0].ParentStateRoot
}

func (c *ChainFork) UpgradeFaucetBurnRecovery(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) (cid.Cid, error) {
	// Some initial parameters
	FundsForMiners := types.FromFil(1_000_000)
	LookbackEpoch := abi.ChainEpoch(32000)
	AccountCap := types.FromFil(0)
	BaseMinerBalance := types.FromFil(20)
	DesiredReimbursementBalance := types.FromFil(5_000_000)

	isSystemAccount := func(addr address.Address) (bool, error) {
		id, err := address.IDFromAddress(addr)
		if err != nil {
			return false, fmt.Errorf("id address: %v", err)
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
	lbts, err := c.cr.GetTipSetByHeight(ctx, ts, LookbackEpoch, false)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to get tipset at lookback height: %v", err)
	}

	pts, err := c.cr.GetTipSet(ctx, lbts.Parents())
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to get tipset : %v", err)
	}

	lbtree, err := c.cr.GetTipSetState(ctx, pts)
	if err != nil {
		return cid.Undef, fmt.Errorf("loading state tree failed: %v", err)
	}

	tree, err := c.StateTree(ctx, root)
	if err != nil {
		return cid.Undef, fmt.Errorf("getting state tree: %v", err)
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
		switch act.Code {
		case builtin0.AccountActorCodeID, builtin0.MultisigActorCodeID, builtin0.PaymentChannelActorCodeID:
			sysAcc, err := isSystemAccount(addr)
			if err != nil {
				return fmt.Errorf("checking system account: %v", err)
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
			if err := c.ipldstore.Get(ctx, act.Head, &st); err != nil {
				return fmt.Errorf("failed to load miner state: %v", err)
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
		return cid.Undef, fmt.Errorf("foreach over state tree failed: %v", err)
	}

	// Execute transfers from previous step
	// fmt.Printf("num:%v, transfers:%v\n", len(transfers), transfers)
	for _, t := range transfers {
		if err := doTransfer(tree, t.From, t.To, t.Amt); err != nil {
			return cid.Undef, fmt.Errorf("transfer %s %s->%s failed: %v", t.Amt, t.From, t.To, err)
		}
	}

	// pull up power table to give miners back some funds proportional to their power
	var ps power0.State
	powAct, find, err := tree.GetActor(ctx, builtin0.StoragePowerActorAddr)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to load power actor: %v", err)
	}

	if !find {
		return cid.Undef, errors.New("did not find power actor")
	}

	if err := c.ipldstore.Get(ctx, powAct.Head, &ps); err != nil {
		return cid.Undef, fmt.Errorf("failed to get power actor state: %v", err)
	}

	totalPower := ps.TotalBytesCommitted

	var transfersBack []transfer
	// Now, we return some funds to places where they are needed
	err = tree.ForEach(func(addr address.Address, act *types.Actor) error {
		lbact, _, err := lbtree.GetActor(ctx, addr)
		if err != nil {
			return fmt.Errorf("failed to get actor in lookback state")
		}

		prevBalance := abi.NewTokenAmount(0)
		if lbact != nil {
			prevBalance = lbact.Balance
		}

		switch act.Code {
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
			if err := c.ipldstore.Get(ctx, act.Head, &st); err != nil {
				return fmt.Errorf("failed to load miner state: %v", err)
			}

			var minfo miner0.MinerInfo
			if err := c.ipldstore.Get(ctx, st.Info, &minfo); err != nil {
				return fmt.Errorf("failed to get miner info: %v", err)
			}

			sectorsArr, err := adt0.AsArray(adt.WrapStore(ctx, c.ipldstore), st.Sectors)
			if err != nil {
				return fmt.Errorf("failed to load sectors array: %v", err)
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
					if err := c.ipldstore.Get(ctx, lbact.Head, &lbst); err != nil {
						return fmt.Errorf("failed to load miner state: %v", err)
					}

					lbsectors, err := adt0.AsArray(adt.WrapStore(ctx, c.ipldstore), lbst.Sectors)
					if err != nil {
						return fmt.Errorf("failed to load lb sectors array: %v", err)
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
		return cid.Undef, fmt.Errorf("foreach over state tree failed: %v", err)
	}

	for _, t := range transfersBack {
		if err := doTransfer(tree, t.From, t.To, t.Amt); err != nil {
			return cid.Undef, fmt.Errorf("transfer %s %s->%s failed: %v", t.Amt, t.From, t.To, err)
		}
	}

	// transfer all burnt funds back to the reserve account
	burntAct, find, err := tree.GetActor(ctx, builtin0.BurntFundsActorAddr)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to load burnt funds actor: %v", err)
	}
	if !find {
		return cid.Undef, errors.New("did not find burnt funds actor")
	}
	if err := doTransfer(tree, builtin0.BurntFundsActorAddr, builtin.ReserveAddress, burntAct.Balance); err != nil {
		return cid.Undef, fmt.Errorf("failed to unburn funds: %v", err)
	}

	// Top up the reimbursement service
	reimbAddr, err := address.NewFromString("t0111")
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to parse reimbursement service address")
	}

	reimb, find, err := tree.GetActor(ctx, reimbAddr)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to load reimbursement account actor: %v", err)
	}
	if !find {
		return cid.Undef, errors.New("did not find reimbursement actor")
	}

	difference := big.Sub(DesiredReimbursementBalance, reimb.Balance)
	if err := doTransfer(tree, builtin.ReserveAddress, reimbAddr, difference); err != nil {
		return cid.Undef, fmt.Errorf("failed to top up reimbursement account: %v", err)
	}

	// Now, a final sanity check to make sure the balances all check out
	total := abi.NewTokenAmount(0)
	err = tree.ForEach(func(addr address.Address, act *types.Actor) error {
		total = big.Add(total, act.Balance)
		return nil
	})
	if err != nil {
		return cid.Undef, fmt.Errorf("checking final state balance failed: %v", err)
	}

	exp := types.FromFil(constants.FilBase)
	if !exp.Equals(total) {
		return cid.Undef, fmt.Errorf("resultant state tree account balance was not correct: %s", total)
	}

	return tree.Flush(ctx)
}

func setNetworkName(ctx context.Context, store adt.Store, tree *vmstate.State, name string) error {
	ia, find, err := tree.GetActor(ctx, builtin0.InitActorAddr)
	if err != nil {
		return fmt.Errorf("getting init actor: %v", err)
	}
	if !find {
		return errors.New("did not find init actor")
	}

	initState, err := init_.Load(store, ia)
	if err != nil {
		return fmt.Errorf("reading init state: %v", err)
	}

	if err := initState.SetNetworkName(name); err != nil {
		return fmt.Errorf("setting network name: %v", err)
	}

	c, err := store.Put(ctx, initState)
	if err != nil {
		return fmt.Errorf("writing new init state: %v", err)
	}
	ia.Head = c

	if err := tree.SetActor(ctx, builtin0.InitActorAddr, ia); err != nil {
		return fmt.Errorf("setting init actor: %v", err)
	}

	return nil
}

// TODO: After the Liftoff epoch, refactor this to use resetMultisigVesting
func resetGenesisMsigs0(ctx context.Context, sm *ChainFork, store adt0.Store, tree *vmstate.State, startEpoch abi.ChainEpoch) error {
	gb, err := sm.cr.GetGenesisBlock(ctx)
	if err != nil {
		return fmt.Errorf("getting genesis block: %v", err)
	}

	gts, err := types.NewTipSet([]*types.BlockHeader{gb})
	if err != nil {
		return fmt.Errorf("getting genesis tipset: %v", err)
	}

	genesisTree, err := sm.StateTree(ctx, gts.Blocks()[0].ParentStateRoot)
	if err != nil {
		return fmt.Errorf("loading state tree: %v", err)
	}

	err = genesisTree.ForEach(func(addr address.Address, genesisActor *types.Actor) error {
		if genesisActor.Code == builtin0.MultisigActorCodeID {
			currActor, find, err := tree.GetActor(ctx, addr)
			if err != nil {
				return fmt.Errorf("loading actor: %v", err)
			}
			if !find {
				return fmt.Errorf("did not find actor: %s", addr.String())
			}

			var currState multisig0.State
			if err := store.Get(ctx, currActor.Head, &currState); err != nil {
				return fmt.Errorf("reading multisig state: %v", err)
			}

			currState.StartEpoch = startEpoch

			head, err := store.Put(ctx, &currState)
			if err != nil {
				return fmt.Errorf("writing new multisig state: %v", err)
			}
			currActor.Head = head

			if err := tree.SetActor(ctx, addr, currActor); err != nil {
				return fmt.Errorf("setting multisig actor: %v", err)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("iterating over genesis actors: %v", err)
	}

	return nil
}

func makeKeyAddr(splitAddr address.Address, count uint64) (address.Address, error) {
	var b bytes.Buffer
	if err := splitAddr.MarshalCBOR(&b); err != nil {
		return address.Undef, fmt.Errorf("marshalling split address: %v", err)
	}

	if err := binary.Write(&b, binary.BigEndian, count); err != nil {
		return address.Undef, fmt.Errorf("writing count into a buffer: %v", err)
	}

	if err := binary.Write(&b, binary.BigEndian, []byte("Ignition upgrade")); err != nil {
		return address.Undef, fmt.Errorf("writing fork name into a buffer: %v", err)
	}

	addr, err := address.NewActorAddress(b.Bytes())
	if err != nil {
		return address.Undef, fmt.Errorf("create actor address: %v", err)
	}

	return addr, nil
}

func splitGenesisMultisig0(ctx context.Context, addr address.Address, store adt0.Store, tree *vmstate.State, portions uint64, epoch abi.ChainEpoch) error {
	if portions < 1 {
		return fmt.Errorf("cannot split into 0 portions")
	}

	mact, find, err := tree.GetActor(ctx, addr)
	if err != nil {
		return fmt.Errorf("getting msig actor: %v", err)
	}
	if !find {
		return fmt.Errorf("did not find actor: %s", addr.String())
	}

	mst, err := multisig.Load(store, mact)
	if err != nil {
		return fmt.Errorf("getting msig state: %v", err)
	}

	signers, err := mst.Signers()
	if err != nil {
		return fmt.Errorf("getting msig signers: %v", err)
	}

	thresh, err := mst.Threshold()
	if err != nil {
		return fmt.Errorf("getting msig threshold: %v", err)
	}

	ibal, err := mst.InitialBalance()
	if err != nil {
		return fmt.Errorf("getting msig initial balance: %v", err)
	}

	se, err := mst.StartEpoch()
	if err != nil {
		return fmt.Errorf("getting msig start epoch: %v", err)
	}

	ud, err := mst.UnlockDuration()
	if err != nil {
		return fmt.Errorf("getting msig unlock duration: %v", err)
	}

	pending, err := adt0.MakeEmptyMap(store).Root()
	if err != nil {
		return fmt.Errorf("failed to create empty map: %v", err)
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
		return fmt.Errorf("storing new state: %v", err)
	}

	newActor := types.Actor{
		Code:    builtin0.MultisigActorCodeID,
		Head:    scid,
		Nonce:   0,
		Balance: big.Zero(),
	}

	i := uint64(0)
	for i < portions {
		keyAddr, err := makeKeyAddr(addr, i)
		if err != nil {
			return fmt.Errorf("creating key address: %v", err)
		}

		idAddr, err := tree.RegisterNewAddress(keyAddr)
		if err != nil {
			return fmt.Errorf("registering new address: %v", err)
		}

		err = tree.SetActor(ctx, idAddr, &newActor)
		if err != nil {
			return fmt.Errorf("setting new msig actor state: %v", err)
		}

		if err := doTransfer(tree, addr, idAddr, newIbal); err != nil {
			return fmt.Errorf("transferring split msig balance: %v", err)
		}

		i++
	}

	return nil
}

func resetMultisigVesting0(ctx context.Context, store adt0.Store, tree *vmstate.State, addr address.Address, startEpoch abi.ChainEpoch, duration abi.ChainEpoch, balance abi.TokenAmount) error {
	act, find, err := tree.GetActor(ctx, addr)
	if err != nil {
		return fmt.Errorf("getting actor: %v", err)
	}
	if !find {
		return fmt.Errorf("did not find actor: %s", addr.String())
	}

	if !builtin.IsMultisigActor(act.Code) {
		return fmt.Errorf("actor wasn't msig: %v", err)
	}

	var msigState multisig0.State
	if err := store.Get(ctx, act.Head, &msigState); err != nil {
		return fmt.Errorf("reading multisig state: %v", err)
	}

	msigState.StartEpoch = startEpoch
	msigState.UnlockDuration = duration
	msigState.InitialBalance = balance

	head, err := store.Put(ctx, &msigState)
	if err != nil {
		return fmt.Errorf("writing new multisig state: %v", err)
	}
	act.Head = head

	if err := tree.SetActor(ctx, addr, act); err != nil {
		return fmt.Errorf("setting multisig actor: %v", err)
	}

	return nil
}

func (c *ChainFork) UpgradeIgnition(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) (cid.Cid, error) {
	store := adt.WrapStore(ctx, c.ipldstore)

	if c.forkUpgrade.UpgradeLiftoffHeight <= epoch {
		return cid.Undef, fmt.Errorf("liftoff height must be beyond ignition height")
	}

	nst, err := nv3.MigrateStateTree(ctx, store, root, epoch)
	if err != nil {
		return cid.Undef, fmt.Errorf("migrating actors state: %v", err)
	}

	tree, err := c.StateTree(ctx, nst)
	if err != nil {
		return cid.Undef, fmt.Errorf("getting state tree: %v", err)
	}

	err = setNetworkName(ctx, store, tree, "ignition")
	if err != nil {
		return cid.Undef, fmt.Errorf("setting network name: %v", err)
	}

	split1, err := address.NewFromString("t0115")
	if err != nil {
		return cid.Undef, fmt.Errorf("first split address: %v", err)
	}

	split2, err := address.NewFromString("t0116")
	if err != nil {
		return cid.Undef, fmt.Errorf("second split address: %v", err)
	}

	err = resetGenesisMsigs0(ctx, c, store, tree, c.forkUpgrade.UpgradeLiftoffHeight)
	if err != nil {
		return cid.Undef, fmt.Errorf("resetting genesis msig start epochs: %v", err)
	}

	err = splitGenesisMultisig0(ctx, split1, store, tree, 50, epoch)
	if err != nil {
		return cid.Undef, fmt.Errorf("splitting first msig: %v", err)
	}

	err = splitGenesisMultisig0(ctx, split2, store, tree, 50, epoch)
	if err != nil {
		return cid.Undef, fmt.Errorf("splitting second msig: %v", err)
	}

	err = nv3.CheckStateTree(ctx, store, nst, epoch, builtin0.TotalFilecoin)
	if err != nil {
		return cid.Undef, fmt.Errorf("sanity check after ignition upgrade failed: %v", err)
	}

	return tree.Flush(ctx)
}

func (c *ChainFork) UpgradeRefuel(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) (cid.Cid, error) {
	store := adt.WrapStore(ctx, c.ipldstore)
	tree, err := c.StateTree(ctx, root)
	if err != nil {
		return cid.Undef, fmt.Errorf("getting state tree: %v", err)
	}

	err = resetMultisigVesting0(ctx, store, tree, builtin.SaftAddress, 0, 0, big.Zero())
	if err != nil {
		return cid.Undef, fmt.Errorf("tweaking msig vesting: %v", err)
	}

	err = resetMultisigVesting0(ctx, store, tree, builtin.ReserveAddress, 0, 0, big.Zero())
	if err != nil {
		return cid.Undef, fmt.Errorf("tweaking msig vesting: %v", err)
	}

	err = resetMultisigVesting0(ctx, store, tree, builtin.RootVerifierAddress, 0, 0, big.Zero())
	if err != nil {
		return cid.Undef, fmt.Errorf("tweaking msig vesting: %v", err)
	}

	return tree.Flush(ctx)
}

func linksForObj(blk ipfsblock.Block, cb func(cid.Cid)) error {
	switch multicodec.Code(blk.Cid().Prefix().Codec) {
	case multicodec.DagCbor:
		err := cbg.ScanForLinks(bytes.NewReader(blk.RawData()), cb)
		if err != nil {
			return fmt.Errorf("cbg.ScanForLinks: %v", err)
		}
		return nil
	case multicodec.Raw, multicodec.Cbor:
		// We implicitly have all children of raw/cbor blocks.
		return nil
	default:
		return fmt.Errorf("vm flush copy method only supports dag cbor")
	}
}

func copyRec(ctx context.Context, from, to blockstore.Blockstore, root cid.Cid, cp func(ipfsblock.Block) error) error {
	if root.Prefix().MhType == 0 {
		// identity cid, skip
		return nil
	}

	blk, err := from.Get(ctx, root)
	if err != nil {
		return fmt.Errorf("get %s failed: %v", root, err)
	}

	var lerr error
	err = linksForObj(blk, func(link cid.Cid) {
		if lerr != nil {
			// Theres no erorr return on linksForObj callback :(
			return
		}

		prefix := link.Prefix()
		codec := multicodec.Code(prefix.Codec)
		switch codec {
		case multicodec.FilCommitmentSealed, cid.FilCommitmentUnsealed:
			return
		}

		// We always have blocks inlined into CIDs, but we may not have their children.
		if multicodec.Code(prefix.MhType) == multicodec.Identity {
			// Unless the inlined block has no children.
			switch codec {
			case multicodec.Raw, multicodec.Cbor:
				return
			}
		} else {
			// If we have an object, we already have its children, skip the object.
			has, err := to.Has(ctx, link)
			if err != nil {
				lerr = fmt.Errorf("has: %v", err)
				return
			}
			if has {
				return
			}
		}

		if err := copyRec(ctx, from, to, link, cp); err != nil {
			lerr = err
			return
		}
	})
	if err != nil {
		return fmt.Errorf("linksForObj (%x): %v", blk.RawData(), err)
	}
	if lerr != nil {
		return lerr
	}

	if err := cp(blk); err != nil {
		return fmt.Errorf("copy: %v", err)
	}
	return nil
}

func Copy(ctx context.Context, from, to blockstore.Blockstore, root cid.Cid) error {
	ctx, span := trace.StartSpan(ctx, "vm.Copy") // nolint
	defer span.End()

	var numBlocks int
	var totalCopySize int

	const batchSize = 128
	const bufCount = 3
	freeBufs := make(chan []ipfsblock.Block, bufCount)
	toFlush := make(chan []ipfsblock.Block, bufCount)
	for i := 0; i < bufCount; i++ {
		freeBufs <- make([]ipfsblock.Block, 0, batchSize)
	}

	errFlushChan := make(chan error)

	go func() {
		for b := range toFlush {
			if err := to.PutMany(ctx, b); err != nil {
				close(freeBufs)
				errFlushChan <- fmt.Errorf("batch put in copy: %v", err)
				return
			}
			freeBufs <- b[:0]
		}
		close(errFlushChan)
		close(freeBufs)
	}()

	batch := <-freeBufs
	batchCp := func(blk ipfsblock.Block) error {
		numBlocks++
		totalCopySize += len(blk.RawData())

		batch = append(batch, blk)

		if len(batch) >= batchSize {
			toFlush <- batch
			var ok bool
			batch, ok = <-freeBufs
			if !ok {
				return <-errFlushChan
			}
		}
		return nil
	}

	if err := copyRec(ctx, from, to, root, batchCp); err != nil {
		return fmt.Errorf("copyRec: %v", err)
	}

	if len(batch) > 0 {
		toFlush <- batch
	}
	close(toFlush)        // close the toFlush triggering the loop to end
	err := <-errFlushChan // get error out or get nil if it was closed
	if err != nil {
		return err
	}

	span.AddAttributes(
		trace.Int64Attribute("numBlocks", int64(numBlocks)),
		trace.Int64Attribute("copySize", int64(totalCopySize)),
	)

	return nil
}

func (c *ChainFork) UpgradeActorsV2(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) (cid.Cid, error) {
	buf := blockstoreutil.NewTieredBstore(c.bs, blockstoreutil.NewTemporarySync())
	store := adt.WrapStore(ctx, cbor.NewCborStore(buf))

	info, err := store.Put(ctx, new(vmstate.StateInfo0))
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to create new state info for actors v2: %v", err)
	}

	newHamtRoot, err := nv4.MigrateStateTree(ctx, store, root, epoch, nv4.DefaultConfig())
	if err != nil {
		return cid.Undef, fmt.Errorf("upgrading to actors v2: %v", err)
	}

	newRoot, err := store.Put(ctx, &vmstate.StateRoot{
		Version: vmstate.StateTreeVersion1,
		Actors:  newHamtRoot,
		Info:    info,
	})
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to persist new state root: %v", err)
	}

	// perform some basic sanity checks to make sure everything still works.
	if newSm, err := vmstate.LoadState(ctx, store, newRoot); err != nil {
		return cid.Undef, fmt.Errorf("state tree sanity load failed: %v", err)
	} else if newRoot2, err := newSm.Flush(ctx); err != nil {
		return cid.Undef, fmt.Errorf("state tree sanity flush failed: %v", err)
	} else if newRoot2 != newRoot {
		return cid.Undef, fmt.Errorf("state-root mismatch: %s != %s", newRoot, newRoot2)
	} else if _, _, err := newSm.GetActor(ctx, builtin0.InitActorAddr); err != nil {
		return cid.Undef, fmt.Errorf("failed to load init actor after upgrade: %v", err)
	}

	{
		from := buf
		to := buf.Read()

		if err := Copy(ctx, from, to, newRoot); err != nil {
			return cid.Undef, fmt.Errorf("copying migrated tree: %v", err)
		}
	}

	return newRoot, nil
}

func (c *ChainFork) UpgradeLiftoff(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) (cid.Cid, error) {
	tree, err := c.StateTree(ctx, root)
	if err != nil {
		return cid.Undef, fmt.Errorf("getting state tree: %v", err)
	}

	err = setNetworkName(ctx, adt.WrapStore(ctx, c.ipldstore), tree, "mainnet")
	if err != nil {
		return cid.Undef, fmt.Errorf("setting network name: %v", err)
	}

	return tree.Flush(ctx)
}

func (c *ChainFork) UpgradeCalico(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) (cid.Cid, error) {
	if c.networkType != types.NetworkMainnet {
		return root, nil
	}

	store := adt.WrapStore(ctx, cbor.NewCborStore(c.bs))
	var stateRoot vmstate.StateRoot
	if err := store.Get(ctx, root, &stateRoot); err != nil {
		return cid.Undef, fmt.Errorf("failed to decode state root: %v", err)
	}

	if stateRoot.Version != vmstate.StateTreeVersion1 {
		return cid.Undef, fmt.Errorf(
			"expected state root version 1 for calico upgrade, got %d",
			stateRoot.Version,
		)
	}

	newHamtRoot, err := nv7.MigrateStateTree(ctx, store, stateRoot.Actors, epoch, nv7.DefaultConfig())
	if err != nil {
		return cid.Undef, fmt.Errorf("running nv7 migration: %v", err)
	}

	newRoot, err := store.Put(ctx, &vmstate.StateRoot{
		Version: stateRoot.Version,
		Actors:  newHamtRoot,
		Info:    stateRoot.Info,
	})
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to persist new state root: %v", err)
	}

	// perform some basic sanity checks to make sure everything still works.
	if newSm, err := vmstate.LoadState(ctx, store, newRoot); err != nil {
		return cid.Undef, fmt.Errorf("state tree sanity load failed: %v", err)
	} else if newRoot2, err := newSm.Flush(ctx); err != nil {
		return cid.Undef, fmt.Errorf("state tree sanity flush failed: %v", err)
	} else if newRoot2 != newRoot {
		return cid.Undef, fmt.Errorf("state-root mismatch: %s != %s", newRoot, newRoot2)
	} else if _, _, err := newSm.GetActor(ctx, builtin0.InitActorAddr); err != nil {
		return cid.Undef, fmt.Errorf("failed to load init actor after upgrade: %v", err)
	}

	return newRoot, nil
}

func terminateActor(ctx context.Context, tree *vmstate.State, addr address.Address, epoch abi.ChainEpoch) error {
	a, found, err := tree.GetActor(context.TODO(), addr)
	if err != nil {
		return fmt.Errorf("failed to get actor to delete: %v", err)
	}
	if !found {
		return types.ErrActorNotFound
	}

	if err := doTransfer(tree, addr, builtin.BurntFundsActorAddr, a.Balance); err != nil {
		return fmt.Errorf("transferring terminated actor's balance: %v", err)
	}

	err = tree.DeleteActor(ctx, addr)
	if err != nil {
		return fmt.Errorf("deleting actor from tree: %v", err)
	}

	ia, found, err := tree.GetActor(ctx, init_.Address)
	if err != nil {
		return fmt.Errorf("loading init actor: %v", err)
	}
	if !found {
		return types.ErrActorNotFound
	}

	ias, err := init_.Load(&vmstate.AdtStore{IpldStore: tree.Store}, ia)
	if err != nil {
		return fmt.Errorf("loading init actor state: %v", err)
	}

	if err := ias.Remove(addr); err != nil {
		return fmt.Errorf("deleting entry from address map: %v", err)
	}

	nih, err := tree.Store.Put(ctx, ias)
	if err != nil {
		return fmt.Errorf("writing new init actor state: %v", err)
	}

	ia.Head = nih

	return tree.SetActor(ctx, init_.Address, ia)
}

func (c *ChainFork) UpgradeActorsV3(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) (cid.Cid, error) {
	// Use all the CPUs except 3.
	workerCount := MigrationMaxWorkerCount - 3
	if workerCount <= 0 {
		workerCount = 1
	}

	cfg := nv10.Config{
		MaxWorkers:        uint(workerCount),
		JobQueueSize:      1000,
		ResultQueueSize:   100,
		ProgressLogPeriod: 10 * time.Second,
	}
	newRoot, err := c.upgradeActorsV3Common(ctx, cache, root, epoch, ts, cfg)
	if err != nil {
		return cid.Undef, fmt.Errorf("migrating actors v3 state: %v", err)
	}

	tree, err := c.StateTree(ctx, newRoot)
	if err != nil {
		return cid.Undef, fmt.Errorf("getting state tree: %v", err)
	}

	if c.networkType == types.NetworkMainnet {
		err := terminateActor(ctx, tree, types.ZeroAddress, epoch)
		if err != nil && !errors.Is(err, types.ErrActorNotFound) {
			return cid.Undef, fmt.Errorf("deleting zero bls actor: %v", err)
		}

		newRoot, err = tree.Flush(ctx)
		if err != nil {
			return cid.Undef, fmt.Errorf("flushing state tree: %v", err)
		}
	}

	return newRoot, nil
}

func (c *ChainFork) PreUpgradeActorsV3(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) error {
	log.Info("PreUpgradeActorsV3 ......")
	// Use half the CPUs for pre-migration, but leave at least 3.
	workerCount := MigrationMaxWorkerCount
	if workerCount <= 4 {
		workerCount = 1
	} else {
		workerCount /= 2
	}
	cfg := nv10.Config{MaxWorkers: uint(workerCount)}
	_, err := c.upgradeActorsV3Common(ctx, cache, root, epoch, ts, cfg)
	return err
}

func (c *ChainFork) upgradeActorsV3Common(
	ctx context.Context, cache MigrationCache,
	root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet,
	config nv10.Config,
) (cid.Cid, error) {
	buf := blockstoreutil.NewTieredBstore(c.bs, blockstoreutil.NewTemporarySync())
	store := adt.WrapStore(ctx, cbor.NewCborStore(buf))

	// Load the state root.
	var stateRoot vmstate.StateRoot
	if err := store.Get(ctx, root, &stateRoot); err != nil {
		return cid.Undef, fmt.Errorf("failed to decode state root: %v", err)
	}

	if stateRoot.Version != vmstate.StateTreeVersion1 {
		return cid.Undef, fmt.Errorf(
			"expected state root version 1 for actors v3 upgrade, got %d",
			stateRoot.Version,
		)
	}

	// Perform the migration
	newHamtRoot, err := nv10.MigrateStateTree(ctx, store, stateRoot.Actors, epoch, config, migrationLogger{}, cache)
	if err != nil {
		return cid.Undef, fmt.Errorf("upgrading to actors v3: %v", err)
	}

	// Persist the result.
	newRoot, err := store.Put(ctx, &vmstate.StateRoot{
		Version: vmstate.StateTreeVersion2,
		Actors:  newHamtRoot,
		Info:    stateRoot.Info,
	})
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to persist new state root: %v", err)
	}

	// Persist the new tree.

	{
		from := buf
		to := buf.Read()

		if err := Copy(ctx, from, to, newRoot); err != nil {
			return cid.Undef, fmt.Errorf("copying migrated tree: %v", err)
		}
	}

	return newRoot, nil
}

func (c *ChainFork) UpgradeActorsV4(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) (cid.Cid, error) {
	// Use all the CPUs except 3.
	workerCount := MigrationMaxWorkerCount - 3
	if workerCount <= 0 {
		workerCount = 1
	}

	config := nv12.Config{
		MaxWorkers:        uint(workerCount),
		JobQueueSize:      1000,
		ResultQueueSize:   100,
		ProgressLogPeriod: 10 * time.Second,
	}

	newRoot, err := c.upgradeActorsV4Common(ctx, cache, root, epoch, ts, config)
	if err != nil {
		return cid.Undef, fmt.Errorf("migrating actors v4 state: %v", err)
	}

	return newRoot, nil
}

func (c *ChainFork) PreUpgradeActorsV4(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) error {
	// Use half the CPUs for pre-migration, but leave at least 3.
	workerCount := MigrationMaxWorkerCount
	if workerCount <= 4 {
		workerCount = 1
	} else {
		workerCount /= 2
	}
	config := nv12.Config{MaxWorkers: uint(workerCount)}
	_, err := c.upgradeActorsV4Common(ctx, cache, root, epoch, ts, config)
	return err
}

func (c *ChainFork) upgradeActorsV4Common(
	ctx context.Context, cache MigrationCache,
	root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet,
	config nv12.Config,
) (cid.Cid, error) {
	buf := blockstoreutil.NewTieredBstore(c.bs, blockstoreutil.NewTemporarySync())
	store := adt.WrapStore(ctx, cbor.NewCborStore(buf))

	// Load the state root.
	var stateRoot vmstate.StateRoot
	if err := store.Get(ctx, root, &stateRoot); err != nil {
		return cid.Undef, fmt.Errorf("failed to decode state root: %v", err)
	}

	if stateRoot.Version != vmstate.StateTreeVersion2 {
		return cid.Undef, fmt.Errorf(
			"expected state root version 2 for actors v4 upgrade, got %d",
			stateRoot.Version,
		)
	}

	// Perform the migration
	newHamtRoot, err := nv12.MigrateStateTree(ctx, store, stateRoot.Actors, epoch, config, migrationLogger{}, cache)
	if err != nil {
		return cid.Undef, fmt.Errorf("upgrading to actors v4: %v", err)
	}

	// Persist the result.
	newRoot, err := store.Put(ctx, &vmstate.StateRoot{
		Version: vmstate.StateTreeVersion3,
		Actors:  newHamtRoot,
		Info:    stateRoot.Info,
	})
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to persist new state root: %v", err)
	}

	// Persist the new tree.

	{
		from := buf
		to := buf.Read()

		if err := Copy(ctx, from, to, newRoot); err != nil {
			return cid.Undef, fmt.Errorf("copying migrated tree: %v", err)
		}
	}

	return newRoot, nil
}

func (c *ChainFork) UpgradeActorsV5(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) (cid.Cid, error) {
	// Use all the CPUs except 3.
	workerCount := MigrationMaxWorkerCount - 3
	if workerCount <= 0 {
		workerCount = 1
	}

	config := nv13.Config{
		MaxWorkers:        uint(workerCount),
		JobQueueSize:      1000,
		ResultQueueSize:   100,
		ProgressLogPeriod: 10 * time.Second,
	}

	newRoot, err := c.upgradeActorsV5Common(ctx, cache, root, epoch, ts, config)
	if err != nil {
		return cid.Undef, fmt.Errorf("migrating actors v5 state: %v", err)
	}

	return newRoot, nil
}

func (c *ChainFork) PreUpgradeActorsV5(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) error {
	// Use half the CPUs for pre-migration, but leave at least 3.
	workerCount := MigrationMaxWorkerCount
	if workerCount <= 4 {
		workerCount = 1
	} else {
		workerCount /= 2
	}
	config := nv13.Config{MaxWorkers: uint(workerCount)}
	_, err := c.upgradeActorsV5Common(ctx, cache, root, epoch, ts, config)
	return err
}

func (c *ChainFork) upgradeActorsV5Common(
	ctx context.Context, cache MigrationCache,
	root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet,
	config nv13.Config,
) (cid.Cid, error) {
	buf := blockstoreutil.NewTieredBstore(c.bs, blockstoreutil.NewTemporarySync())
	store := adt.WrapStore(ctx, cbor.NewCborStore(buf))

	// Load the state root.
	var stateRoot vmstate.StateRoot
	if err := store.Get(ctx, root, &stateRoot); err != nil {
		return cid.Undef, fmt.Errorf("failed to decode state root: %v", err)
	}

	if stateRoot.Version != vmstate.StateTreeVersion3 {
		return cid.Undef, fmt.Errorf(
			"expected state root version 3 for actors v5 upgrade, got %d",
			stateRoot.Version,
		)
	}

	// Perform the migration
	newHamtRoot, err := nv13.MigrateStateTree(ctx, store, stateRoot.Actors, epoch, config, migrationLogger{}, cache)
	if err != nil {
		return cid.Undef, fmt.Errorf("upgrading to actors v5: %v", err)
	}

	// Persist the result.
	newRoot, err := store.Put(ctx, &vmstate.StateRoot{
		Version: vmstate.StateTreeVersion4,
		Actors:  newHamtRoot,
		Info:    stateRoot.Info,
	})
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to persist new state root: %v", err)
	}

	// Persist the new tree.

	{
		from := buf
		to := buf.Read()

		if err := Copy(ctx, from, to, newRoot); err != nil {
			return cid.Undef, fmt.Errorf("copying migrated tree: %v", err)
		}
	}

	return newRoot, nil
}

func (c *ChainFork) UpgradeActorsV6(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) (cid.Cid, error) {
	// Use all the CPUs except 3.
	workerCount := MigrationMaxWorkerCount - 3
	if workerCount <= 0 {
		workerCount = 1
	}

	config := nv14.Config{
		MaxWorkers:        uint(workerCount),
		JobQueueSize:      1000,
		ResultQueueSize:   100,
		ProgressLogPeriod: 10 * time.Second,
	}

	newRoot, err := c.upgradeActorsV6Common(ctx, cache, root, epoch, ts, config)
	if err != nil {
		return cid.Undef, fmt.Errorf("migrating actors v5 state: %w", err)
	}

	return newRoot, nil
}

func (c *ChainFork) PreUpgradeActorsV6(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) error {
	// Use half the CPUs for pre-migration, but leave at least 3.
	workerCount := MigrationMaxWorkerCount
	if workerCount <= 4 {
		workerCount = 1
	} else {
		workerCount /= 2
	}
	config := nv14.Config{MaxWorkers: uint(workerCount)}
	_, err := c.upgradeActorsV6Common(ctx, cache, root, epoch, ts, config)
	return err
}

func (c *ChainFork) upgradeActorsV6Common(
	ctx context.Context,
	cache MigrationCache,
	root cid.Cid, epoch abi.ChainEpoch,
	ts *types.TipSet,
	config nv14.Config,
) (cid.Cid, error) {
	buf := blockstoreutil.NewTieredBstore(c.bs, blockstoreutil.NewTemporarySync())
	store := adt.WrapStore(ctx, cbor.NewCborStore(buf))

	// Load the state root.
	var stateRoot vmstate.StateRoot
	if err := store.Get(ctx, root, &stateRoot); err != nil {
		return cid.Undef, fmt.Errorf("failed to decode state root: %w", err)
	}

	if stateRoot.Version != vmstate.StateTreeVersion4 {
		return cid.Undef, fmt.Errorf(
			"expected state root version 4 for actors v6 upgrade, got %d",
			stateRoot.Version,
		)
	}

	// Perform the migration
	newHamtRoot, err := nv14.MigrateStateTree(ctx, store, stateRoot.Actors, epoch, config, migrationLogger{}, cache)
	if err != nil {
		return cid.Undef, fmt.Errorf("upgrading to actors v5: %w", err)
	}

	// Persist the result.
	newRoot, err := store.Put(ctx, &vmstate.StateRoot{
		Version: vmstate.StateTreeVersion4,
		Actors:  newHamtRoot,
		Info:    stateRoot.Info,
	})
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to persist new state root: %w", err)
	}

	// Persist the new tree.

	{
		from := buf
		to := buf.Read()

		if err := Copy(ctx, from, to, newRoot); err != nil {
			return cid.Undef, fmt.Errorf("copying migrated tree: %w", err)
		}
	}

	return newRoot, nil
}

func (c *ChainFork) UpgradeActorsV7(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) (cid.Cid, error) {
	// Use all the CPUs except 3.
	workerCount := MigrationMaxWorkerCount - 3
	if workerCount <= 0 {
		workerCount = 1
	}

	config := nv15.Config{
		MaxWorkers:        uint(workerCount),
		JobQueueSize:      1000,
		ResultQueueSize:   100,
		ProgressLogPeriod: 10 * time.Second,
	}

	newRoot, err := c.upgradeActorsV7Common(ctx, cache, root, epoch, ts, config)
	if err != nil {
		return cid.Undef, fmt.Errorf("migrating actors v6 state: %w", err)
	}

	return newRoot, nil
}

func (c *ChainFork) PreUpgradeActorsV7(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) error {
	// Use half the CPUs for pre-migration, but leave at least 3.
	workerCount := MigrationMaxWorkerCount
	if workerCount <= 4 {
		workerCount = 1
	} else {
		workerCount /= 2
	}

	ver := c.GetNetworkVersion(ctx, epoch)
	lbts, lbRoot, err := c.cr.GetLookbackTipSetForRound(ctx, ts, epoch, ver)
	if err != nil {
		return fmt.Errorf("error getting lookback ts for premigration: %w", err)
	}

	config := nv15.Config{
		MaxWorkers:        uint(workerCount),
		ProgressLogPeriod: time.Minute * 5,
	}

	_, err = c.upgradeActorsV7Common(ctx, cache, lbRoot, epoch, lbts, config)
	return err
}

func (c *ChainFork) upgradeActorsV7Common(
	ctx context.Context,
	cache MigrationCache,
	root cid.Cid, epoch abi.ChainEpoch,
	ts *types.TipSet,
	config nv15.Config,
) (cid.Cid, error) {
	writeStore := blockstoreutil.NewAutobatch(ctx, c.bs, units.GiB/4)
	// TODO: pretty sure we'd achieve nothing by doing this, confirm in review
	// buf := blockstore.NewTieredBstore(sm.ChainStore().StateBlockstore(), writeStore)
	store := adt.WrapStore(ctx, cbor.NewCborStore(writeStore))
	// Load the state root.
	var stateRoot vmstate.StateRoot
	if err := store.Get(ctx, root, &stateRoot); err != nil {
		return cid.Undef, fmt.Errorf("failed to decode state root: %w", err)
	}

	if stateRoot.Version != vmstate.StateTreeVersion4 {
		return cid.Undef, fmt.Errorf(
			"expected state root version 4 for actors v7 upgrade, got %d",
			stateRoot.Version,
		)
	}

	// Perform the migration
	newHamtRoot, err := nv15.MigrateStateTree(ctx, store, stateRoot.Actors, epoch, config, migrationLogger{}, cache)
	if err != nil {
		return cid.Undef, fmt.Errorf("upgrading to actors v7: %w", err)
	}

	// Persist the result.
	newRoot, err := store.Put(ctx, &vmstate.StateRoot{
		Version: vmstate.StateTreeVersion4,
		Actors:  newHamtRoot,
		Info:    stateRoot.Info,
	})
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to persist new state root: %w", err)
	}

	// Persists the new tree and shuts down the flush worker
	if err := writeStore.Flush(ctx); err != nil {
		return cid.Undef, fmt.Errorf("writeStore flush failed: %w", err)
	}

	if err := writeStore.Shutdown(ctx); err != nil {
		return cid.Undef, fmt.Errorf("writeStore shutdown failed: %w", err)
	}
	return newRoot, nil
}

func (c *ChainFork) UpgradeActorsV8(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) (cid.Cid, error) {
	// Use all the CPUs except 3.
	workerCount := MigrationMaxWorkerCount - 3
	if workerCount <= 0 {
		workerCount = 1
	}

	config := nv16.Config{
		MaxWorkers:        uint(workerCount),
		JobQueueSize:      1000,
		ResultQueueSize:   100,
		ProgressLogPeriod: 10 * time.Second,
	}

	newRoot, err := c.upgradeActorsV8Common(ctx, cache, root, epoch, ts, config)
	if err != nil {
		return cid.Undef, fmt.Errorf("migrating actors v8 state: %w", err)
	}

	fmt.Print(fvmLiftoffBanner)

	return newRoot, nil
}

func (c *ChainFork) PreUpgradeActorsV8(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) error {
	// Use half the CPUs for pre-migration, but leave at least 3.
	workerCount := MigrationMaxWorkerCount
	if workerCount <= 4 {
		workerCount = 1
	} else {
		workerCount /= 2
	}

	ver := c.GetNetworkVersion(ctx, epoch)
	lbts, lbRoot, err := c.cr.GetLookbackTipSetForRound(ctx, ts, epoch, ver)
	if err != nil {
		return fmt.Errorf("error getting lookback ts for premigration: %w", err)
	}

	config := nv16.Config{
		MaxWorkers:        uint(workerCount),
		ProgressLogPeriod: time.Minute * 5,
	}

	_, err = c.upgradeActorsV8Common(ctx, cache, lbRoot, epoch, lbts, config)
	return err
}

func (c *ChainFork) upgradeActorsV8Common(
	ctx context.Context, cache MigrationCache,
	root cid.Cid,
	epoch abi.ChainEpoch,
	ts *types.TipSet,
	config nv16.Config,
) (cid.Cid, error) {
	buf := blockstoreutil.NewTieredBstore(c.bs, blockstoreutil.NewTemporarySync())
	store := adt.WrapStore(ctx, cbor.NewCborStore(buf))

	// ensure that the manifest is loaded in the blockstore
	if err := actors.LoadBundles(ctx, buf, actorstypes.Version8); err != nil {
		return cid.Undef, fmt.Errorf("failed to load manifest bundle: %w", err)
	}

	// Load the state root.
	var stateRoot vmstate.StateRoot
	if err := store.Get(ctx, root, &stateRoot); err != nil {
		return cid.Undef, fmt.Errorf("failed to decode state root: %w", err)
	}

	if stateRoot.Version != vmstate.StateTreeVersion4 {
		return cid.Undef, fmt.Errorf(
			"expected state root version 4 for actors v8 upgrade, got %d",
			stateRoot.Version,
		)
	}

	manifest, ok := actors.GetManifest(actorstypes.Version8)
	if !ok {
		return cid.Undef, fmt.Errorf("no manifest CID for v8 upgrade")
	}

	// Perform the migration
	newHamtRoot, err := nv16.MigrateStateTree(ctx, store, manifest, stateRoot.Actors, epoch, config, migrationLogger{}, cache)
	if err != nil {
		return cid.Undef, fmt.Errorf("upgrading to actors v8: %w", err)
	}

	// Persist the result.
	newRoot, err := store.Put(ctx, &vmstate.StateRoot{
		Version: vmstate.StateTreeVersion4,
		Actors:  newHamtRoot,
		Info:    stateRoot.Info,
	})
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to persist new state root: %w", err)
	}

	// Persist the new tree.

	{
		from := buf
		to := buf.Read()

		if err := Copy(ctx, from, to, newRoot); err != nil {
			return cid.Undef, fmt.Errorf("copying migrated tree: %w", err)
		}
	}

	return newRoot, nil
}

func (c *ChainFork) UpgradeActorsV9(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) (cid.Cid, error) {
	// Use all the CPUs except 3.
	workerCount := MigrationMaxWorkerCount - 3
	if workerCount <= 0 {
		workerCount = 1
	}

	config := nv17.Config{
		MaxWorkers:        uint(workerCount),
		JobQueueSize:      1000,
		ResultQueueSize:   100,
		ProgressLogPeriod: 10 * time.Second,
	}

	newRoot, err := c.upgradeActorsV9Common(ctx, cache, root, epoch, ts, config)
	if err != nil {
		return cid.Undef, fmt.Errorf("migrating actors v8 state: %w", err)
	}

	return newRoot, nil
}

func (c *ChainFork) PreUpgradeActorsV9(ctx context.Context,
	cache MigrationCache,
	root cid.Cid,
	epoch abi.ChainEpoch,
	ts *types.TipSet,
) error {
	// Use half the CPUs for pre-migration, but leave at least 3.
	workerCount := MigrationMaxWorkerCount
	if workerCount <= 4 {
		workerCount = 1
	} else {
		workerCount /= 2
	}

	ver := c.GetNetworkVersion(ctx, epoch)
	lbts, lbRoot, err := c.cr.GetLookbackTipSetForRound(ctx, ts, epoch, ver)
	if err != nil {
		return fmt.Errorf("error getting lookback ts for premigration: %w", err)
	}

	config := nv17.Config{
		MaxWorkers:        uint(workerCount),
		ProgressLogPeriod: time.Minute * 5,
	}

	_, err = c.upgradeActorsV9Common(ctx, cache, lbRoot, epoch, lbts, config)
	return err
}

func (c *ChainFork) upgradeActorsV9Common(ctx context.Context,
	cache MigrationCache,
	root cid.Cid,
	epoch abi.ChainEpoch,
	ts *types.TipSet,
	config nv17.Config,
) (cid.Cid, error) {
	writeStore := blockstoreutil.NewAutobatch(ctx, c.bs, units.GiB/4)
	store := adt.WrapStore(ctx, cbor.NewCborStore(writeStore))

	// ensure that the manifest is loaded in the blockstore
	if err := actors.LoadBundles(ctx, c.bs, actorstypes.Version9); err != nil {
		return cid.Undef, fmt.Errorf("failed to load manifest bundle: %w", err)
	}

	// Load the state root.
	var stateRoot vmstate.StateRoot
	if err := store.Get(ctx, root, &stateRoot); err != nil {
		return cid.Undef, fmt.Errorf("failed to decode state root: %w", err)
	}

	if stateRoot.Version != vmstate.StateTreeVersion4 {
		return cid.Undef, fmt.Errorf("expected state root version 4 for actors v9 upgrade, got %d", stateRoot.Version)
	}

	manifest, ok := actors.GetManifest(actorstypes.Version9)
	if !ok {
		return cid.Undef, fmt.Errorf("no manifest CID for v9 upgrade")
	}

	// Perform the migration
	newHamtRoot, err := nv17.MigrateStateTree(ctx, store, manifest, stateRoot.Actors, epoch, config,
		migrationLogger{}, cache)
	if err != nil {
		return cid.Undef, fmt.Errorf("upgrading to actors v9: %w", err)
	}

	// Persist the result.
	newRoot, err := store.Put(ctx, &vmstate.StateRoot{
		Version: vmstate.StateTreeVersion4,
		Actors:  newHamtRoot,
		Info:    stateRoot.Info,
	})
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to persist new state root: %w", err)
	}

	// Persists the new tree and shuts down the flush worker
	if err := writeStore.Flush(ctx); err != nil {
		return cid.Undef, fmt.Errorf("writeStore flush failed: %w", err)
	}

	if err := writeStore.Shutdown(ctx); err != nil {
		return cid.Undef, fmt.Errorf("writeStore shutdown failed: %w", err)
	}

	return newRoot, nil
}

func (c *ChainFork) UpgradeActorsV10(ctx context.Context,
	cache MigrationCache,
	root cid.Cid,
	epoch abi.ChainEpoch,
	ts *types.TipSet,
) (cid.Cid, error) {
	// Use all the CPUs except 3.
	workerCount := runtime.NumCPU() - 3
	if workerCount <= 0 {
		workerCount = 1
	}

	config := migration.Config{
		MaxWorkers:        uint(workerCount),
		JobQueueSize:      1000,
		ResultQueueSize:   100,
		ProgressLogPeriod: 10 * time.Second,
	}

	newRoot, err := c.upgradeActorsV10Common(ctx, cache, root, epoch, ts, config)
	if err != nil {
		return cid.Undef, fmt.Errorf("migrating actors v10 state: %w", err)
	}

	return newRoot, nil
}

func (c *ChainFork) PreUpgradeActorsV10(ctx context.Context,
	cache MigrationCache,
	root cid.Cid,
	epoch abi.ChainEpoch,
	ts *types.TipSet,
) error {
	// Use half the CPUs for pre-migration, but leave at least 3.
	workerCount := runtime.NumCPU()
	if workerCount <= 4 {
		workerCount = 1
	} else {
		workerCount /= 2
	}

	nv := c.GetNetworkVersion(ctx, epoch)
	lbts, lbRoot, err := c.cr.GetLookbackTipSetForRound(ctx, ts, epoch, nv)
	if err != nil {
		return fmt.Errorf("error getting lookback ts for premigration: %w", err)
	}

	config := migration.Config{
		MaxWorkers:        uint(workerCount),
		ProgressLogPeriod: time.Minute * 5,
	}

	_, err = c.upgradeActorsV10Common(ctx, cache, lbRoot, epoch, lbts, config)
	return err
}

func (c *ChainFork) upgradeActorsV10Common(
	ctx context.Context,
	cache MigrationCache,
	root cid.Cid,
	epoch abi.ChainEpoch,
	ts *types.TipSet,
	config migration.Config,
) (cid.Cid, error) {
	buf := blockstoreutil.NewTieredBstore(c.bs, blockstoreutil.NewTemporarySync())
	store := adt.WrapStore(ctx, cbor.NewCborStore(buf))

	// ensure that the manifest is loaded in the blockstore
	if err := actors.LoadBundles(ctx, c.bs, actorstypes.Version10); err != nil {
		return cid.Undef, fmt.Errorf("failed to load manifest bundle: %w", err)
	}

	// Load the state root.
	var stateRoot vmstate.StateRoot
	if err := store.Get(ctx, root, &stateRoot); err != nil {
		return cid.Undef, fmt.Errorf("failed to decode state root: %w", err)
	}

	if stateRoot.Version != vmstate.StateTreeVersion4 {
		return cid.Undef, fmt.Errorf(
			"expected state root version 4 for actors v9 upgrade, got %d",
			stateRoot.Version,
		)
	}

	manifest, ok := actors.GetManifest(actorstypes.Version10)
	if !ok {
		return cid.Undef, fmt.Errorf("no manifest CID for v10 upgrade")
	}

	// Perform the migration
	newHamtRoot, err := nv18.MigrateStateTree(ctx, store, manifest, stateRoot.Actors, epoch, config,
		migrationLogger{}, cache)
	if err != nil {
		return cid.Undef, fmt.Errorf("upgrading to actors v10: %w", err)
	}

	// Persist the result.
	newRoot, err := store.Put(ctx, &vmstate.StateRoot{
		Version: vmstate.StateTreeVersion5,
		Actors:  newHamtRoot,
		Info:    stateRoot.Info,
	})
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to persist new state root: %w", err)
	}

	// Persist the new tree.

	{
		from := buf
		to := buf.Read()

		if err := Copy(ctx, from, to, newRoot); err != nil {
			return cid.Undef, fmt.Errorf("copying migrated tree: %w", err)
		}
	}

	return newRoot, nil
}

func (c *ChainFork) PreUpgradeActorsV11(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) error {
	// Use half the CPUs for pre-migration, but leave at least 3.
	workerCount := MigrationMaxWorkerCount
	if workerCount <= 4 {
		workerCount = 1
	} else {
		workerCount /= 2
	}

	nv := c.GetNetworkVersion(ctx, epoch)
	lbts, lbRoot, err := c.cr.GetLookbackTipSetForRound(ctx, ts, epoch, nv)
	if err != nil {
		return fmt.Errorf("error getting lookback ts for premigration: %w", err)
	}

	config := migration.Config{
		MaxWorkers:        uint(workerCount),
		ProgressLogPeriod: time.Minute * 5,
	}

	_, err = c.upgradeActorsV11Common(ctx, cache, lbRoot, epoch, lbts, config)
	return err
}

func (c *ChainFork) UpgradeActorsV11(ctx context.Context, cache MigrationCache,
	root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) (cid.Cid, error) {
	// Use all the CPUs except 2.
	workerCount := MigrationMaxWorkerCount - 3
	if workerCount <= 0 {
		workerCount = 1
	}
	config := migration.Config{
		MaxWorkers:        uint(workerCount),
		JobQueueSize:      1000,
		ResultQueueSize:   100,
		ProgressLogPeriod: 10 * time.Second,
	}
	newRoot, err := c.upgradeActorsV11Common(ctx, cache, root, epoch, ts, config)
	if err != nil {
		return cid.Undef, fmt.Errorf("migrating actors v11 state: %w", err)
	}
	return newRoot, nil
}

func (c *ChainFork) upgradeActorsV11Common(
	ctx context.Context, cache MigrationCache,
	root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet,
	config migration.Config,
) (cid.Cid, error) {
	writeStore := blockstoreutil.NewAutobatch(ctx, c.bs, units.GiB/4)
	store := adt.WrapStore(ctx, cbor.NewCborStore(writeStore))

	// ensure that the manifest is loaded in the blockstore
	if err := actors.LoadBundles(ctx, c.bs, actorstypes.Version11); err != nil {
		return cid.Undef, fmt.Errorf("failed to load manifest bundle: %w", err)
	}

	// Load the state root.
	var stateRoot vmstate.StateRoot
	if err := store.Get(ctx, root, &stateRoot); err != nil {
		return cid.Undef, fmt.Errorf("failed to decode state root: %w", err)
	}

	if stateRoot.Version != vmstate.StateTreeVersion5 {
		return cid.Undef, fmt.Errorf(
			"expected state root version 5 for actors v11 upgrade, got %d",
			stateRoot.Version,
		)
	}

	manifest, ok := actors.GetManifest(actorstypes.Version11)
	if !ok {
		return cid.Undef, fmt.Errorf("no manifest CID for v11 upgrade")
	}

	// Perform the migration
	newHamtRoot, err := nv19.MigrateStateTree(ctx, store, manifest, stateRoot.Actors, epoch, config,
		migrationLogger{}, cache)
	if err != nil {
		return cid.Undef, fmt.Errorf("upgrading to actors v11: %w", err)
	}

	// Persist the result.
	newRoot, err := store.Put(ctx, &vmstate.StateRoot{
		Version: vmstate.StateTreeVersion5,
		Actors:  newHamtRoot,
		Info:    stateRoot.Info,
	})
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to persist new state root: %w", err)
	}

	// Persist the new tree and shuts down the flush worker
	if err := writeStore.Flush(ctx); err != nil {
		return cid.Undef, fmt.Errorf("writeStore flush failed: %w", err)
	}
	if err := writeStore.Shutdown(ctx); err != nil {
		return cid.Undef, fmt.Errorf("writeStore shutdown failed: %w", err)
	}

	return newRoot, nil
}

func (c *ChainFork) PreUpgradeActorsV12(ctx context.Context,
	cache MigrationCache,
	root cid.Cid,
	epoch abi.ChainEpoch,
	ts *types.TipSet,
) error {
	// Use half the CPUs for pre-migration, but leave at least 3.
	workerCount := MigrationMaxWorkerCount
	if workerCount <= 4 {
		workerCount = 1
	} else {
		workerCount /= 2
	}

	nv := c.GetNetworkVersion(ctx, epoch)
	lbts, lbRoot, err := c.cr.GetLookbackTipSetForRound(ctx, ts, epoch, nv)
	if err != nil {
		return fmt.Errorf("error getting lookback ts for premigration: %w", err)
	}

	config := migration.Config{
		MaxWorkers:        uint(workerCount),
		ProgressLogPeriod: time.Minute * 5,
	}

	_, err = c.upgradeActorsV12Common(ctx, cache, lbRoot, epoch, lbts, config)
	return err
}

func (c *ChainFork) UpgradeActorsV12(ctx context.Context,
	cache MigrationCache,
	root cid.Cid,
	epoch abi.ChainEpoch,
	ts *types.TipSet,
) (cid.Cid, error) {
	// Use all the CPUs except 2.
	workerCount := MigrationMaxWorkerCount - 3
	if workerCount <= 0 {
		workerCount = 1
	}
	config := migration.Config{
		MaxWorkers:        uint(workerCount),
		JobQueueSize:      1000,
		ResultQueueSize:   100,
		ProgressLogPeriod: 10 * time.Second,
	}
	newRoot, err := c.upgradeActorsV12Common(ctx, cache, root, epoch, ts, config)
	if err != nil {
		return cid.Undef, fmt.Errorf("migrating actors v11 state: %w", err)
	}
	return newRoot, nil
}

var (
	calibnetv12BuggyMinerCID1 = cid.MustParse("bafk2bzacecnh2ouohmonvebq7uughh4h3ppmg4cjsk74dzxlbbtlcij4xbzxq")
	calibnetv12BuggyMinerCID2 = cid.MustParse("bafk2bzaced7emkbbnrewv5uvrokxpf5tlm4jslu2jsv77ofw2yqdglg657uie")

	calibnetv12BuggyBundleSuffix1 = "calibrationnet-12-rc1"
	calibnetv12BuggyBundleSuffix2 = "calibrationnet-12-rc2"

	calibnetv12BuggyManifestCID1   = cid.MustParse("bafy2bzacedrunxfqta5skb7q7x32lnp4efz2oq7fn226ffm7fu5iqs62jkmvs")
	calibnetv12BuggyManifestCID2   = cid.MustParse("bafy2bzacebl4w5ptfvuw6746w7ev562idkbf5ppq72e6zub22435ws2rukzru")
	calibnetv12CorrectManifestCID1 = cid.MustParse("bafy2bzacednzb3pkrfnbfhmoqtb3bc6dgvxszpqklf3qcc7qzcage4ewzxsca")
)

func (c *ChainFork) upgradeActorsV12Common(
	ctx context.Context,
	cache MigrationCache,
	root cid.Cid,
	epoch abi.ChainEpoch,
	ts *types.TipSet,
	config migration.Config,
) (cid.Cid, error) {
	writeStore := blockstoreutil.NewAutobatch(ctx, c.bs, units.GiB/4)
	adtStore := adt.WrapStore(ctx, cbor.NewCborStore(writeStore))

	// ensure that the manifest is loaded in the blockstore
	if err := actors.LoadBundles(ctx, writeStore, actorstypes.Version12); err != nil {
		return cid.Undef, fmt.Errorf("failed to load manifest bundle: %w", err)
	}

	// Load the state root.
	var stateRoot vmstate.StateRoot
	if err := adtStore.Get(ctx, root, &stateRoot); err != nil {
		return cid.Undef, fmt.Errorf("failed to decode state root: %w", err)
	}

	if stateRoot.Version != vmstate.StateTreeVersion5 {
		return cid.Undef, fmt.Errorf(
			"expected state root version 5 for actors v12 upgrade, got %d",
			stateRoot.Version,
		)
	}

	// check whether or not this is a calibnet upgrade
	// we do this because calibnet upgraded to a "wrong" actors bundle, which was then corrected
	// we thus upgrade to calibrationnet-buggy in this upgrade
	actorsIn, err := vmstate.LoadState(ctx, adtStore, root)
	if err != nil {
		return cid.Undef, fmt.Errorf("loading state tree: %w", err)
	}

	initActor, found, err := actorsIn.GetActor(ctx, builtin.InitActorAddr)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to get system actor: %w", err)
	}
	if !found {
		return cid.Undef, fmt.Errorf("system actor %s not found", err)
	}

	var initState init11.State
	if err := adtStore.Get(ctx, initActor.Head, &initState); err != nil {
		return cid.Undef, fmt.Errorf("failed to get system actor state: %w", err)
	}

	var manifestCid cid.Cid
	if initState.NetworkName == "calibrationnet" {
		embedded, ok := actors.GetEmbeddedBuiltinActorsBundle(actorstypes.Version12, calibnetv12BuggyBundleSuffix1)
		if !ok {
			return cid.Undef, fmt.Errorf("didn't find buggy calibrationnet bundle")
		}

		var err error
		manifestCid, err = actors.LoadBundle(ctx, writeStore, bytes.NewReader(embedded))
		if err != nil {
			return cid.Undef, fmt.Errorf("failed to load buggy calibnet bundle: %w", err)
		}

		if manifestCid != calibnetv12BuggyManifestCID1 {
			return cid.Undef, fmt.Errorf("didn't find expected buggy calibnet bundle manifest: %s != %s", manifestCid, calibnetv12BuggyManifestCID1)
		}
	} else {
		var ok bool
		manifestCid, ok = actors.GetManifest(actorstypes.Version12)
		if !ok {
			return cid.Undef, fmt.Errorf("no manifest CID for v12 upgrade")
		}
	}

	// Perform the migration
	newHamtRoot, err := nv21.MigrateStateTree(ctx, adtStore, manifestCid, stateRoot.Actors, epoch, config,
		migrationLogger{}, cache)
	if err != nil {
		return cid.Undef, fmt.Errorf("upgrading to actors v12: %w", err)
	}

	// Persist the result.
	newRoot, err := adtStore.Put(ctx, &vmstate.StateRoot{
		Version: vmstate.StateTreeVersion5,
		Actors:  newHamtRoot,
		Info:    stateRoot.Info,
	})
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to persist new state root: %w", err)
	}

	// Persists the new tree and shuts down the flush worker
	if err := writeStore.Flush(ctx); err != nil {
		return cid.Undef, fmt.Errorf("writeStore flush failed: %w", err)
	}

	if err := writeStore.Shutdown(ctx); err != nil {
		return cid.Undef, fmt.Errorf("writeStore shutdown failed: %w", err)
	}

	return newRoot, nil
}

// ////////////////////
func (c *ChainFork) buildUpgradeActorsV12MinerFix(oldBuggyMinerCID, newManifestCID cid.Cid) func(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) (cid.Cid, error) {
	return func(ctx context.Context, cache MigrationCache, root cid.Cid, epoch abi.ChainEpoch, ts *types.TipSet) (cid.Cid, error) {
		stateStore := c.bs
		adtStore := adt.WrapStore(ctx, cbor.NewCborStore(stateStore))

		// ensure that the manifest is loaded in the blockstore

		// this loads the "correct" bundle for UpgradeWatermelonFix2Height
		if err := actors.LoadBundles(ctx, stateStore, actorstypes.Version12); err != nil {
			return cid.Undef, fmt.Errorf("failed to load manifest bundle: %w", err)
		}

		// this loads the second buggy bundle, for UpgradeWatermelonFixHeight
		embedded, ok := actors.GetEmbeddedBuiltinActorsBundle(actorstypes.Version12, calibnetv12BuggyBundleSuffix2)
		if !ok {
			return cid.Undef, fmt.Errorf("didn't find buggy calibrationnet bundle")
		}

		_, err := actors.LoadBundle(ctx, stateStore, bytes.NewReader(embedded))
		if err != nil {
			return cid.Undef, fmt.Errorf("failed to load buggy calibnet bundle: %w", err)
		}

		// now confirm we have the one we're migrating to
		if haveManifest, err := stateStore.Has(ctx, newManifestCID); err != nil {
			return cid.Undef, fmt.Errorf("blockstore error when loading manifest %s: %w", newManifestCID, err)
		} else if !haveManifest {
			return cid.Undef, fmt.Errorf("missing new manifest %s in blockstore", newManifestCID)
		}

		// Load input state tree
		actorsIn, err := vmstate.LoadState(ctx, adtStore, root)
		if err != nil {
			return cid.Undef, fmt.Errorf("loading state tree: %w", err)
		}

		// load old manifest data
		systemActor, found, err := actorsIn.GetActor(ctx, builtin.SystemActorAddr)
		if err != nil {
			return cid.Undef, fmt.Errorf("failed to get system actor: %w", err)
		}
		if !found {
			return cid.Undef, fmt.Errorf("system actor not found")
		}

		var systemState system11.State
		if err := adtStore.Get(ctx, systemActor.Head, &systemState); err != nil {
			return cid.Undef, fmt.Errorf("failed to get system actor state: %w", err)
		}

		var oldManifestData manifest.ManifestData
		if err := adtStore.Get(ctx, systemState.BuiltinActors, &oldManifestData); err != nil {
			return cid.Undef, fmt.Errorf("failed to get old manifest data: %w", err)
		}

		// load new manifest
		var newManifest manifest.Manifest
		if err := adtStore.Get(ctx, newManifestCID, &newManifest); err != nil {
			return cid.Undef, fmt.Errorf("error reading actor manifest: %w", err)
		}

		if err := newManifest.Load(ctx, adtStore); err != nil {
			return cid.Undef, fmt.Errorf("error loading actor manifest: %w", err)
		}

		// build the CID mapping
		codeMapping := make(map[cid.Cid]cid.Cid, len(oldManifestData.Entries))
		for _, oldEntry := range oldManifestData.Entries {
			newCID, ok := newManifest.Get(oldEntry.Name)
			if !ok {
				return cid.Undef, fmt.Errorf("missing manifest entry for %s", oldEntry.Name)
			}

			// Note: we expect newCID to be the same as oldEntry.Code for all actors except the miner actor
			codeMapping[oldEntry.Code] = newCID
		}

		// Create empty actorsOut

		actorsOut, err := vmstate.NewState(adtStore, actorsIn.Version())
		if err != nil {
			return cid.Undef, fmt.Errorf("failed to create new tree: %w", err)
		}

		// Perform the migration
		err = actorsIn.ForEach(func(a address.Address, actor *types.Actor) error {
			newCid, ok := codeMapping[actor.Code]
			if !ok {
				return fmt.Errorf("didn't find mapping for %s", actor.Code)
			}

			return actorsOut.SetActor(ctx, a, &types.Actor{
				Code:    newCid,
				Head:    actor.Head,
				Nonce:   actor.Nonce,
				Balance: actor.Balance,
				Address: actor.Address,
			})
		})
		if err != nil {
			return cid.Undef, fmt.Errorf("failed to perform migration: %w", err)
		}

		systemState.BuiltinActors = newManifest.Data
		newSystemHead, err := adtStore.Put(ctx, &systemState)
		if err != nil {
			return cid.Undef, fmt.Errorf("failed to put new system state: %w", err)
		}

		systemActor.Head = newSystemHead
		if err = actorsOut.SetActor(ctx, builtin.SystemActorAddr, systemActor); err != nil {
			return cid.Undef, fmt.Errorf("failed to put new system actor: %w", err)
		}

		// Sanity checking

		err = actorsIn.ForEach(func(a address.Address, inActor *types.Actor) error {
			outActor, found, err := actorsOut.GetActor(ctx, a)
			if err != nil {
				return fmt.Errorf("failed to get actor in outTree: %w", err)
			}
			if !found {
				return fmt.Errorf("not found actor in outTree: %s", a)
			}

			if inActor.Nonce != outActor.Nonce {
				return fmt.Errorf("mismatched nonce for actor %s", a)
			}

			if !inActor.Balance.Equals(outActor.Balance) {
				return fmt.Errorf("mismatched balance for actor %s: %d != %d", a, inActor.Balance, outActor.Balance)
			}

			if inActor.Address != outActor.Address && inActor.Address.String() != outActor.Address.String() {
				return fmt.Errorf("mismatched address for actor %s: %s != %s", a, inActor.Address, outActor.Address)
			}

			if inActor.Head != outActor.Head && a != builtin.SystemActorAddr {
				return fmt.Errorf("mismatched head for actor %s", a)
			}

			// Actor Codes are only expected to change for the miner actor
			if inActor.Code != oldBuggyMinerCID && inActor.Code != outActor.Code {
				return fmt.Errorf("unexpected change in code for actor %s", a)
			}

			return nil
		})
		if err != nil {
			return cid.Undef, fmt.Errorf("failed to sanity check migration: %w", err)
		}

		// Persist the result.
		newRoot, err := actorsOut.Flush(ctx)
		if err != nil {
			return cid.Undef, fmt.Errorf("failed to persist new state root: %w", err)
		}

		return newRoot, nil
	}
}

func (c *ChainFork) GetForkUpgrade() *config.ForkUpgradeConfig {
	return c.forkUpgrade
}

// Example upgrade function if upgrade requires only code changes
// func (c *ChainFork) upgradeActorsV9Common(
// 	ctx context.Context, cache MigrationCache,
// 	root cid.Cid,
// 	epoch abi.ChainEpoch,
// 	ts *types.TipSet,
// 	config nv16.Config,
// ) (cid.Cid, error) {
// 	buf := blockstoreutil.NewTieredBstore(c.bs, blockstoreutil.NewTemporarySync())

// 	av := actors.Version9
// 	// This may change for upgrade
// 	newStateTreeVersion := vmstate.StateTreeVersion4

// 	// ensure that the manifest is loaded in the blockstore
// 	if err := actors.LoadBundles(ctx, buf, actors.Version9); err != nil {
// 		return cid.Undef, fmt.Errorf("failed to load manifest bundle: %w", err)
// 	}

// 	newActorsManifestCid, ok := actors.GetManifest(av)
// 	if !ok {
// 		return cid.Undef, fmt.Errorf("no manifest CID for v8 upgrade")
// 	}

// 	bstore := c.bs
// 	return LiteMigration(ctx, bstore, newActorsManifestCid, root, av, vmstate.StateTreeVersion4, newStateTreeVersion)
// }

func LiteMigration(ctx context.Context, bstore blockstoreutil.Blockstore, newActorsManifestCid cid.Cid, root cid.Cid, oldAv actorstypes.Version, newAv actorstypes.Version, oldStateTreeVersion vmstate.StateTreeVersion, newStateTreeVersion vmstate.StateTreeVersion) (cid.Cid, error) {
	buf := blockstoreutil.NewTieredBstore(bstore, blockstoreutil.NewTemporarySync())
	store := adt.WrapStore(ctx, cbor.NewCborStore(buf))
	adtStore := gstStore.WrapStore(ctx, store)

	// Load the state root.
	var stateRoot vmstate.StateRoot
	if err := store.Get(ctx, root, &stateRoot); err != nil {
		return cid.Undef, fmt.Errorf("failed to decode state root: %w", err)
	}

	if stateRoot.Version != oldStateTreeVersion {
		return cid.Undef, fmt.Errorf(
			"expected state tree version %d for actors code upgrade, got %d",
			oldStateTreeVersion,
			stateRoot.Version,
		)
	}

	st, err := vmstate.LoadState(ctx, store, root)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to load state tree: %w", err)
	}

	oldManifestData, err := getManifestData(ctx, st)
	if err != nil {
		return cid.Undef, fmt.Errorf("error loading old actor manifest: %w", err)
	}

	// load new manifest
	newManifest, err := actors.LoadManifest(ctx, newActorsManifestCid, store)
	if err != nil {
		return cid.Undef, fmt.Errorf("error loading new manifest: %w", err)
	}

	newManifestData := manifest.ManifestData{}
	if err := store.Get(ctx, newManifest.Data, &newManifestData); err != nil {
		return cid.Undef, fmt.Errorf("error loading new manifest data: %w", err)
	}

	if len(oldManifestData.Entries) != len(manifest.GetBuiltinActorsKeys(oldAv)) {
		return cid.Undef, fmt.Errorf("incomplete old manifest with %d code CIDs", len(oldManifestData.Entries))
	}
	if len(newManifestData.Entries) != len(manifest.GetBuiltinActorsKeys(newAv)) {
		return cid.Undef, fmt.Errorf("incomplete new manifest with %d code CIDs", len(newManifestData.Entries))
	}

	// Maps prior version code CIDs to migration functions.
	migrations := make(map[cid.Cid]cid.Cid)

	for _, entry := range oldManifestData.Entries {
		newCodeCid, ok := newManifest.Get(entry.Name)
		if !ok {
			return cid.Undef, fmt.Errorf("code cid for %s actor not found in new manifest", entry.Name)
		}

		migrations[entry.Code] = newCodeCid
	}

	startTime := time.Now()

	// Load output state tree
	actorsOut, err := vmstate.NewState(adtStore, newStateTreeVersion)
	if err != nil {
		return cid.Undef, err
	}

	// Insert migrated records in output state tree.
	err = st.ForEach(func(addr address.Address, actorIn *types.Actor) error {
		newCid, ok := migrations[actorIn.Code]
		if !ok {
			return fmt.Errorf("new code cid not found in migrations for actor %s", addr)
		}
		var head cid.Cid
		if addr == system.Address {
			newSystemState, err := system.MakeState(store, newAv, newManifest.Data)
			if err != nil {
				return fmt.Errorf("could not make system actor state: %w", err)
			}
			head, err = store.Put(ctx, newSystemState)
			if err != nil {
				return fmt.Errorf("could not set system actor state head: %w", err)
			}
		} else {
			head = actorIn.Head
		}
		newActor := types.Actor{
			Code:    newCid,
			Head:    head,
			Nonce:   actorIn.Nonce,
			Balance: actorIn.Balance,
		}
		err = actorsOut.SetActor(ctx, addr, &newActor)
		if err != nil {
			return fmt.Errorf("could not set actor at address %s: %w", addr, err)
		}

		return nil
	})
	if err != nil {
		return cid.Undef, fmt.Errorf("failed update actor states: %w", err)
	}

	elapsed := time.Since(startTime)
	log.Infof("All done after %v. Flushing state tree root.", elapsed)
	newRoot, err := actorsOut.Flush(ctx)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to flush new actors: %w", err)
	}

	// Persist the new tree.
	{
		from := buf
		to := buf.Read()

		if err := Copy(ctx, from, to, newRoot); err != nil {
			return cid.Undef, fmt.Errorf("copying migrated tree: %w", err)
		}
	}

	return newRoot, nil
}

func getManifestData(ctx context.Context, st *vmstate.State) (*manifest.ManifestData, error) {
	wrapStore := gstStore.WrapStore(ctx, st.Store)

	systemActor, found, err := st.GetActor(ctx, system.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to get system actor: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("not found actor")
	}
	systemActorState, err := system.Load(wrapStore, systemActor)
	if err != nil {
		return nil, fmt.Errorf("failed to load system actor state: %w", err)
	}

	actorsManifestDataCid := systemActorState.GetBuiltinActors()

	var mfData manifest.ManifestData
	if err := wrapStore.Get(ctx, actorsManifestDataCid, &mfData); err != nil {
		return nil, fmt.Errorf("error fetching data: %w", err)
	}

	return &mfData, nil
}

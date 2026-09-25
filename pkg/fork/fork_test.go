package fork

import (
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/fixtures/networks"
	"github.com/filecoin-project/venus/pkg/config"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

// funcName returns the bare name of the scheduled function/method:
// "github.com/…/pkg/fork.(*ChainFork).UpgradeActorsV19-fm" → "UpgradeActorsV19".
func funcName(fn any) string {
	v := reflect.ValueOf(fn)
	if !v.IsValid() || v.IsNil() {
		return "<nil>"
	}
	name := strings.TrimSuffix(runtime.FuncForPC(v.Pointer()).Name(), "-fm")
	if i := strings.LastIndex(name, ")."); i >= 0 {
		return name[i+2:]
	}
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}

// Solstice (nv29) entries in the upgrade schedule. None of the three mistakes below is
// reported by the compiler: a missing schedule entry, a missing per-network
// UpgradeSolsticeHeight, or an entry copied from nv28 whose migration functions were
// never changed. Validate runs only once at node startup, and no other case in CI covers
// this path. Two networks pin their height positively against lotus v1.37.0-rc1: calibrationnet
// schedules it, butterflynet leaves it unscheduled (a wrong value upgrades at the wrong height,
// or activates an upgrade the network never runs).
func TestSolsticeUpgradeSchedule(t *testing.T) {
	tf.UnitTest(t)

	confs := map[string]*networks.NetworkConf{
		"mainnet":        networks.Mainnet(),
		"calibrationnet": networks.Calibration(),
		"net_2k":         networks.Net2k(),
		"forcenet":       networks.ForceNet(),
		"interopnet":     networks.InteropNet(),
		"butterflynet":   networks.ButterflySnapNet(),
		"integrationnet": networks.IntegrationNet(),
	}

	for name, conf := range confs {
		upgrade := conf.Network.ForkUpgradeParam
		height := upgrade.UpgradeSolsticeHeight
		schedule := DefaultUpgradeSchedule(nil, upgrade)
		assert.NoErrorf(t, schedule.Validate(), "%s: upgrade schedule fails Validate (upgrade heights must strictly increase)", name)

		var entry *Upgrade
		for i := range schedule {
			if schedule[i].Network == network.Version29 {
				entry = &schedule[i]
				break
			}
		}

		// Zero means the fixture left the field unset; DefaultUpgradeSchedule filters only
		// Height < 0, so 0 gets into the schedule and migrates at genesis.
		assert.NotEqualf(t, abi.ChainEpoch(0), height,
			"%s: UpgradeSolsticeHeight is 0 (the fixture left the field unset), so nv29 migrates at epoch 0", name)

		if !assert.NotNilf(t, entry,
			"%s: DefaultUpgradeSchedule has no network.Version29 entry, so the node stays on actors v18 past the upgrade height → diverges from the network", name) {
			continue
		}
		assert.Equalf(t, height, entry.Height, "%s: the nv29 schedule entry does not take the fixture height", name)

		// Pin this down positively: only the two v19 migration functions are accepted; nil and
		// any other version (v17/v18/a closure) are rejected.
		assert.Equalf(t, "UpgradeActorsV19", funcName(entry.Migration),
			"%s: nv29 Migration is not (*ChainFork).UpgradeActorsV19, got %s", name, funcName(entry.Migration))
		assert.NotEmptyf(t, entry.PreMigrations, "%s: nv29 has no pre-migration, so the upgrade would run synchronously on the block-production path", name)
		if len(entry.PreMigrations) > 0 {
			assert.Equalf(t, "PreUpgradeActorsV19", funcName(entry.PreMigrations[0].PreMigration),
				"%s: nv29 PreMigration is not (*ChainFork).PreUpgradeActorsV19, got %s",
				name, funcName(entry.PreMigrations[0].PreMigration))
		}
	}

	assert.Equalf(t, abi.ChainEpoch(4109133),
		networks.Calibration().Network.ForkUpgradeParam.UpgradeSolsticeHeight,
		"calibrationnet Solstice height must equal 4109133, the value in lotus v1.37.0-rc1 params_calibnet")

	// butterflynet is the same network as lotus butterflynet, which parks Solstice with
	// UpgradeHeightUnscheduled: a concrete height here activates nv29 on that network, where
	// lotus never migrates.
	assert.Equalf(t, config.UpgradeHeightUnscheduled,
		networks.ButterflySnapNet().Network.ForkUpgradeParam.UpgradeSolsticeHeight,
		"butterflynet Solstice height must stay unscheduled, the value in lotus v1.37.0-rc1 params_butterfly")
}

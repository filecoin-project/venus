package consensus

import (
	"sort"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/pkg/errors"
)

type ProtocolUpgrade struct {
	Version     uint64
	EffectiveAt *types.BlockHeight
}

type ProtocolUpgradeTable struct {
	Network  string
	upgrades []ProtocolUpgrade
}

func NewProtocolUpgradeTable(network string) *ProtocolUpgradeTable {
	return &ProtocolUpgradeTable{
		Network: network,
	}
}

func (put *ProtocolUpgradeTable) Add(network string, version uint64, effectiveAt *types.BlockHeight) {
	// ignore upgrade if not part of our network
	if network != put.Network {
		return
	}

	upgrade := ProtocolUpgrade{
		Version:     version,
		EffectiveAt: effectiveAt,
	}

	idx := sort.Search(len(put.upgrades), func(i int) bool {
		return effectiveAt.GreaterEqual(put.upgrades[i].EffectiveAt)
	})

	// insert upgrade sorted by effective at
	put.upgrades = append(put.upgrades, ProtocolUpgrade{})
	copy(put.upgrades[idx+1:], put.upgrades[idx:])

	put.upgrades = append(put.upgrades[:idx], append([]ProtocolUpgrade{upgrade}, put.upgrades[idx:]...)...)
}

func (put *ProtocolUpgradeTable) VersionAt(height types.BlockHeight) (uint64, error) {
	// find index of first upgrade that is yet active (or len(upgrades) if they are all active.
	idx := sort.Search(len(put.upgrades), func(i int) bool {
		return height.LessThan(put.upgrades[i].EffectiveAt)
	})

	// providing a height less than the first upgrade is an error
	if idx == 0 {
		if len(put.upgrades) == 0 {
			return 0, errors.Errorf("no protocol versions for %s network", put.Network)
		}
		return 0, errors.Errorf("chain height %s is less than effective start of first upgrade %s for network %s",
			height.String(), put.upgrades[0].EffectiveAt.String(), put.Network)
	}

	// return the upgrade just prior to the index to get the last upgrade in effect.
	return put.upgrades[idx-1].Version, nil
}

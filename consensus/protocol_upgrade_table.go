package consensus

import (
	"sort"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/pkg/errors"
)

// protocolUpgrade specifies that a particular protocol version goes into effect at a particular block height
type protocolUpgrade struct {
	Version     uint64
	EffectiveAt *types.BlockHeight
}

// ProtocolUpgradeTable is a data structure capable of specifying which protocol versions are active at which block heights
type ProtocolUpgradeTable struct {
	Network  string
	upgrades []protocolUpgrade
}

// NewProtocolUpgradeTable creates a new ProtocolUpgradeTable that only tracks upgrades for the given network
func NewProtocolUpgradeTable(network string) *ProtocolUpgradeTable {
	return &ProtocolUpgradeTable{
		Network: network,
	}
}

// Add configures an upgrade for a network. If the network doesn't match the PUT's network, this upgrade will be ignored.
func (put *ProtocolUpgradeTable) Add(network string, version uint64, effectiveAt *types.BlockHeight) {
	// ignore upgrade if not part of our network
	if network != put.Network {
		return
	}

	upgrade := protocolUpgrade{
		Version:     version,
		EffectiveAt: effectiveAt,
	}

	// add after last upgrade effectiveAt is greater than
	idx := sort.Search(len(put.upgrades), func(i int) bool {
		return effectiveAt.LessThan(put.upgrades[i].EffectiveAt)
	})

	// insert upgrade sorted by effective at
	put.upgrades = append(put.upgrades[:idx], append([]protocolUpgrade{upgrade}, put.upgrades[idx:]...)...)
}

// VersionAt returns the protocol versions at the given block height for this PUT's network.
func (put *ProtocolUpgradeTable) VersionAt(height *types.BlockHeight) (uint64, error) {
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

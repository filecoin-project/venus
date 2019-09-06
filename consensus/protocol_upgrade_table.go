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
// It must be constructed with the ProtocolUpgradeTableBuilder
type ProtocolUpgradeTable struct {
	upgrades []protocolUpgrade
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
			return 0, errors.Errorf("no protocol versions")
		}
		return 0, errors.Errorf("chain height %s is less than effective start of first upgrade %s",
			height.String(), put.upgrades[0].EffectiveAt.String())
	}

	// return the upgrade just prior to the index to get the last upgrade in effect.
	return put.upgrades[idx-1].Version, nil
}

// ProtocolUpgradeTableBuilder constructs a protocol upgrade table
type ProtocolUpgradeTableBuilder struct {
	network  string
	upgrades protocolUpgradesByEffectiveAt
}

// NewProtocolUpgradeTable creates a new ProtocolUpgradeTable that only tracks upgrades for the given network
func NewProtocolUpgradeTableBuilder(network string) *ProtocolUpgradeTableBuilder {
	return &ProtocolUpgradeTableBuilder{
		network:  network,
		upgrades: []protocolUpgrade{},
	}
}

// Add configures an upgrade for a network. If the network doesn't match the current network, this upgrade will be ignored.
func (putb *ProtocolUpgradeTableBuilder) Add(network string, version uint64, effectiveAt *types.BlockHeight) *ProtocolUpgradeTableBuilder {
	// ignore upgrade if not part of our network
	if network != putb.network {
		return putb
	}

	upgrade := protocolUpgrade{
		Version:     version,
		EffectiveAt: effectiveAt,
	}

	// insert upgrade sorted by effective at
	putb.upgrades = append(putb.upgrades, upgrade)

	return putb
}

// Build constructs a protocol upgrade table populated with properly sorted upgrades
func (putb *ProtocolUpgradeTableBuilder) Build() *ProtocolUpgradeTable {
	// sort upgrades in place
	sort.Sort(putb.upgrades)

	// copy to insure an Add doesn't alter the table
	upgrades := make([]protocolUpgrade, len(putb.upgrades))
	copy(upgrades, putb.upgrades)

	return &ProtocolUpgradeTable{upgrades: upgrades}
}

// sort methods for protocolUpgrade slice
type protocolUpgradesByEffectiveAt []protocolUpgrade

func (a protocolUpgradesByEffectiveAt) Len() int      { return len(a) }
func (a protocolUpgradesByEffectiveAt) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a protocolUpgradesByEffectiveAt) Less(i, j int) bool {
	return a[i].EffectiveAt.LessThan(a[j].EffectiveAt)
}

package version

import (
	"sort"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/pkg/errors"
)

// protocolVersion specifies that a particular protocol version goes into effect at a particular block height
type protocolVersion struct {
	Version     uint64
	EffectiveAt *types.BlockHeight
}

// ProtocolVersionTable is a data structure capable of specifying which protocol versions are active at which block heights.
// It must be constructed with the ProtocolVersionTableBuilder which enforces that the table has at least one
// entry at block height zero and that all the versions are sorted.
type ProtocolVersionTable struct {
	versions []protocolVersion
}

// VersionAt returns the protocol versions at the given block height for this PVT's network.
func (pvt *ProtocolVersionTable) VersionAt(height *types.BlockHeight) (uint64, error) {
	// find index of first version that is not yet active (or len(versions) if they are all active.
	idx := sort.Search(len(pvt.versions), func(i int) bool {
		return height.LessThan(pvt.versions[i].EffectiveAt)
	})

	// providing a height less than the first version is an error
	if idx == 0 {
		if len(pvt.versions) == 0 {
			return 0, errors.Errorf("no protocol versions")
		}
		return 0, errors.Errorf("chain height %s is less than effective start of first version %s",
			height.String(), pvt.versions[0].EffectiveAt.String())
	}

	// return the version just prior to the index to get the last version in effect.
	return pvt.versions[idx-1].Version, nil
}

// ProtocolVersionTableBuilder constructs a protocol version table
type ProtocolVersionTableBuilder struct {
	network  string
	versions protocolVersionsByEffectiveAt
}

// NewProtocolVersionTableBuilder creates a new ProtocolVersionTable that only tracks versions for the given network
func NewProtocolVersionTableBuilder(network string) *ProtocolVersionTableBuilder {
	return &ProtocolVersionTableBuilder{
		network:  network,
		versions: []protocolVersion{},
	}
}

// Add configures an version for a network. If the network doesn't match the current network, this version will be ignored.
func (pvtb *ProtocolVersionTableBuilder) Add(network string, version uint64, effectiveAt *types.BlockHeight) *ProtocolVersionTableBuilder {
	// ignore version if not part of our network
	if network != pvtb.network {
		return pvtb
	}

	protocolVersion := protocolVersion{
		Version:     version,
		EffectiveAt: effectiveAt,
	}

	pvtb.versions = append(pvtb.versions, protocolVersion)

	return pvtb
}

// Build constructs a protocol version table populated with properly sorted versions.
// It is an error to build whose first version is not at block height 0.
func (pvtb *ProtocolVersionTableBuilder) Build() (*ProtocolVersionTable, error) {
	// sort versions in place
	sort.Sort(pvtb.versions)

	// copy to insure an Add doesn't alter the table
	versions := make([]protocolVersion, len(pvtb.versions))
	copy(versions, pvtb.versions)

	// enforce that the current network has an entry at block height zero
	if len(versions) == 0 {
		return nil, errors.Errorf("no protocol versions specified for network %s", pvtb.network)
	}
	if !versions[0].EffectiveAt.Equal(types.NewBlockHeight(0)) {
		return nil, errors.Errorf("no protocol version at genesis for network %s", pvtb.network)
	}

	// enforce that version numbers increase monotonically with effective at
	lastVersion := versions[0].Version
	for _, version := range versions[1:] {
		if version.Version <= lastVersion {
			return nil, errors.Errorf("protocol version %d effective at %s is not greater than previous version, %d",
				version.Version, version.EffectiveAt.String(), lastVersion)
		}
		lastVersion = version.Version
	}

	return &ProtocolVersionTable{versions: versions}, nil
}

// sort methods for protocolVersion slice
type protocolVersionsByEffectiveAt []protocolVersion

func (a protocolVersionsByEffectiveAt) Len() int      { return len(a) }
func (a protocolVersionsByEffectiveAt) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a protocolVersionsByEffectiveAt) Less(i, j int) bool {
	if a[i].EffectiveAt.Equal(a[j].EffectiveAt) {
		return a[i].Version < a[j].Version
	}
	return a[i].EffectiveAt.LessThan(a[j].EffectiveAt)
}

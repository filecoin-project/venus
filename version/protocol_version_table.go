package version

import (
	"sort"

	"github.com/filecoin-project/go-filecoin/types"
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

// VersionAt returns the protocol versions at the given block height for this PUT's network.
func (put *ProtocolVersionTable) VersionAt(height *types.BlockHeight) (uint64, error) {
	// find index of first version that is yet active (or len(versions) if they are all active.
	idx := sort.Search(len(put.versions), func(i int) bool {
		return height.LessThan(put.versions[i].EffectiveAt)
	})

	// providing a height less than the first version is an error
	if idx == 0 {
		if len(put.versions) == 0 {
			return 0, errors.Errorf("no protocol versions")
		}
		return 0, errors.Errorf("chain height %s is less than effective start of first version %s",
			height.String(), put.versions[0].EffectiveAt.String())
	}

	// return the version just prior to the index to get the last version in effect.
	return put.versions[idx-1].Version, nil
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
func (putb *ProtocolVersionTableBuilder) Add(network string, version uint64, effectiveAt *types.BlockHeight) *ProtocolVersionTableBuilder {
	// ignore version if not part of our network
	if network != putb.network {
		return putb
	}

	protocolVersion := protocolVersion{
		Version:     version,
		EffectiveAt: effectiveAt,
	}

	// insert version sorted by effective at
	putb.versions = append(putb.versions, protocolVersion)

	return putb
}

// Build constructs a protocol version table populated with properly sorted versions.
// It is an error to build whose first version is not at block height 0.
func (putb *ProtocolVersionTableBuilder) Build() (*ProtocolVersionTable, error) {
	// sort versions in place
	sort.Sort(putb.versions)

	// copy to insure an Add doesn't alter the table
	versions := make([]protocolVersion, len(putb.versions))
	copy(versions, putb.versions)

	// enforce that the current network has an entry at block height zero
	if len(versions) == 0 {
		return nil, errors.Errorf("no protocol versions specified for network %s", putb.network)
	}
	if !versions[0].EffectiveAt.Equal(types.NewBlockHeight(0)) {
		return nil, errors.Errorf("no protocol version at genesis for network %s", putb.network)
	}

	return &ProtocolVersionTable{versions: versions}, nil
}

// sort methods for protocolVersion slice
type protocolVersionsByEffectiveAt []protocolVersion

func (a protocolVersionsByEffectiveAt) Len() int      { return len(a) }
func (a protocolVersionsByEffectiveAt) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a protocolVersionsByEffectiveAt) Less(i, j int) bool {
	return a[i].EffectiveAt.LessThan(a[j].EffectiveAt)
}

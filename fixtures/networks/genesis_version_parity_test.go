package networks

import (
	"testing"

	"github.com/filecoin-project/go-state-types/network"
	"github.com/stretchr/testify/assert"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

// TestGenesisNetworkVersionParityWithLotus pins the genesis network version of every
// network against lotus build/buildconstants, which is the authority for genesis state.
func TestGenesisNetworkVersionParityWithLotus(t *testing.T) {
	tf.UnitTest(t)

	confs := []struct {
		name string
		conf *NetworkConf
	}{
		{"2k", Net2k()},
		{"butterfly", ButterflySnapNet()},
		{"calibnet", Calibration()},
		{"mainnet", Mainnet()},
		{"interopnet", InteropNet()},
		{"forcenet", ForceNet()},
		{"integrationnet", IntegrationNet()},
	}
	for _, c := range confs {
		t.Logf("venus %-14s GenesisNetworkVersion = %d", c.name, c.conf.Network.GenesisNetworkVersion)
	}

	// devnet genesis is nv29's version, so devnet exercises the nv28 -> nv29 upgrade.
	assert.Equal(t, network.Version28, Net2k().Network.GenesisNetworkVersion)
	assert.Equal(t, network.Version28, ButterflySnapNet().Network.GenesisNetworkVersion)

	// lotus did not bump these for nv29.
	assert.Equal(t, network.Version0, Calibration().Network.GenesisNetworkVersion)
	assert.Equal(t, network.Version0, Mainnet().Network.GenesisNetworkVersion)
	assert.Equal(t, network.Version22, InteropNet().Network.GenesisNetworkVersion)
}

package networks

import (
	"fmt"
	"testing"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/stretchr/testify/assert"
)

func TestGetNetworkFromName(t *testing.T) {
	tf.UnitTest(t)

	testCast := []struct {
		name    string
		network types.NetworkType
		err     error
	}{
		{
			name:    "mainnet",
			network: types.NetworkMainnet,
			err:     nil,
		},
		{
			name:    "force",
			network: types.NetworkForce,
			err:     nil,
		},
		{
			name:    "integrationnet",
			network: types.Integrationnet,
			err:     nil,
		},
		{
			name:    "2k",
			network: types.Network2k,
			err:     nil,
		},
		{
			name:    "calibrationnet",
			network: types.NetworkCalibnet,
			err:     nil,
		},
		{
			name:    "interopnet",
			network: types.NetworkInterop,
			err:     nil,
		},
		{
			name:    "butterflynet",
			network: types.NetworkButterfly,
			err:     nil,
		},
		// {
		// 	name:    "hyperspacenet",
		// 	network: types.NetworkHyperspace,
		// 	err:     nil,
		// },
		{
			name:    "unknown",
			network: 0,
			err:     fmt.Errorf("unknown network name %s", "unknown"),
		},
	}

	for _, test := range testCast {
		network, err := GetNetworkFromName(test.name)
		assert.Equal(t, test.network, network)
		assert.Equal(t, test.err, err)
	}
}

func TestGetNetworkConfig(t *testing.T) {
	tf.UnitTest(t)

	testCast := []struct {
		name    string
		network *NetworkConf
		err     error
	}{
		{
			name:    "mainnet",
			network: Mainnet(),
			err:     nil,
		},
		{
			name:    "force",
			network: ForceNet(),
			err:     nil,
		},
		{
			name:    "integrationnet",
			network: IntegrationNet(),
			err:     nil,
		},
		{
			name:    "2k",
			network: Net2k(),
			err:     nil,
		},
		{
			name:    "calibrationnet",
			network: Calibration(),
			err:     nil,
		},
		{
			name:    "interopnet",
			network: InteropNet(),
			err:     nil,
		},
		{
			name:    "butterflynet",
			network: ButterflySnapNet(),
			err:     nil,
		},
		// {
		// 	name:    "hyperspacenet",
		// 	network: HyperspaceNet(),
		// 	err:     nil,
		// },
		{
			name:    "unknown",
			network: nil,
			err:     fmt.Errorf("unknown network name %s", "unknown"),
		},
	}

	for _, test := range testCast {
		network, err := GetNetworkConfigFromName(test.name)
		assert.Equal(t, test.network, network)
		assert.Equal(t, test.err, err)
	}
}

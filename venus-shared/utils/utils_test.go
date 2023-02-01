package utils

import (
	"context"
	"testing"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/api/chain/v1/mock"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNetworkNamtToNetworkType(t *testing.T) {
	tf.UnitTest(t)
	assert.Len(t, NetworkTypeWithNetworkName, 6)
	assert.Len(t, NetworkNameWithNetworkType, 6)
	for nt, nn := range NetworkTypeWithNetworkName {
		got, err := NetworkNameToNetworkType(nn)
		assert.Nil(t, err)
		assert.Equal(t, nt, got)
	}

	nt, err2 := NetworkNameToNetworkType("2k")
	assert.Nil(t, err2)
	assert.Equal(t, types.Network2k, nt)
}

func TestNetworkTypeToNetworkName(t *testing.T) {
	tf.UnitTest(t)
	for nt, nn := range NetworkTypeWithNetworkName {
		got := NetworkTypeToNetworkName(nt)
		assert.Equal(t, nn, got)
	}
	assert.Equal(t, types.NetworkName(""), NetworkTypeToNetworkName(types.Network2k))
}

func TestLoadBuiltinActors(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	ctrl := gomock.NewController(t)
	full := mock.NewMockFullNode(ctrl)

	for nn := range NetworkNameWithNetworkType {
		full.EXPECT().StateNetworkName(ctx).Return(nn, nil)
		assert.Nil(t, LoadBuiltinActors(ctx, full))

		for _, actorsMetadata := range actors.EmbeddedBuiltinActorsMetadata {
			if actorsMetadata.Network == string(nn) {
				for name, actor := range actorsMetadata.Actors {
					checkActorCode(t, actorsMetadata.Version, actor, name, actorsMetadata.Network)
				}
			}
		}
	}
}

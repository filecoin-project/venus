package utils

import (
	"context"
	"testing"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/api/chain/v1/mock"
	builtinactors "github.com/filecoin-project/venus/venus-shared/builtin-actors"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNetworkNamtToNetworkType(t *testing.T) {
	tf.UnitTest(t)
	for nt, nn := range TypeName {
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
	for nt, nn := range TypeName {
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

	for nn := range NameType {
		full.EXPECT().StateNetworkName(ctx).Return(nn, nil)

		assert.Nil(t, LoadBuiltinActors(ctx, full))

		for _, actorsMetadata := range builtinactors.EmbeddedBuiltinActorsMetadata {
			if actorsMetadata.Network == string(nn) {
				for name, actor := range actorsMetadata.Actors {
					res, ok := actors.GetActorCodeID(actorsMetadata.Version, name)
					assert.True(t, ok)
					assert.Equal(t, actor, res)

					_, ok2 := MethodsMap[actor]
					assert.True(t, ok2)
				}
			}
		}
	}
}

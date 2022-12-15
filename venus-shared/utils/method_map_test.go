package utils

import (
	"testing"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/stretchr/testify/assert"
)

func TestMethodMap(t *testing.T) {
	tf.UnitTest(t)

	t.Run("Default to load mainnet actors", func(t *testing.T) {
		for _, actorsMetadata := range actors.EmbeddedBuiltinActorsMetadata {
			if actorsMetadata.Network == string(types.NetworkNameMain) {
				for name, actor := range actorsMetadata.Actors {
					_, ok := MethodsMap[actor]
					if skipEvmActor(name) {
						assert.False(t, ok)
					} else {
						assert.True(t, ok)
					}
				}
			}
		}
	})

	t.Run("ReLoad butterflynet actors", func(t *testing.T) {
		for _, actorsMetadata := range actors.EmbeddedBuiltinActorsMetadata {
			if actorsMetadata.Network == string(types.NetworkNameButterfly) {
				for _, actor := range actorsMetadata.Actors {
					_, ok := MethodsMap[actor]
					assert.False(t, ok)
				}
			}
		}

		assert.Nil(t, actors.SetNetworkBundle(int(types.NetworkButterfly)))
		ReloadMethodsMap()
		for _, actorsMetadata := range actors.EmbeddedBuiltinActorsMetadata {
			if actorsMetadata.Network == string(types.NetworkNameButterfly) {
				for name, actor := range actorsMetadata.Actors {
					_, ok := MethodsMap[actor]
					if skipEvmActor(name) {
						assert.False(t, ok)
					} else {
						assert.True(t, ok)
					}
				}
			}
		}
	})
}

// 没有把 v10 actor注入，等注入后移除
func skipEvmActor(name string) bool {
	if name == actors.EamKey || name == actors.EvmKey || name == actors.EmbryoKey {
		return true
	}
	return false
}

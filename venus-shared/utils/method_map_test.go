package utils

import (
	"fmt"
	"testing"

	actortypes "github.com/filecoin-project/go-state-types/actors"
	"github.com/filecoin-project/go-state-types/manifest"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
)

func TestMethodMap(t *testing.T) {
	tf.UnitTest(t)

	t.Run("Default to load mainnet actors", func(t *testing.T) {
		for _, actorsMetadata := range actors.EmbeddedBuiltinActorsMetadata {
			if actorsMetadata.Network == string(types.NetworkNameMain) {
				for name, actor := range actorsMetadata.Actors {
					checkActorCode(t, actorsMetadata.Version, actor, name, actorsMetadata.Network)
				}
			}
		}
	})

	t.Run("ReLoad butterflynet actors", func(t *testing.T) {
		assert.Nil(t, actors.SetNetworkBundle(int(types.NetworkCalibnet)))
		ReloadMethodsMap()
		for _, actorsMetadata := range actors.EmbeddedBuiltinActorsMetadata {
			if actorsMetadata.Network == string(types.NetworkNameCalibration) {
				for name, actor := range actorsMetadata.Actors {
					checkActorCode(t, actorsMetadata.Version, actor, name, actorsMetadata.Network)
				}
			}
		}
	})
}

func checkActorCode(t *testing.T, actorVersion actortypes.Version, actorCode cid.Cid, actorName, networkName string) {
	actorKeysWithVersion := map[string]actortypes.Version{
		// DatacapKey is only available from actor v9 and above
		manifest.DatacapKey: actortypes.Version9,

		// EamKey EvmKey PlaceholderKey EthAccountKey is only available from actor v10 and above
		manifest.EamKey:         actortypes.Version10,
		manifest.EvmKey:         actortypes.Version10,
		manifest.PlaceholderKey: actortypes.Version10,
		manifest.EthAccountKey:  actortypes.Version10,
	}

	ver, ok := actorKeysWithVersion[actorName]
	if ok {
		if actorVersion < ver {
			return
		}
	}

	_, ok = MethodsMap[actorCode]
	comment := fmt.Sprintf("actor %s not found: %s %d %s", actorCode, networkName, actorVersion, actorName)
	assert.Truef(t, ok, comment)

	res, ok := actors.GetActorCodeID(actorVersion, actorName)
	assert.Truef(t, ok, comment)
	assert.Equalf(t, actorCode, res, "actor not found: name %s expect %s, actual %s", actorName, actorCode, res)
}

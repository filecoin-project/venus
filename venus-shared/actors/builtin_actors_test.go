package actors

import (
	"testing"

	actorstypes "github.com/filecoin-project/go-state-types/actors"
	"github.com/filecoin-project/go-state-types/manifest"
	"github.com/stretchr/testify/require"
)

// Test that the embedded metadata is correct.
func TestEmbeddedMetadata(t *testing.T) {
	metadata, err := ReadEmbeddedBuiltinActorsMetadata()
	require.NoError(t, err)

	for i, v1 := range metadata {
		v2 := EmbeddedBuiltinActorsMetadata[i]
		require.Equal(t, v1.Network, v2.Network)
		require.Equal(t, v1.Version, v2.Version)
		require.Equal(t, v1.ManifestCid, v2.ManifestCid)
		require.Equal(t, v1.Actors, v2.Actors)
	}
}

// Test that we're registering the manifest correctly.
func TestRegistration(t *testing.T) {
	manifestCid, found := GetManifest(actorstypes.Version8)
	require.True(t, found)
	require.True(t, manifestCid.Defined())

	for _, key := range manifest.GetBuiltinActorsKeys(actorstypes.Version8) {
		actorCid, found := GetActorCodeID(actorstypes.Version8, key)
		require.True(t, found)
		name, version, found := GetActorMetaByCode(actorCid)
		require.True(t, found)
		require.Equal(t, actorstypes.Version8, version)
		require.Equal(t, key, name)
	}
}

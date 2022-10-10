package actors

import (
	"testing"

	actorstypes "github.com/filecoin-project/go-state-types/actors"
	"github.com/stretchr/testify/require"
)

// Test that the embedded metadata is correct.
func TestEmbeddedMetadata(t *testing.T) {
	metadata, err := ReadEmbeddedBuiltinActorsMetadata()
	require.NoError(t, err)

	require.Equal(t, metadata, EmbeddedBuiltinActorsMetadata)
}

// Test that we're registering the manifest correctly.
func TestRegistration(t *testing.T) {
	manifestCid, found := GetManifest(actorstypes.Version8)
	require.True(t, found)
	require.True(t, manifestCid.Defined())

	for _, key := range GetBuiltinActorsKeys(actorstypes.Version8) {
		actorCid, found := GetActorCodeID(actorstypes.Version8, key)
		require.True(t, found)
		name, version, found := GetActorMetaByCode(actorCid)
		require.True(t, found)
		require.Equal(t, actorstypes.Version8, version)
		require.Equal(t, key, name)
	}
}

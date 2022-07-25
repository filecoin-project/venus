package actors

import (
	"testing"

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
	manifestCid, found := GetManifest(Version8)
	require.True(t, found)
	require.True(t, manifestCid.Defined())

	for _, key := range GetBuiltinActorsKeys() {
		actorCid, found := GetActorCodeID(Version8, key)
		require.True(t, found)
		name, version, found := GetActorMetaByCode(actorCid)
		require.True(t, found)
		require.Equal(t, Version8, version)
		require.Equal(t, key, name)
	}
}

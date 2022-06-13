package builtinactors

import (
	"testing"

	"github.com/filecoin-project/venus/venus-shared/actors"
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
	manifestCid, found := actors.GetManifest(actors.Version8)
	require.True(t, found)
	require.True(t, manifestCid.Defined())

	for _, key := range actors.GetBuiltinActorsKeys() {
		actorCid, found := actors.GetActorCodeID(actors.Version8, key)
		require.True(t, found)
		name, version, found := actors.GetActorMetaByCode(actorCid)
		require.True(t, found)
		require.Equal(t, actors.Version8, version)
		require.Equal(t, key, name)
	}
}

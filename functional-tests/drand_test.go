package functional

import (
	"encoding/hex"
	"testing"

	"github.com/drand/drand/core"
	"github.com/drand/drand/key"
	"github.com/drand/kyber"
	"github.com/stretchr/testify/require"
)

func TestDrandPublic(t *testing.T) {
	client := core.NewGrpcClient()
	addr := "localhost:8080"
	groupResp, err := client.Group(addr, false)
	require.NoError(t, err)

	pubKey := key.DistPublic{}
	pubKey.Coefficients = make([]kyber.Point, len(groupResp.Distkey))
	for i, k := range groupResp.Distkey {
		keyBytes, err := hex.DecodeString(k)
		require.NoError(t, err)
		pubKey.Coefficients[i] = key.KeyGroup.Point()
		pubKey.Coefficients[i].UnmarshalBinary(keyBytes)
	}

	pub, err := client.LastPublic(addr, &pubKey, false)
	require.NoError(t, err)

	t.Log(pub)
}

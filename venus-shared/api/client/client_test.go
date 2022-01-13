package client

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestAPIClient(t *testing.T) {
	t.SkipNow()
	tf.UnitTest(t)
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJBbGxvdyI6WyJyZWFkIiwid3JpdGUiLCJzaWduIiwiYWRtaW4iXX0.J9r8fhNJpf1bD4G4_tqT9UR81S5CNJHS6fD86rqxfpQ"
	addr := "ws://127.0.0.1:3453/rpc/v0"

	ctx := context.Background()
	header := http.Header{}
	header.Add("Authorization", "Bearer "+token)

	v0cli, v0close, err := NewFullRPCV0(ctx, addr, header)
	assert.Nil(t, err)
	defer v0close()

	v1cli, v1close, err := NewFullRPCV1(ctx, addr, header)
	assert.Nil(t, err)
	defer v1close()

	head, err := v0cli.ChainHead(context.Background())
	assert.Nil(t, err)
	head2, err := v1cli.ChainHead(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, head, head2)

	v0version, err := v0cli.Version(context.Background())
	assert.Nil(t, err)
	t.Log(v0version)

	v1version, err := v1cli.Version(context.Background())
	assert.Nil(t, err)
	t.Log(v1version)

	wcli, wclose, err := NewWalletRPC(ctx, addr, header)
	if err != nil {
		t.Fatal(err)
	}
	defer wclose()

	addrs := wcli.WalletAddresses(context.Background())
	t.Log(addrs)
}

package commands

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSwarmConnectPeers(t *testing.T) {
	assert := assert.New(t)

	args := []string{
		"--cmdapiaddr=:4444 --swarmlisten=/ip4/127.0.0.1/tcp/6000",
		"--cmdapiaddr=:4445 --swarmlisten=/ip4/127.0.0.1/tcp/6001",
	}

	ds := withDaemonsArgs(2, args, func() {
		id1 := getID(t, ":4444")
		id2 := getID(t, ":4445")

		t.Log("[failure] invalid peer")
		fail := run("go-filecoin swarm connect --cmdapiaddr=:4444 /ip4/hello")
		assert.NoError(fail.Error)
		assert.Equal(fail.Code, 1)
		assert.Contains(fail.ReadStderr(), "invalid peer address")

		_ = runSuccess(t, fmt.Sprintf("go-filecoin swarm connect --cmdapiaddr=:4444 /ip4/127.0.0.1/tcp/6001/ipfs/%s", id2))
		peers1 := runSuccess(t, "go-filecoin swarm peers --cmdapiaddr=:4444")
		peers2 := runSuccess(t, "go-filecoin swarm peers --cmdapiaddr=:4445")

		t.Log("[success] 1 -> 2")
		assert.Contains(peers1.ReadStdout(), id2)

		t.Log("[success] 2 -> 1")
		assert.Contains(peers2.ReadStdout(), id1)
	})
	for _, d := range ds {
		assert.NoError(d.Error)
		assert.Equal(d.Code, 0)
		assert.Empty(d.ReadStderr())
	}
}

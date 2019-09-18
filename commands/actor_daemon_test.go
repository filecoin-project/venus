package commands_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/commands"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/node/test"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestActorDaemon(t *testing.T) {
	tf.IntegrationTest(t)

	t.Run("actor ls --enc json returns NDJSON containing all actors in the state tree", func(t *testing.T) {
		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		op1 := d.RunSuccess("actor", "ls", "--enc", "json")
		result1 := op1.ReadStdoutTrimNewlines()

		var avs []commands.ActorView
		for _, line := range bytes.Split([]byte(result1), []byte{'\n'}) {
			requireSchemaConformance(t, line, "actor_ls")

			// unmarshall JSON to actor view an add to slice
			var av commands.ActorView
			err := json.Unmarshal(line, &av)
			require.NoError(t, err)
			avs = append(avs, av)
		}

		assert.NotZero(t, len(avs))

		// The order of actors is consistent, but only within builds of genesis.car.
		// We just want to make sure the views have something valid in them.
		for _, av := range avs {
			assert.Contains(t, []string{"StoragemarketActor", "AccountActor", "PaymentbrokerActor", "MinerActor", "BootstrapMinerActor", "InitActor"}, av.ActorType)
			if av.ActorType == "AccountActor" {
				assert.Zero(t, len(av.Exports))
			} else {
				assert.NotZero(t, len(av.Exports))
			}
		}
	})
}

func TestActorPower(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()

	var mockSigner, _ = types.NewMockSignersAndKeyInfo(6)
	addr1, addr2, addr3 := mockSigner.Addresses[0], mockSigner.Addresses[1], mockSigner.Addresses[2]
	// create a valid miner
	minerAddr1, minerAddr2, minerAddr3 := mockSigner.Addresses[3], mockSigner.Addresses[4], mockSigner.Addresses[5]

	testGen := consensus.MakeGenesisFunc(
		consensus.ActorAccount(addr1, types.NewAttoFILFromFIL(10000)),
		consensus.ActorAccount(addr2, types.NewAttoFILFromFIL(10000)),
		consensus.ActorAccount(addr3, types.NewAttoFILFromFIL(10000)),
		consensus.MinerActor(minerAddr1, addr1, th.RequireRandomPeerID(t), types.ZeroAttoFIL, types.OneKiBSectorSize),
		consensus.MinerActor(minerAddr2, addr2, th.RequireRandomPeerID(t), types.NewAttoFILFromFIL(10), types.OneKiBSectorSize),
		consensus.MinerActor(minerAddr3, addr3, th.RequireRandomPeerID(t), types.NewAttoFILFromFIL(100), types.OneKiBSectorSize),
	)

	// Create node (it's not necessary to start it).
	b := test.NewNodeBuilder(t)
	node := b.WithGenesisInit(testGen).
		WithInitOpt(node.AutoSealIntervalSecondsOpt(120)).
		Build(ctx)
	require.NoError(t, node.ChainReader.Load(ctx))

	// Run the command API.
	cmd, stop := test.RunNodeAPI(ctx, node, t)
	defer stop()

	out := cmd.RunSuccess(ctx, "actor", "power-table").ReadStdout()
	assert.Contains(t, out, minerAddr1.String())
	assert.Contains(t, out, minerAddr2.String())
	assert.Contains(t, out, minerAddr3.String())
}

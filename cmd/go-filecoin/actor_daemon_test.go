package commands_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestActorDaemon(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()
	t.Run("actor ls --enc json returns NDJSON containing all actors in the state tree", func(t *testing.T) {
		builder := test.NewNodeBuilder(t)
		n := builder.BuildAndStart(ctx)
		defer n.Stop(ctx)
		cmdClient, done := test.RunNodeAPI(ctx, n, t)
		defer done()

		op1 := cmdClient.RunSuccess(ctx, "actor", "ls", "--enc", "json")
		result1 := op1.ReadStdoutTrimNewlines()

		var avs []commands.ActorView
		for _, line := range bytes.Split([]byte(result1), []byte{'\n'}) {
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
			assert.Contains(t, []string{"StoragemarketActor", "AccountActor", "PaymentbrokerActor", "PowerActor", "MinerActor", "BootstrapMinerActor", "InitActor"}, av.ActorType)
		}
	})
}

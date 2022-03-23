package cmd_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/filecoin-project/venus/cmd"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/app/node/test"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestActorDaemon(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()
	t.Run("state ls --enc json returns NDJSON containing all actors in the state tree", func(t *testing.T) {
		builder := test.NewNodeBuilder(t)

		_, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		op1 := cmdClient.RunSuccess(ctx, "state", "list-actor", "--enc", "json")
		result1 := op1.ReadStdoutTrimNewlines()

		var avs []cmd.ActorView
		for _, line := range bytes.Split([]byte(result1), []byte{'\n'}) {
			// unmarshall JSON to actor view an add to slice
			var av cmd.ActorView
			err := json.Unmarshal(line, &av)
			require.NoError(t, err)
			avs = append(avs, av)
		}

		assert.NotZero(t, len(avs))
	})
}

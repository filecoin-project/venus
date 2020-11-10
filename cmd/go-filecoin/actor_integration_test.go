package commands_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commands "github.com/filecoin-project/venus/cmd/go-filecoin"
	"github.com/filecoin-project/venus/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
)

func TestActorDaemon(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()
	t.Run("actor ls --enc json returns NDJSON containing all actors in the state tree", func(t *testing.T) {
		builder := test.NewNodeBuilder(t)

		_, cmdClient, done := builder.BuildAndStartAPI(ctx)
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
	})
}

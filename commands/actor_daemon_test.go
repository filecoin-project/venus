package commands

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/filecoin-project/go-filecoin/api"
	th "github.com/filecoin-project/go-filecoin/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActorDaemon(t *testing.T) {
	t.Parallel()
	t.Run("actor ls --enc json returns NDJSON containing all actors in the state tree", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		op1 := d.RunSuccess("actor", "ls", "--enc", "json")
		result1 := op1.ReadStdoutTrimNewlines()

		var avs []api.ActorView
		for _, line := range bytes.Split([]byte(result1), []byte{'\n'}) {
			requireSchemaConformance(t, line, "actor_ls")

			// unmarshall JSON to actor view an add to slice
			var av api.ActorView
			err := json.Unmarshal(line, &av)
			require.NoError(err)
			avs = append(avs, av)
		}

		assert.NotZero(len(avs))

		// Expect the second actor to be an account actor and to have no exports.
		// This checks that a bug that gave all actors the same as the first has been fixed.
		assert.Equal("AccountActor", avs[1].ActorType)
		assert.Equal(0, len(avs[1].Exports))
	})
}

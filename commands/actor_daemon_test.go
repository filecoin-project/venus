package commands_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/commands"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
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
			assert.Contains(t, []string{"StoragemarketActor", "AccountActor", "PaymentbrokerActor", "MinerActor", "BootstrapMinerActor"}, av.ActorType)
			if av.ActorType == "AccountActor" {
				assert.Zero(t, len(av.Exports))
			} else {
				assert.NotZero(t, len(av.Exports))
			}
		}
	})
}

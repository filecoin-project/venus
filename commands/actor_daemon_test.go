package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xeipuuv/gojsonschema"
)

func TestActorDaemon(t *testing.T) {
	t.Run("actor ls --enc json returns NDJSON containing all actors in the state tree", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		d := NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		op1 := d.RunSuccess("actor", "ls", "--enc", "json")
		result1 := op1.ReadStdoutTrimNewlines()

		wd, _ := os.Getwd()
		schemaLoader := gojsonschema.NewReferenceLoader("file://" + wd + "/schema/actor_ls.schema.json")

		var avs []actorView
		for _, line := range bytes.Split([]byte(result1), []byte{'\n'}) {
			// test that json conforms to our schema
			jsonLoader := gojsonschema.NewBytesLoader(line)
			result, err := gojsonschema.Validate(schemaLoader, jsonLoader)
			require.NoError(err)

			assert.True(result.Valid())
			for _, desc := range result.Errors() {
				t.Errorf("- %s\n", desc)
			}

			// unmarshall JSON to actor view an add to slice
			var av actorView
			err = json.Unmarshal(line, &av)
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

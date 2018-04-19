package commands

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActorDaemon(t *testing.T) {
	t.Run("actor ls --enc json returns NDJSON containing all actors in the state tree", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		d := NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		op1 := d.RunSuccess("actor", "ls", "--enc", "json")
		result1 := op1.ReadStdoutTrimNewlines()

		var avs []actorView
		for _, line := range bytes.Split([]byte(result1), []byte{'\n'}) {
			var av actorView
			err := json.Unmarshal(line, &av)
			require.NoError(err)
			avs = append(avs, av)
		}

		assert.NotZero(len(avs))
	})
}

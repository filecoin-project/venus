package commands

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-filecoin/config"
	th "github.com/filecoin-project/go-filecoin/testhelpers"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

func TestConfigDaemon(t *testing.T) {
	t.Run("config <key> prints config value", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)

		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		op1 := d.RunSuccess("config", "datastore")
		jsonOut := op1.ReadStdout()
		wrapped1 := config.NewDefaultConfig().Datastore
		var decodedOutput1 config.DatastoreConfig
		err := json.Unmarshal([]byte(jsonOut), &decodedOutput1)
		require.NoError(err)
		assert.Equal(wrapped1, &decodedOutput1)

		op2 := d.RunSuccess("config", "datastore.path")
		jsonOut = op2.ReadStdout()
		assert.Equal(fmt.Sprintf("%q\n", config.NewDefaultConfig().Datastore.Path), jsonOut)
	})

	t.Run("config <key> simple_value updates config", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		period := "1m"

		d.RunSuccess("config", "bootstrap.period", period)
		op1 := d.RunSuccess("config", "bootstrap.period")

		// validate output
		jsonOut := op1.ReadStdout()
		bootstrapConfig := config.NewDefaultConfig().Bootstrap
		assert.Equal(fmt.Sprintf("\"%s\"\n", period), jsonOut)

		// validate config write
		cfg := d.Config()
		assert.Equal(cfg.Bootstrap, bootstrapConfig)
	})

	t.Run("config <key> <val> updates config", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)

		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		d.RunSuccess("config", "bootstrap", `{"addresses": ["fake1", "fake2"], "period": "1m", "minPeerThreshold": 0}`)
		op1 := d.RunSuccess("config", "bootstrap")

		// validate output
		jsonOut := op1.ReadStdout()
		bootstrapConfig := config.NewDefaultConfig().Bootstrap
		bootstrapConfig.Addresses = []string{"fake1", "fake2"}
		someJSON, err := json.MarshalIndent(bootstrapConfig, "", "\t")
		require.NoError(err)
		assert.Equal(fmt.Sprintf("%s\n", string(someJSON)), jsonOut)

		// validate config write
		cfg := d.Config()
		assert.Equal(cfg.Bootstrap, bootstrapConfig)
	})
}

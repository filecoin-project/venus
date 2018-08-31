package commands

import (
	"bytes"
	"encoding/json"
	"testing"

	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChainDaemon(t *testing.T) {
	t.Parallel()
	t.Run("chain ls returns the whole chain", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)

		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		op1 := d.RunSuccess("mining", "once", "--enc", "text")
		result1 := op1.ReadStdoutTrimNewlines()
		c, err := cid.Parse(result1)
		require.NoError(err)

		op2 := d.RunSuccess("chain", "ls", "--enc", "json")
		result2 := op2.ReadStdoutTrimNewlines()

		var bs [][]types.Block
		for _, line := range bytes.Split([]byte(result2), []byte{'\n'}) {
			var b []types.Block
			err := json.Unmarshal(line, &b)
			require.NoError(err)
			bs = append(bs, b)
			require.Equal(1, len(b))
			line = bytes.TrimPrefix(line, []byte{'['})
			line = bytes.TrimSuffix(line, []byte{']'})

			// ensure conformance with JSON schema
			requireSchemaConformance(t, line, "filecoin_block")
		}

		assert.Equal(2, len(bs))
		assert.True(bs[1][0].Parents.Empty())
		assert.True(c.Equals(bs[0][0].Cid()))
	})

	t.Run("chain head with chain of size 1 returns genesis block", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)

		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		op := d.RunSuccess("chain", "ls", "--enc", "json")
		result := op.ReadStdoutTrimNewlines()

		var b []types.Block
		err := json.Unmarshal([]byte(result), &b)
		require.NoError(err)

		assert.True(b[0].Parents.Empty())
	})
}

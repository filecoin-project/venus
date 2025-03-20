// stm: ignore
// Only tests external library behavior, therefore it should not be annotated
package beacon

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/config"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"

	dchain "github.com/drand/drand/v2/common/chain"
	hclient "github.com/drand/go-clients/client/http"
	"github.com/stretchr/testify/assert"
)

func TestPrintGroupInfo(t *testing.T) {
	tf.UnitTest(t)
	server := config.DrandConfigs[config.DrandDevnet].Servers[0]
	chainInfo := config.DrandConfigs[config.DrandDevnet].ChainInfoJSON
	drandChain, err := dchain.InfoFromJSON(bytes.NewReader([]byte(chainInfo)))
	assert.NoError(t, err)
	c, err := hclient.NewWithInfo(&logger{&log.SugaredLogger}, server, drandChain, nil)
	assert.NoError(t, err)
	chain, err := c.FetchChainInfo(context.Background(), nil)
	assert.NoError(t, err)
	err = chain.ToJSON(os.Stdout, nil)
	assert.NoError(t, err)
}

func TestMaxBeaconRoundForEpoch(t *testing.T) {
	tf.UnitTest(t)
	todayTS := uint64(1652222222)
	drandCfg := config.DrandConfigs[config.DrandDevnet]
	db, err := NewDrandBeacon(todayTS, config.NewDefaultConfig().NetworkParams.BlockDelay, drandCfg)
	assert.NoError(t, err)
	assert.True(t, db.IsChained())
	mbr15 := db.MaxBeaconRoundForEpoch(network.Version15, 100)
	mbr16 := db.MaxBeaconRoundForEpoch(network.Version16, 100)
	assert.Equal(t, mbr15+1, mbr16)
}

func TestQuicknetIsChained(t *testing.T) {
	tf.UnitTest(t)
	todayTS := uint64(1652222222)
	drandCfg := config.DrandConfigs[config.DrandQuicknet]
	db, err := NewDrandBeacon(todayTS, config.NewDefaultConfig().NetworkParams.BlockDelay, drandCfg)
	assert.NoError(t, err)
	assert.False(t, db.IsChained())
}

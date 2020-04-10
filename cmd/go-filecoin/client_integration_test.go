package commands_test

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"testing"

	commands "github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestProposeDeal(t *testing.T) {
	tf.IntegrationTest(t)

	// setup 2 nodes: bootstrap, client
	// bootstrap: miner stats
	// client: create file, import file, get cid
	// client; propose storage deal

	ctx := context.Background()
	nodes, cancel := test.MustCreateNodesWithBootstrap(ctx, t, 1)
	defer cancel()

	miner := nodes[0]

	maddr, err := miner.BlockMining.BlockMiningAPI.MinerAddress()
	require.NoError(t, err)

	mstats, err := miner.PorcelainAPI.MinerGetStatus(ctx, maddr, miner.PorcelainAPI.ChainHeadKey())
	require.NoError(t, err)

	client := nodes[1]

	// create a temp file to import
	tmpFile, err := ioutil.TempFile(os.TempDir(), "test_propose_deal-")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Example writing to the file
	if _, err = tmpFile.Write([]byte("he was way behind and he was in a bind and willing to make a deal.")); err != nil {
		log.Fatal("Failed to write to temporary file", err)
	}

	node, err := client.PorcelainAPI.DAGImportData(ctx, tmpFile)
	require.NoError(t, err)

	req, err := cmds.NewRequest(ctx, []string{"client", "propose-storage-deal"}, cmdkit.OptMap{}, []string{
		mstats.ActorAddress.String(),
		node.Cid().String(),
		"1000",
		"2000",
		".0001",
		"1",
	}, nil, commands.RootCmd)
	require.NoError(t, err)

	emitter := &TestEmitter{}
	err = commands.ClientProposeStorageDealCmd.Run(req, emitter, commands.CreateServerEnv(ctx, client))
	require.NoError(t, err)
	require.NoError(t, emitter.err)
}

type TestEmitter struct {
	value interface{}
	err   error
}

func (t *TestEmitter) SetLength(_ uint64) {}
func (t *TestEmitter) Close() error       { return nil }

func (t *TestEmitter) CloseWithError(err error) error {
	t.err = err
	return nil
}

func (t *TestEmitter) Emit(value interface{}) error {
	t.value = value
	return nil
}

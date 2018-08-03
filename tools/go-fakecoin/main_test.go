package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"
	bserv "gx/ipfs/QmTfTKeBhTLjSjxXQsjkF2b1DfZmYEMnknGE2y2gX57C6v/go-blockservice"
	"gx/ipfs/QmZxjqR9Qgompju73kakSoUj3rbVndAzky3oCDiBNCxPs1/go-ipfs-exchange-offline"
	"gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"
	//	"gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"

	//	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddFakeChain(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	var length = 9
	ctx := context.Background()
	r := repo.NewInMemoryRepo()

	bs := blockstore.NewBlockstore(r.Datastore())
	cst := &hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}
	genesis, err := consensus.InitGenesis(cst, bs)
	require.NoError(err)
	genCid := genesis.Cid()
	genTS, err := consensus.NewTipSet(genesis)
	require.NoError(err)

	chainStore := chain.NewDefaultStore(r.Datastore(), cst, genCid)
	defer chainStore.Stop()
	require.NoError(chainStore.PutTipSetAndState(ctx, &chain.TipSetAndState{
		TipSet:          genTS,
		TipSetStateRoot: genesis.StateRoot,
	}))
	require.NoError(chainStore.SetHead(ctx, genTS))
	require.NoError(chainStore.Load(ctx))

	// binom == false
	assert.NoError(fake(ctx, length, false, chainStore))
	h, err := chainStore.Head().Height()
	assert.NoError(err)
	assert.Equal(uint64(9), h)

	// binom == true
	assert.NoError(fake(ctx, length, true, chainStore))
	_, err = chainStore.Head().Height()
	assert.NoError(err)
}

func GetFakecoinBinary() (string, error) {
	bin := filepath.FromSlash(fmt.Sprintf("%s/src/github.com/filecoin-project/go-filecoin/tools/go-fakecoin/go-fakecoin", os.Getenv("GOPATH")))
	_, err := os.Stat(bin)
	if err == nil {
		return bin, nil
	}

	if os.IsNotExist(err) {
		return "", fmt.Errorf("You are missing the fakecoin binary...try building, searched in '%s'", bin)
	}

	return "", err
}

var testRepoPath = filepath.FromSlash("/tmp/fakecoin/")

func TestCommandsSucceed(t *testing.T) {
	t.Skip("TODO: flaky test")
	require := require.New(t)

	os.RemoveAll(testRepoPath)       // go-filecoin init will fail if repo exists.
	defer os.RemoveAll(testRepoPath) // clean up when we're done.

	bin, err := GetFakecoinBinary()
	require.NoError(err)

	// 'go-fakecoin actors' completes without error. (runs init, so must be the first)
	cmdActors := exec.Command(bin, "actors", "-repodir", testRepoPath)
	out, err := cmdActors.CombinedOutput()
	require.NoError(err, string(out))

	// 'go-fakecoin fake' completes without error.
	cmdFake := exec.Command(bin, "fake", "-repodir", testRepoPath)
	out, err = cmdFake.CombinedOutput()

	require.NoError(err, string(out))
}

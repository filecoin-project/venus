package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddFakeChain(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	var length = 9
	var gbbCount, pbCount int
	ctx := context.Background()

	getHeaviestTipSet := func() core.TipSet {
		gbbCount++
		return core.RequireNewTipSet(require, new(types.Block))
	}
	processBlock := func(context context.Context, block *types.Block) (core.BlockProcessResult, error) {
		pbCount++
		return 0, nil
	}
	loadState := func(context context.Context, ts core.TipSet) (state.Tree, error) {
		return state.NewEmptyStateTree(hamt.NewCborStore()), nil
	}
	fake(ctx, length, false, getHeaviestTipSet, processBlock, loadState)
	assert.Equal(1, gbbCount)
	assert.Equal(length, pbCount)
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

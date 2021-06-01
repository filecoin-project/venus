package node_test

import (
	"runtime"
	"testing"

	"github.com/filecoin-project/venus/app/node"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/repo"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestRuntime(t *testing.T) {
	tf.UnitTest(t)

	mr := repo.NewInMemoryRepo()
	g := node.NewInspectorAPI(mr)
	rt := g.Runtime()

	assert.Equal(t, runtime.GOOS, rt.OS)
	assert.Equal(t, runtime.GOARCH, rt.Arch)
	assert.Equal(t, runtime.Version(), rt.Version)
	assert.Equal(t, runtime.Compiler, rt.Compiler)
	assert.Equal(t, runtime.NumCPU(), rt.NumProc)
	assert.Equal(t, runtime.GOMAXPROCS(0), rt.GoMaxProcs)
	assert.Equal(t, runtime.NumCgoCall(), rt.NumCGoCalls)
}

func TestDisk(t *testing.T) {
	tf.UnitTest(t)

	mr := repo.NewInMemoryRepo()
	g := node.NewInspectorAPI(mr)
	d, err := g.Disk()

	assert.NoError(t, err)
	assert.Equal(t, uint64(0), d.Free)
	assert.Equal(t, uint64(0), d.Total)
	assert.Equal(t, "0", d.FSType)
}

func TestMemory(t *testing.T) {
	tf.UnitTest(t)

	mr := repo.NewInMemoryRepo()
	g := node.NewInspectorAPI(mr)

	_, err := g.Memory()
	assert.NoError(t, err)
}

func TestConfig(t *testing.T) {
	tf.UnitTest(t)

	mr := repo.NewInMemoryRepo()
	g := node.NewInspectorAPI(mr)
	c := g.Config()

	defCfg := config.NewDefaultConfig()
	defCfg.Wallet.PassphraseConfig = config.TestPassphraseConfig()
	assert.Equal(t, defCfg, c)
}

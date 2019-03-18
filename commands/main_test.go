package commands

import (
	"context"
	"os"
	"os/exec"
	"path"
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	"github.com/filecoin-project/go-filecoin/testhelpers"

	"gx/ipfs/Qmf46mr235gtyxizkKUkTH5fo62Thza2zwXR4DWC7rkoqF/go-ipfs-cmds"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
)

func TestRequiresDaemon(t *testing.T) {
	assert := assert.New(t)

	reqWithDaemon, err := cmds.NewRequest(context.Background(), []string{}, nil, []string{"chain"}, nil, chainCmd)
	assert.NoError(err)

	reqWithoutDaemon, err := cmds.NewRequest(context.Background(), []string{}, nil, []string{"daemon"}, nil, daemonCmd)
	assert.NoError(err)

	assert.True(requiresDaemon(reqWithDaemon))
	assert.False(requiresDaemon(reqWithoutDaemon))
}

func TestNoDaemonNoHang(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Start the daemon to initialize a new repo
	d := testhelpers.NewDaemon(t).Start()

	// rename the lock files to a safe place
	repoDir := d.RepoDir()
	require.NoError(os.Rename(path.Join(repoDir, "api"), path.Join(repoDir, "api.backup")))
	require.NoError(os.Rename(path.Join(repoDir, "repo.lock"), path.Join(repoDir, "repo.lock.backup")))

	// shut down the daemon
	d.Stop()

	// put the lock files back
	require.NoError(os.Rename(path.Join(repoDir, "api.backup"), path.Join(repoDir, "api")))
	require.NoError(os.Rename(path.Join(repoDir, "repo.lock.backup"), path.Join(repoDir, "repo.lock")))

	// run actor ls with the old repo that still has the lock file, but no running daemon
	out, _ := exec.Command(testhelpers.MustGetFilecoinBinary(), "--repodir", repoDir, "actor", "ls").CombinedOutput()

	assert.Contains(string(out), "Is the daemon running?")
}

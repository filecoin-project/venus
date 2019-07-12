package commands_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strings"
	"testing"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"

	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multiaddr-net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {
	tf.IntegrationTest(t)

	commit := getCodeCommit(t)

	verOut, err := exec.Command(th.MustGetFilecoinBinary(), "version").Output()
	require.NoError(t, err)

	version := string(verOut)
	assert.Exactly(t, version, fmt.Sprintf("commit: %s", commit))
}

func TestVersionOverHttp(t *testing.T) {
	tf.IntegrationTest(t)

	td := th.NewDaemon(t).Start()
	defer td.ShutdownSuccess()

	maddr, err := multiaddr.NewMultiaddr(td.CmdAddr())
	require.NoError(t, err)

	_, host, err := manet.DialArgs(maddr)
	require.NoError(t, err)

	url := fmt.Sprintf("http://%s/api/version", host)
	req, err := http.NewRequest("POST", url, nil)
	require.NoError(t, err)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)

	commit := strings.Trim(getCodeCommit(t), "\n ")
	expected := fmt.Sprintf("{\"Commit\":\"%s\"}\n", commit)

	defer res.Body.Close() // nolint: errcheck
	body, err := ioutil.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, expected, string(body))
}

func getCodeCommit(t *testing.T) string {
	var gitOut []byte
	var err error
	gitArgs := []string{"rev-parse", "--verify", "HEAD"}
	if gitOut, err = exec.Command("git", gitArgs...).Output(); err != nil {
		assert.NoError(t, err)
	}
	return string(gitOut)
}

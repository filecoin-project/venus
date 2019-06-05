package commands_test

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestVersion(t *testing.T) {
	tf.IntegrationTest(t)

	var gitOut, verOut []byte
	var err error
	gitArgs := []string{"rev-parse", "--verify", "HEAD"}
	if gitOut, err = exec.Command("git", gitArgs...).Output(); err != nil {
		assert.NoError(t, err)
	}
	commit := string(gitOut)

	if verOut, err = exec.Command(th.MustGetFilecoinBinary(), "version").Output(); err != nil {
		assert.NoError(t, err)
	}
	version := string(verOut)
	assert.Exactly(t, version, fmt.Sprintf("commit: %s", commit))
}

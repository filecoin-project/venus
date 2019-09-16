package journal_test

import (
	"io/ioutil"
	"path"
	"path/filepath"
	"testing"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/journal"
	"github.com/filecoin-project/go-filecoin/repo"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJournal(t *testing.T) {
	tf.UnitTest(t)
	container, err := ioutil.TempDir("", "container")
	require.NoError(t, err)
	defer repo.RequireRemoveAll(t, container)
	repoPath := path.Join(container, "repo")

	cfg := config.NewDefaultConfig()
	assert.NoError(t, err, repo.InitFSRepo(repoPath, 42, cfg))

	_, err = repo.OpenFSRepo(repoPath, 42)
	assert.NoError(t, err)

	err = journal.InitJournal(repoPath, false)
	assert.NoError(t, err)

	t.Run("test", func(t *testing.T) {
		journal.Record("test", "message", "a", 1, "b", 2)
		journalText := getJournalText(filepath.Join(repoPath, "journal/test.json"))
		assert.Contains(t, journalText, `"a":1`)
		assert.Contains(t, journalText, `"b":2`)
	})
}

func getJournalText(path string) string {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(content)
}

package helpers

import "testing"

func TestShowGitInfo(t *testing.T) {
	t.Log("commitId:", GetCommitSha())
	t.Log("tag:", GetLastTag())
}

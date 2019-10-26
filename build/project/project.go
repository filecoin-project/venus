package project

import (
	"path/filepath"

	"github.com/filecoin-project/go-filecoin/build/flags"
	"github.com/filecoin-project/go-filecoin/build/internal/helpers"
)

// Root return the project root joined with any path fragments
func Root(paths ...string) string {
	if flags.GitRoot == "" {
		// load the root if flag not present
		// Note: in some environments (i.e. IDE's) it wont be present
		flags.GitRoot = helpers.GetGitRoot()
	}
	allPaths := append([]string{flags.GitRoot}, paths...)
	return filepath.Join(allPaths...)
}

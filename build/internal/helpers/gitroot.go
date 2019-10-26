package helpers

import (
	"os/exec"
	"strings"
)

// GetGitRoot return the project root joined with any path fragments
func GetGitRoot() string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic("could not find git root")
	}

	return strings.Trim(string(out), "\n")
}

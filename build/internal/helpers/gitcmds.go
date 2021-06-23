package helpers

import (
	"log"
	"os/exec"
	"strings"
)

var lineBreak = "\n"

// GetCommitSha get commit hash
func GetCommitSha() string {
	return runCapture("git log -n 1 --format=%H")
}

// GetLastTag get last tag
func GetLastTag() string {
	lastTag := runCapture("git rev-list --tags --max-count=1")
	return runCapture("git describe --tags " + lastTag)
}

func runCapture(name string) string {
	args := strings.Split(name, " ")
	cmd := exec.Command(args[0], args[1:]...) // #nosec
	log.Println(name)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Command '%s' failed: %s\n", name, err)
	}

	return strings.Trim(string(output), lineBreak)
}

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var lineBreak = "\n"

func init() {
	log.SetFlags(0)
	if runtime.GOOS == "windows" {
		lineBreak = "\r\n"
	}
}

// run executes a given command on the shell, like
// `run("git status")`
func run(name string) string {
	args := strings.Split(name, " ")
	return runParts(args...)
}

func runParts(args ...string) string {
	name := strings.Join(args, " ")
	cmd := exec.Command(args[0], args[1:]...) // #nosec
	log.Println(name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("%s", out)
		log.Fatalf("Command '%s' failed: %s\n", name, err)
	}

	return strings.Trim(string(out), lineBreak)
}

// deps installs all dependencies
func deps() {
	log.Println("Installing dependencies...")

	list := []string{
		"go get -u github.com/whyrusleeping/gx",
		"go get -u github.com/whyrusleeping/gx-go",
		"./fetch_go-ipfs_deps.sh",
		"gx install",
		"go get -u github.com/alecthomas/gometalinter",
		"gometalinter --install",
		"go get -u github.com/stretchr/testify",
	}

	for _, name := range list {
		log.Println(run(name))
	}
}

// lint runs linting using gometalinter
func lint(packages []string) {
	if len(packages) == 0 {
		packages = []string{"./..."}
	}

	log.Printf("Linting %s ...\n", strings.Join(packages, " "))

	// Run fast linters batched together
	configs := []string{
		"gometalinter",
		"--skip=sharness",
		"--disable-all",
	}

	fastLinters := []string{
		"--enable=vet",
		"--enable=vetshadow",
		"--enable=gofmt",
		"--enable=misspell",
		"--enable=goconst",
		"--enable=golint",
		"--enable=errcheck",
		"--min-occurrences=6", // for goconst
	}

	log.Println(runParts(append(append(configs, fastLinters...), packages...)...))

	slowLinters := []string{
		"--deadline=10m",
		"--enable=unconvert",
		"--enable=gosimple",
		"--enable=megacheck",
		"--enable=dupl",
		"--enable=varcheck",
		"--enable=structcheck",
		"--enable=dupl",
		"--enable=deadcode",
	}

	log.Println(runParts(append(append(configs, slowLinters...), packages...)...))
}

func build() {
	log.Println("Building...")

	commit := run("git log -n 1 --format=%H")

	log.Println(
		runParts(
			"go", "build",
			"-ldflags", fmt.Sprintf("-X github.com/filecoin-project/go-filecoin/flags.Commit=%s", commit),
			"-v", "-o", "go-filecoin", ".",
		),
	)
}

// test executes tests and passes along all additional arguments to `go test`.
func test(args []string) {
	log.Println("Testing...")

	log.Println(run(fmt.Sprintf("go test ./... %s", strings.Join(args, " "))))
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		log.Fatalf("Missing command")
	}

	cmd := args[0]

	switch cmd {
	case "deps":
		deps()
	case "lint":
		lint(args[1:])
	case "build":
		build()
	case "test":
		test(args[1:])
	default:
		log.Fatalf("Unknown command: %s\n", cmd)
	}
}

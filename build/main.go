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
		"gx install",
		"go get -u github.com/alecthomas/gometalinter",
		"gometalinter --install",
		"go get -u github.com/onsi/gomega",
		"go get -u github.com/stretchr/testify",
	}

	for _, name := range list {
		log.Println(run(name))
	}
}

// lint runs linting using gometalinter
func lint() {
	log.Println("Linting...")

	log.Println(run("gometalinter --config=metalint.json ./..."))
}

func build() {
	log.Println("Building...")

	commit := run("git log -n 1 --format=%h")

	log.Println(
		runParts(
			"go", "build",
			fmt.Sprintf(`-ldflags="-X=main.Version=%s"`, commit),
			"-v", "-o", "go-filecoin", ".",
		),
	)
}

// test executes tests and passes along all additonal arguments to `go test`.
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
		lint()
	case "build":
		build()
	case "test":
		test(args[1:])
	default:
		log.Fatalf("Unknown command: %s\n", cmd)
	}
}

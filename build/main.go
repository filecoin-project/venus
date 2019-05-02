package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/filecoin-project/go-filecoin/util/version"
)

var lineBreak = "\n"

func init() {
	log.SetFlags(0)
	if runtime.GOOS == "windows" {
		lineBreak = "\r\n"
	}
	// We build with go modules.
	if err := os.Setenv("GO111MODULE", "on"); err != nil {
		fmt.Println("Failed to set GO111MODULE env")
		os.Exit(1)
	}
}

// command is a structure representing a shell command to be run in the
// specified directory
type command struct {
	dir   string
	parts []string
}

// cmd creates a new command using the pwd and its cwd
func cmd(parts ...string) command {
	return cmdWithDir("./", parts...)
}

// cmdWithDir creates a new command using the specified directory as its cwd
func cmdWithDir(dir string, parts ...string) command {
	return command{
		dir:   dir,
		parts: parts,
	}
}

func runCmd(c command) {
	parts := c.parts
	if len(parts) == 1 {
		parts = strings.Split(parts[0], " ")
	}

	name := strings.Join(parts, " ")
	cmd := exec.Command(parts[0], parts[1:]...) // #nosec
	cmd.Dir = c.dir
	log.Println(name)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, err = io.Copy(os.Stderr, stderr); err != nil {
			panic(err)
		}
	}()
	go func() {
		defer wg.Done()
		if _, err = io.Copy(os.Stdout, stdout); err != nil {
			panic(err)
		}
	}()

	if err := cmd.Start(); err != nil {
		panic(err)
	}

	wg.Wait()
	if err := cmd.Wait(); err != nil {
		log.Fatalf("Command '%s' failed: %s\n", name, err)
	}
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

// deps installs all dependencies
func deps() {
	runCmd(cmd("pkg-config --version"))

	log.Println("Installing dependencies...")

	cmds := []command{
		// Download all go modules. While not strictly necessary (go
		// will do this automatically), this:
		//  1. Makes it easier to cache dependencies in CI.
		//  2. Makes it possible to fetch all deps ahead of time for
		//     offline development.
		cmd("go mod download"),
		// Download and build proofs.
		cmd("./scripts/install-rust-fil-proofs.sh"),
		cmd("./scripts/install-bls-signatures.sh"),
		cmd("./scripts/install-filecoin-parameters.sh"),
	}

	for _, c := range cmds {
		runCmd(c)
	}
}

// lint runs linting using golangci-lint
func lint(packages ...string) {
	if len(packages) == 0 {
		packages = []string{"./..."}
	}

	log.Printf("Linting %s ...\n", strings.Join(packages, " "))

	runCmd(cmd("go", "run", "github.com/golangci/golangci-lint/cmd/golangci-lint", "run"))
}

func build() {
	buildFilecoin()
	buildGengen()
	buildFaucet()
	buildGenesisFileServer()
	generateGenesis()
	buildMigrations()
}

func forcebuild() {
	forceBuildFC()
	buildGengen()
	buildFaucet()
	buildGenesisFileServer()
	generateGenesis()
	buildMigrations()
}

func forceBuildFC() {
	log.Println("Force building go-filecoin...")

	runCmd(cmd([]string{
		"go", "build",
		"-ldflags", fmt.Sprintf("-X github.com/filecoin-project/go-filecoin/flags.Commit=%s", getCommitSha()),
		"-a", "-v", "-o", "go-filecoin", ".",
	}...))
}

func generateGenesis() {
	log.Println("Generating genesis...")
	runCmd(cmd([]string{
		"./gengen/gengen",
		"--keypath", "fixtures/live",
		"--out-car", "fixtures/live/genesis.car",
		"--out-json", "fixtures/live/gen.json",
		"--config", "./fixtures/setup.json",
	}...))
	runCmd(cmd([]string{
		"./gengen/gengen",
		"--keypath", "fixtures/test",
		"--out-car", "fixtures/test/genesis.car",
		"--out-json", "fixtures/test/gen.json",
		"--config", "./fixtures/setup.json",
		"--test-proofs-mode",
	}...))
}

func buildFilecoin() {
	log.Println("Building go-filecoin...")

	runCmd(cmd([]string{
		"go", "build",
		"-ldflags", fmt.Sprintf("-X github.com/filecoin-project/go-filecoin/flags.Commit=%s", getCommitSha()),
		"-v", "-o", "go-filecoin", ".",
	}...))
}

func buildGengen() {
	log.Println("Building gengen utils...")

	runCmd(cmd([]string{"go", "build", "-o", "./gengen/gengen", "./gengen"}...))
}

func buildFaucet() {
	log.Println("Building faucet...")

	runCmd(cmd([]string{"go", "build", "-o", "./tools/faucet/faucet", "./tools/faucet/"}...))
}

func buildGenesisFileServer() {
	log.Println("Building genesis file server...")

	runCmd(cmd([]string{"go", "build", "-o", "./tools/genesis-file-server/genesis-file-server", "./tools/genesis-file-server/"}...))
}

func buildMigrations() {
	log.Println("Building migrations...")
	runCmd(cmd([]string{
		"go", "build", "-o", "./tools/migration/go-filecoin-migrate", "./tools/migration/main.go"}...))
}

func install() {
	log.Println("Installing...")

	runCmd(cmd("go", "install", "-ldflags", fmt.Sprintf(`"-X github.com/filecoin-project/go-filecoin/flags.Commit=%s"`, getCommitSha())))
}

// test executes tests and passes along all additional arguments to `go test`.
func test(args ...string) {
	log.Println("Testing...")

	parallelism, ok := os.LookupEnv("TEST_PARALLELISM")

	if !ok {
		parallelism = "8"
	}

	packages, ok := os.LookupEnv("TEST_PACKAGES")

	if !ok {
		packages = "./..."
	}

	runCmd(cmd(fmt.Sprintf("go test -timeout 30m -parallel %s %s %s",
		parallelism, strings.Replace(packages, "\n", " ", -1), strings.Join(args, " "))))
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		log.Fatalf("Missing command")
	}

	if !version.Check(runtime.Version()) {
		log.Fatalf("Invalid go version: %s", runtime.Version())
	}

	cmd := args[0]

	switch cmd {
	case "deps", "smartdeps":
		deps()
	case "lint":
		lint(args[1:]...)
	case "build-filecoin":
		buildFilecoin()
	case "build-gengen":
		buildGengen()
	case "generate-genesis":
		generateGenesis()
	case "build-migrations":
		buildMigrations()
	case "build":
		build()
	case "fbuild":
		forcebuild()
	case "test":
		test(args[1:]...)
	case "install":
		install()
	case "best":
		build()
		test(args[1:]...)
	case "all":
		deps()
		lint()
		build()
		test(args[1:]...)
	default:
		log.Fatalf("Unknown command: %s\n", cmd)
	}
}

func getCommitSha() string {
	commit := runCapture("git log -n 1 --format=%H")
	if os.Getenv("FILECOIN_OVERRIDE_BUILD_SHA") != "" {
		commit = os.Getenv("FILECOIN_OVERRIDE_BUILD_SHA")
	}
	return commit
}

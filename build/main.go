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
		cmd("./scripts/install-rust-fil-proofs.sh"),
		cmd("./scripts/install-bls-signatures.sh"),
		cmd("./proofs/bin/paramfetch --all --json=./proofs/misc/parameters.json"),
		cmd("./proofs/bin/paramcache"),
		cmd("./scripts/copy-groth-params.sh"),
	}

	for _, c := range cmds {
		runCmd(c)
	}
}

// lint runs linting using gometalinter
func lint(packages ...string) {
	if len(packages) == 0 {
		packages = []string{"./..."}
	}

	log.Printf("Linting %s ...\n", strings.Join(packages, " "))

	// Run fast linters batched together
	configs := []string{
		"golangci-lint",
		"run",
		"--no-config",
		"--skip-dirs=sharness,vendor",
		"--disable-all",
	}

	fastLinters := []string{
		"--enable=vet",
		"--enable=gofmt",
		"--enable=misspell",
		"--enable=goconst",
		"--enable=golint",
		"--enable=errcheck",
	}

	runCmd(cmd(append(append(configs, fastLinters...), packages...)...))

	slowLinters := []string{
		"--deadline=10m",
		"--enable=unconvert",
		"--enable=staticcheck",
		"--enable=varcheck",
		"--enable=structcheck",
		"--enable=deadcode",
	}

	runCmd(cmd(append(append(configs, slowLinters...), packages...)...))
}

func build() {
	buildFilecoin()
	buildGengen()
	buildFaucet()
	buildGenesisFileServer()
	generateGenesis()
}

func forcebuild() {
	forceBuildFC()
	buildGengen()
	buildFaucet()
	buildGenesisFileServer()
	generateGenesis()
}

func forceBuildFC() {
	log.Println("Force building go-filecoin...")

	commit := runCapture("git log -n 1 --format=%H")

	runCmd(cmd([]string{
		"go", "build",
		"-ldflags", fmt.Sprintf("-X github.com/filecoin-project/go-filecoin/flags.Commit=%s", commit),
		"-a", "-v", "-o", "go-filecoin", ".",
	}...))
}

func generateGenesis() {
	log.Println("Generating genesis...")
	runCmd(cmd([]string{
		"./gengen/gengen",
		"--keypath", "fixtures",
		"--out-car", "fixtures/genesis.car",
		"--out-json", "fixtures/gen.json",
		"--config", "./fixtures/setup.json",
	}...))
}

func buildFilecoin() {
	log.Println("Building go-filecoin...")

	commit := runCapture("git log -n 1 --format=%H")

	runCmd(cmd([]string{
		"go", "build",
		"-ldflags", fmt.Sprintf("-X github.com/filecoin-project/go-filecoin/flags.Commit=%s", commit),
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

func install() {
	log.Println("Installing...")

	runCmd(cmd("go install"))
}

// test executes tests and passes along all additional arguments to `go test`.
func test(args ...string) {
	log.Println("Testing...")

	runCmd(cmd(fmt.Sprintf("go test -timeout 30m -parallel 8 ./... %s", strings.Join(args, " "))))
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
	case "deps":
		deps()
	case "lint":
		lint(args[1:]...)
	case "build-filecoin":
		buildFilecoin()
	case "build-gengen":
		buildGengen()
	case "generate-genesis":
		generateGenesis()
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

package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	pf "github.com/filecoin-project/go-paramfetch"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/build/internal/helpers"
	"github.com/filecoin-project/go-filecoin/build/internal/version"
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

	runCmd(cmd("go mod download"))

	dat, err := ioutil.ReadFile("./parameters.json")
	if err != nil {
		panic(errors.Wrap(err, "failed to read contents of ./parameters.json"))
	}

	err = pf.GetParams(dat, 1024)
	if err != nil {
		panic(errors.Wrap(err, "failed to acquire Groth parameters for 1KiB sectors"))
	}

	runCmd(cmd("./scripts/install-filecoin-ffi.sh"))
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
	buildPrereleaseTool()
}

func forcebuild() {
	forceBuildFC()
	buildGengen()
	buildFaucet()
	buildGenesisFileServer()
	generateGenesis()
	buildMigrations()
	buildPrereleaseTool()
}

func forceBuildFC() {
	log.Println("Force building go-filecoin...")

	runCmd(cmd([]string{
		"bash", "-c", fmt.Sprintf("go build %s -a -v -o go-filecoin .", flags()),
	}...))
}

// cleanDirectory removes the child of a directly wihtout removing the directory itself, unlike `RemoveAll`.
// There is also an additional parameter to ignore dot files which is important for directories which are normally
// empty. Git has no concept of directories, so for a directory to automatically be created on checkout, a file must
// exist in side of it. We use this pattern in a few places, so the need to keep the dot files around is impotant.
func cleanDirectory(dir string, ignoredots bool) error {
	if abs := filepath.IsAbs(dir); !abs {
		return fmt.Errorf("Directory %s is not an absolute path, could not clean directory", dir)
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		fname := file.Name()
		if ignoredots && []rune(fname)[0] == '.' {
			continue
		}

		fpath := filepath.Join(dir, fname)

		fmt.Println("Removing", fpath)
		if err := os.RemoveAll(fpath); err != nil {
			return err
		}
	}

	return nil
}

func generateGenesis() {
	log.Println("Generating genesis...")

	liveFixtures, err := filepath.Abs("./fixtures/live")
	if err != nil {
		panic(err)
	}

	if err := cleanDirectory(liveFixtures, true); err != nil {
		panic(err)
	}

	runCmd(cmd([]string{
		"./tools/gengen/gengen",
		"--keypath", liveFixtures,
		"--out-car", filepath.Join(liveFixtures, "genesis.car"),
		"--out-json", filepath.Join(liveFixtures, "gen.json"),
		"--config", "./fixtures/setup.json",
	}...))

	testFixtures, err := filepath.Abs("./fixtures/test")
	if err != nil {
		panic(err)
	}

	if err := cleanDirectory(testFixtures, true); err != nil {
		panic(err)
	}

	runCmd(cmd([]string{
		"./tools/gengen/gengen",
		"--keypath", testFixtures,
		"--out-car", filepath.Join(testFixtures, "genesis.car"),
		"--out-json", filepath.Join(testFixtures, "gen.json"),
		"--config", "./fixtures/setup.json",
		"--test-proofs-mode",
	}...))
}

func flags() string {
	return fmt.Sprintf("-ldflags=github.com/filecoin-project/go-filecoin=\"%s\"", strings.Join([]string{
		fmt.Sprintf("-X github.com/filecoin-project/go-filecoin/build/flags.GitRoot=%s", helpers.GetGitRoot()),
		fmt.Sprintf("-X github.com/filecoin-project/go-filecoin/build/flags.GitCommit=%s", getCommitSha()),
	}, " "))
}

func buildFilecoin() {
	log.Println("Building go-filecoin...")

	runCmd(cmd([]string{
		"bash", "-c", fmt.Sprintf("go build %s -v -o go-filecoin .", flags()),
	}...))
}

func buildGengen() {
	log.Println("Building gengen utils...")

	runCmd(cmd([]string{"go", "build", "-o", "./tools/gengen/gengen", "./tools/gengen"}...))
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

func buildPrereleaseTool() {
	log.Println("Building prerelease-tool...")

	runCmd(cmd([]string{"go", "build", "-o", "./tools/prerelease-tool/prerelease-tool", "./tools/prerelease-tool/"}...))
}

func install() {
	log.Println("Installing...")

	runCmd(cmd(
		"bash", "-c", fmt.Sprintf("go install %s", flags()),
	))
}

// test executes tests and passes along all additional arguments to `go test`.
func test(userArgs ...string) {
	log.Println("Running tests...")

	// Consult environment for test packages, in order to support CI container-level parallelism.
	packages, ok := os.LookupEnv("TEST_PACKAGES")
	if !ok {
		packages = "./..."
	}

	begin := time.Now()
	runCmd(cmd(
		"bash", "-c", fmt.Sprintf("go test %s %s",
			strings.Replace(packages, "\n", " ", -1),
			strings.Join(userArgs, " "))))
	end := time.Now()
	log.Printf("Tests finished in %.1f seconds\n", end.Sub(begin).Seconds())
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
	return runCapture("git log -n 1 --format=%H")
}

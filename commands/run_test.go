package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

// Output manages running, inprocess, a filecoin command.
type Output struct {
	lk sync.Mutex
	// Input is the the raw input we got.
	Input string
	// Args is the cleaned up version of the input.
	Args []string
	// Code is the unix style exit code, set after the command exited.
	Code int
	// Error is the error returned from the command, after it exited.
	Error  error
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	stdout []byte
	Stderr io.ReadCloser
	stderr []byte

	test testing.TB
}

func (o *Output) Close(code int, err error) {
	o.lk.Lock()
	defer o.lk.Unlock()

	o.Code = code
	o.Error = err
}

func (o *Output) ReadStderr() string {
	o.lk.Lock()
	defer o.lk.Unlock()

	return string(o.stderr)
}

func (o *Output) ReadStdout() string {
	o.lk.Lock()
	defer o.lk.Unlock()

	return string(o.stdout)
}

func (o *Output) readStdoutTrimNewlines() string {
	return strings.Trim(o.ReadStdout(), "\n")
}

func runSuccessFirstLine(td *TestDaemon, args ...string) string {
	return runSuccessLines(td, args...)[0]
}

func runSuccessLines(td *TestDaemon, args ...string) []string {
	output := td.RunSuccess(args...)
	result := output.readStdoutTrimNewlines()
	return strings.Split(result, "\n")
}

type TestDaemon struct {
	cmdAddr   string
	swarmAddr string
	repoDir   string

	init bool

	// The filecoin daemon process
	process *exec.Cmd

	lk     sync.Mutex
	Stdin  io.Writer
	Stdout io.Reader
	Stderr io.Reader

	test *testing.T
}

func (td *TestDaemon) Run(args ...string) *Output {
	td.test.Helper()
	return td.RunWithStdin(nil, args...)
}

func (td *TestDaemon) RunWithStdin(stdin io.Reader, args ...string) *Output {
	td.test.Helper()
	bin, err := GetFilecoinBinary()
	require.NoError(td.test, err)

	// handle Run("cmd subcmd")
	if len(args) == 1 {
		args = strings.Split(args[0], " ")
	}

	finalArgs := append(args, "--repodir="+td.repoDir, "--cmdapiaddr="+td.cmdAddr)

	td.test.Logf("run: %q", strings.Join(finalArgs, " "))
	cmd := exec.Command(bin, finalArgs...)

	if stdin != nil {
		cmd.Stdin = stdin
	}

	stderr, err := cmd.StderrPipe()
	require.NoError(td.test, err)

	stdout, err := cmd.StdoutPipe()
	require.NoError(td.test, err)

	require.NoError(td.test, cmd.Start())

	stderrBytes, err := ioutil.ReadAll(stderr)
	require.NoError(td.test, err)

	stdoutBytes, err := ioutil.ReadAll(stdout)
	require.NoError(td.test, err)

	o := &Output{
		Args:   args,
		Stdout: stdout,
		stdout: stdoutBytes,
		Stderr: stderr,
		stderr: stderrBytes,
		test:   td.test,
	}

	err = cmd.Wait()

	switch err := err.(type) {
	case *exec.ExitError:
		// TODO: its non-trivial to get the 'exit code' cross platform...
		o.Code = 1
	default:
		o.Error = err
	case nil:
		// okay
	}

	return o
}

func (td *TestDaemon) RunSuccess(args ...string) *Output {
	td.test.Helper()
	return td.Run(args...).AssertSuccess()
}

func (o *Output) AssertSuccess() *Output {
	o.test.Helper()
	assert.NoError(o.test, o.Error)
	oErr := o.ReadStderr()

	assert.Equal(o.test, o.Code, 0, oErr)
	assert.NotContains(o.test, oErr, "CRITICAL")
	assert.NotContains(o.test, oErr, "ERROR")
	assert.NotContains(o.test, oErr, "WARNING")
	return o

}

func (td *TestDaemon) RunFail(err string, args ...string) *Output {
	td.test.Helper()
	return td.Run(args...).AssertFail(err)
}

func (o *Output) AssertFail(err string) *Output {
	o.test.Helper()
	assert.NoError(o.test, o.Error)
	assert.Equal(o.test, 1, o.Code)
	assert.Empty(o.test, o.ReadStdout())
	assert.Contains(o.test, o.ReadStderr(), err)
	return o
}

func (td *TestDaemon) GetID() string {
	out := td.RunSuccess("id")
	var parsed map[string]interface{}
	require.NoError(td.test, json.Unmarshal([]byte(out.ReadStdout()), &parsed))

	return parsed["ID"].(string)
}

func (td *TestDaemon) GetAddress() string {
	out := td.RunSuccess("id")
	var parsed map[string]interface{}
	require.NoError(td.test, json.Unmarshal([]byte(out.ReadStdout()), &parsed))

	adders := parsed["Addresses"].([]interface{})
	return adders[0].(string)
}

func (td *TestDaemon) ConnectSuccess(remote *TestDaemon) *Output {
	// Connect the nodes
	out := td.RunSuccess("swarm", "connect", remote.GetAddress())
	peers1 := td.RunSuccess("swarm", "peers")
	peers2 := remote.RunSuccess("swarm", "peers")

	td.test.Log("[success] 1 -> 2")
	require.Contains(td.test, peers1.ReadStdout(), remote.GetID())

	td.test.Log("[success] 2 -> 1")
	require.Contains(td.test, peers2.ReadStdout(), td.GetID())

	return out
}

func (td *TestDaemon) ReadStdout() string {
	td.lk.Lock()
	defer td.lk.Unlock()
	out, err := ioutil.ReadAll(td.Stdout)
	if err != nil {
		panic(err)
	}
	return string(out)
}

func (td *TestDaemon) ReadStderr() string {
	td.lk.Lock()
	defer td.lk.Unlock()
	out, err := ioutil.ReadAll(td.Stderr)
	if err != nil {
		panic(err)
	}
	return string(out)
}

func (td *TestDaemon) Start() *TestDaemon {
	require.NoError(td.test, td.process.Start())
	require.NoError(td.test, td.WaitForAPI(), "Daemon failed to start")
	return td
}

func (td *TestDaemon) Shutdown() {
	if err := td.process.Process.Signal(syscall.SIGTERM); err != nil {
		td.test.Errorf("Daemon Stderr:\n%s", td.ReadStderr())
		td.test.Fatalf("Failed to kill daemon %s", err)
	}

	if td.repoDir == "" {
		panic("testdaemon had no repodir set")
	}

	_ = os.RemoveAll(td.repoDir)
}

func (td *TestDaemon) ShutdownSuccess() {
	err := td.process.Process.Signal(syscall.SIGTERM)
	assert.NoError(td.test, err)
	tdOut := td.ReadStderr()
	assert.NoError(td.test, err, tdOut)
	assert.NotContains(td.test, tdOut, "CRITICAL")
	assert.NotContains(td.test, tdOut, "ERROR")
	assert.NotContains(td.test, tdOut, "WARNING")
}

func (td *TestDaemon) Kill() {
	if err := td.process.Process.Kill(); err != nil {
		td.test.Errorf("Daemon Stderr:\n%s", td.ReadStderr())
		td.test.Fatalf("Failed to kill daemon %s", err)
	}
}

func (td *TestDaemon) WaitForAPI() error {
	for i := 0; i < 100; i++ {
		err := tryAPICheck(td)
		if err == nil {
			return nil
		}
		time.Sleep(time.Millisecond * 100)
	}
	return fmt.Errorf("filecoin node failed to come online in given time period (20 seconds)")
}

// CreateMinerAddr issues a new message to the network, mines the message
// and returns the address of the new miner
// equivalent to:
//     `go-filecoin miner create --from $TEST_ACCOUNT 100000 20`
func (td *TestDaemon) CreateMinerAddr() types.Address {
	// need money
	td.RunSuccess("mining", "once")

	addr := td.Config().Mining.RewardAddress
	miner := td.RunSuccess("miner", "create", "--from", addr.String(), "1000000", "1000")
	minerMessageCid, err := cid.Parse(strings.Trim(miner.ReadStdout(), "\n"))
	require.NoError(td.test, err)

	var wg sync.WaitGroup
	var minerAddr types.Address

	wg.Add(1)
	go func() {
		wait := td.RunSuccess("message", "wait",
			"--return",
			"--message=false",
			"--receipt=false",
			minerMessageCid.String(),
		)
		addr, err := types.NewAddressFromString(strings.Trim(wait.ReadStdout(), "\n"))
		require.NoError(td.test, err)
		require.NotEqual(td.test, addr, types.Address{})
		minerAddr = addr
		wg.Done()
	}()

	td.RunSuccess("mining", "once")

	wg.Wait()
	return minerAddr
}

// CreateWalletAddr adds a new address to the daemons wallet and
// returns it.
// equivalent to:
//     `go-filecoin wallet addrs new`
func (td *TestDaemon) CreateWalletAddr() string {
	td.test.Helper()
	outNew := td.RunSuccess("wallet", "addrs", "new")
	addr := strings.Trim(outNew.ReadStdout(), "\n")
	require.NotEmpty(td.test, addr)
	return addr
}

// Config is a helper to read out the config of the deamon
func (td *TestDaemon) Config() *config.Config {
	cfg, err := config.ReadFile(filepath.Join(td.repoDir, "config.toml"))
	require.NoError(td.test, err)
	return cfg
}

// MineAndPropagate mines a block and ensure the block has propogated to all `peers`
// by comparing the current head block of `td` with the head block of each peer in `peers`
func (td *TestDaemon) MineAndPropagate(wait time.Duration, peers ...*TestDaemon) {
	td.RunSuccess("mining", "once")
	// short circuit
	if peers == nil {
		return
	}
	// ensure all peers have same chain head as `td`
	td.MustHaveChainHeadBy(wait, peers)
}

// MustHaveChainHeadBy ensures all `peers` have the same chain head as `td`, by
// duration `wait`
func (td *TestDaemon) MustHaveChainHeadBy(wait time.Duration, peers []*TestDaemon) {
	// will signal all nodes have completed check
	done := make(chan struct{})
	var wg sync.WaitGroup

	expHead := td.GetChainHead()

	for _, p := range peers {
		wg.Add(1)
		go func(p *TestDaemon) {
			for {
				actHead := p.GetChainHead()
				if expHead.Cid().Equals(actHead.Cid()) {
					wg.Done()
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
		}(p)
	}

	go func() {
		wg.Wait()
		done <- struct{}{}
	}()

	select {
	case <-done:
		return
	case <-time.After(wait):
		td.test.Fatal("Timeout waiting for chains to sync")
	}
}

// GetChainHead returns the head block from `td`
func (td *TestDaemon) GetChainHead() types.Block {
	out := td.RunSuccess("chain", "ls", "--enc=json")
	bc := td.MustUnmarshalChain(out.ReadStdout())
	return bc[0]
}

// MustUnmarshalChain unmarshals the chain from `input` into a slice of blocks
func (td *TestDaemon) MustUnmarshalChain(input string) []types.Block {
	chain := strings.Trim(input, "\n")
	var bs []types.Block

	for _, line := range bytes.Split([]byte(chain), []byte{'\n'}) {
		var b types.Block
		if err := json.Unmarshal(line, &b); err != nil {
			td.test.Fatal(err)
		}
		bs = append(bs, b)
	}

	return bs
}

// MakeMoney mines a block and receives the block reward
func (td *TestDaemon) MakeMoney(rewards int) {
	for i := 0; i < rewards; i++ {
		td.MineAndPropagate(time.Second * 1)
	}
}

// MakeDeal will make a deal with the miner `miner`, using data `dealData`.
// MakeDeal will return the cid of `dealData`
func (td *TestDaemon) MakeDeal(dealData string, miner *TestDaemon) string {

	// The daemons need 2 monies each.
	td.MakeMoney(2)
	miner.MakeMoney(2)

	// How long to wait for miner blocks to propagate to other nodes
	propWait := time.Second * 3

	m := miner.CreateMinerAddr()

	askO := miner.RunSuccess(
		"miner", "add-ask",
		"--from", miner.Config().Mining.RewardAddress.String(),
		m.String(), "1200", "1",
	)
	miner.MineAndPropagate(propWait, td)
	miner.RunSuccess("message", "wait", "--return", strings.TrimSpace(askO.ReadStdout()))

	td.RunSuccess(
		"client", "add-bid",
		"--from", td.Config().Mining.RewardAddress.String(),
		"500", "1",
	)
	td.MineAndPropagate(propWait, miner)

	buf := strings.NewReader(dealData)
	o := td.RunWithStdin(buf, "client", "import").AssertSuccess()
	ddCid := strings.TrimSpace(o.ReadStdout())

	negidO := td.RunSuccess("client", "propose-deal", "--ask=0", "--bid=0", ddCid)
	time.Sleep(time.Millisecond * 20)

	miner.MineAndPropagate(propWait, td)

	negid := strings.Split(strings.Split(negidO.ReadStdout(), "\n")[1], " ")[1]
	// ensure we have made the deal
	td.RunSuccess("client", "query-deal", negid)
	// return the cid for the dealData (ddCid)
	return ddCid
}

func tryAPICheck(td *TestDaemon) error {
	url := fmt.Sprintf("http://127.0.0.1%s/api/id", td.cmdAddr)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}

	out := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		return fmt.Errorf("liveness check failed: %s", err)
	}

	_, ok := out["ID"]
	if !ok {
		return fmt.Errorf("liveness check failed: ID field not present in output")
	}

	return nil
}

func SwarmAddr(addr string) func(*TestDaemon) {
	return func(td *TestDaemon) {
		td.swarmAddr = addr
	}
}

func RepoDir(dir string) func(*TestDaemon) {
	return func(td *TestDaemon) {
		td.repoDir = dir
	}
}

func ShouldInit(i bool) func(*TestDaemon) {
	return func(td *TestDaemon) {
		td.init = i
	}
}

func NewDaemon(t *testing.T, options ...func(*TestDaemon)) *TestDaemon {
	// Ensure we have the actual binary
	filecoinBin, err := GetFilecoinBinary()
	if err != nil {
		t.Fatal(err)
	}

	//Ask the kernel for a port to avoid conflicts
	cmdPort, err := GetFreePort()
	if err != nil {
		t.Fatal(err)
	}
	swarmPort, err := GetFreePort()
	if err != nil {
		t.Fatal(err)
	}

	dir, err := ioutil.TempDir("", "go-fil-test")
	if err != nil {
		t.Fatal(err)
	}

	td := &TestDaemon{
		cmdAddr:   fmt.Sprintf(":%d", cmdPort),
		swarmAddr: fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", swarmPort),
		test:      t,
		repoDir:   dir,
		init:      true, // we want to init unless told otherwise
	}

	// configure TestDaemon options
	for _, option := range options {
		option(td)
	}

	repodirFlag := fmt.Sprintf("--repodir=%s", td.repoDir)
	if td.init {
		out, err := RunInit(repodirFlag)
		if err != nil {
			t.Log(string(out))
			t.Fatal(err)
		}
	}

	// define filecoin daemon process
	td.process = exec.Command(filecoinBin, "daemon",
		fmt.Sprintf("--repodir=%s", td.repoDir),
		fmt.Sprintf("--cmdapiaddr=%s", td.cmdAddr),
		fmt.Sprintf("--swarmlisten=%s", td.swarmAddr),
	)

	// setup process pipes
	td.Stdout, err = td.process.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	td.Stderr, err = td.process.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	td.Stdin, err = td.process.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	return td
}

// Credit: https://github.com/phayes/freeport
func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func RunInit(opts ...string) ([]byte, error) {
	return RunCommand("init", opts...)
}

func RunCommand(cmd string, opts ...string) ([]byte, error) {
	filecoinBin, err := GetFilecoinBinary()
	if err != nil {
		return nil, err
	}

	process := exec.Command(filecoinBin, append([]string{cmd}, opts...)...)
	return process.CombinedOutput()
}

func ConfigExists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "config.toml"))
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}

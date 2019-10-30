package testhelpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/build/project"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-cid"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multiaddr-net"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// DefaultDaemonCmdTimeout is the default timeout for executing commands.
	DefaultDaemonCmdTimeout = 1 * time.Minute
	repoName                = "repo"
	sectorsName             = "sectors"
)

// RunSuccessFirstLine executes the given command, asserts success and returns
// the first line of stdout.
func RunSuccessFirstLine(td *TestDaemon, args ...string) string {
	return RunSuccessLines(td, args...)[0]
}

// RunSuccessLines executes the given command, asserts success and returns
// an array of lines of the stdout.
func RunSuccessLines(td *TestDaemon, args ...string) []string {
	output := td.RunSuccess(args...)
	result := output.ReadStdoutTrimNewlines()
	return strings.Split(result, "\n")
}

// TestDaemon is used to manage a Filecoin daemon instance for testing purposes.
type TestDaemon struct {
	containerDir     string // Path to directory containing repo and sectors
	genesisFile      string
	keyFiles         []string
	withMiner        string
	autoSealInterval string
	isRelay          bool

	firstRun bool
	init     bool

	lk     sync.Mutex
	Stdin  io.Writer
	Stdout io.Reader
	Stderr io.Reader

	process        *exec.Cmd
	test           *testing.T
	cmdTimeout     time.Duration
	defaultAddress string
	daemonArgs     []string
}

// RepoDir returns the repo directory of the test daemon.
func (td *TestDaemon) RepoDir() string {
	return path.Join(td.containerDir, repoName)
}

// SectorDir returns the sector root directory of the test daemon.
func (td *TestDaemon) SectorDir() string {
	return path.Join(td.containerDir, sectorsName)
}

// CmdAddr returns the command address of the test daemon (if it is running).
func (td *TestDaemon) CmdAddr() (ma.Multiaddr, error) {
	str, err := ioutil.ReadFile(filepath.Join(td.RepoDir(), "api"))
	if err != nil {
		return nil, err
	}

	return ma.NewMultiaddr(strings.TrimSpace(string(str)))
}

// Config is a helper to read out the config of the daemon.
func (td *TestDaemon) Config() *config.Config {
	cfg, err := config.ReadFile(filepath.Join(td.RepoDir(), "config.json"))
	require.NoError(td.test, err)
	return cfg
}

// GetMinerAddress returns the miner address for this daemon.
func (td *TestDaemon) GetMinerAddress() address.Address {
	return td.Config().Mining.MinerAddress
}

// Run executes the given command against the test daemon.
func (td *TestDaemon) Run(args ...string) *CmdOutput {
	td.test.Helper()
	return td.RunWithStdin(nil, args...)
}

// RunWithStdin executes the given command against the test daemon, allowing to control
// stdin of the process.
func (td *TestDaemon) RunWithStdin(stdin io.Reader, args ...string) *CmdOutput {
	td.test.Helper()
	bin := MustGetFilecoinBinary()

	ctx, cancel := context.WithTimeout(context.Background(), td.cmdTimeout)
	defer cancel()

	addr, err := td.CmdAddr()
	require.NoError(td.test, err)

	// handle Run("cmd subcmd")
	if len(args) == 1 {
		args = strings.Split(args[0], " ")
	}

	finalArgs := append(args, "--repodir="+td.RepoDir(), "--cmdapiaddr="+addr.String())

	td.logRun(finalArgs...)
	cmd := exec.CommandContext(ctx, bin, finalArgs...)

	if stdin != nil {
		cmd.Stdin = stdin
	}

	stderr, err := cmd.StderrPipe()
	require.NoError(td.test, err)

	stdout, err := cmd.StdoutPipe()
	require.NoError(td.test, err)

	require.NoError(td.test, cmd.Start())

	o := ReadOutput(td.test, args, stdout, stderr)
	td.test.Logf("stdout\n%s", o.ReadStdout())
	td.test.Logf("stderr\n%s", o.ReadStderr())

	err = cmd.Wait()

	switch err := err.(type) {
	case *exec.ExitError:
		if ctx.Err() == context.DeadlineExceeded {
			o.SetInvocationError(errors.Wrapf(err, "context deadline exceeded for command: %q", strings.Join(finalArgs, " ")))
		} else {
			// "Successful" invocation, but a non-zero exit code.
			// It's non-trivial to get the real 'exit code' cross platform, so just use "1".
			o.SetStatus(1)
		}
	default:
		o.SetInvocationError(err)
	case nil:
		o.SetStatus(0)
	}

	return o
}

// RunSuccess is like Run, but asserts that the command exited successfully.
func (td *TestDaemon) RunSuccess(args ...string) *CmdOutput {
	td.test.Helper()
	return td.Run(args...).AssertSuccess()
}

// RunFail is like Run, but asserts that the command exited with an error
// matching the passed in error.
func (td *TestDaemon) RunFail(err string, args ...string) *CmdOutput {
	td.test.Helper()
	return td.Run(args...).AssertFail(err)
}

// GetID returns the id of the daemon.
func (td *TestDaemon) GetID() string {
	out := td.RunSuccess("id")
	var parsed map[string]interface{}
	require.NoError(td.test, json.Unmarshal([]byte(out.ReadStdout()), &parsed))

	return parsed["ID"].(string)
}

// GetAddresses returns all of the addresses of the daemon.
func (td *TestDaemon) GetAddresses() []string {
	out := td.RunSuccess("id")
	var parsed map[string]interface{}
	require.NoError(td.test, json.Unmarshal([]byte(out.ReadStdout()), &parsed))
	adders := parsed["Addresses"].([]interface{})

	res := make([]string, len(adders))
	for i, addr := range adders {
		res[i] = addr.(string)
	}
	return res
}

// ConnectSuccess connects the daemon to another daemon, asserting that
// the operation was successful.
func (td *TestDaemon) ConnectSuccess(remote *TestDaemon) *CmdOutput {
	remoteAddrs := remote.GetAddresses()
	delay := 100 * time.Millisecond

	// Connect the nodes
	// This usually works on the first try, but leaving this here, to ensure we
	// connect and don't fail the test.
	var out *CmdOutput
	var status int
	var err error
Outer:
	for i := 0; i < 5; i++ {
		for j, remoteAddr := range remoteAddrs {
			out = td.Run("swarm", "connect", remoteAddr)
			status, err = out.Status()
			if err == nil && status == 0 {
				if i > 0 || j > 0 {
					fmt.Printf("WARNING: swarm connect took %d tries", (i+1)*(j+1))
				}
				break Outer
			}
			time.Sleep(delay)
		}
	}
	assert.NoError(td.test, err, "failed to execute swarm connect")

	localID := td.GetID()
	remoteID := remote.GetID()

	connected1 := false
	for i := 0; i < 10; i++ {
		peers1 := td.RunSuccess("swarm", "peers")

		p1 := peers1.ReadStdout()
		connected1 = strings.Contains(p1, remoteID)
		if connected1 {
			break
		}
		td.test.Log(p1)
		time.Sleep(delay)
	}

	connected2 := false
	for i := 0; i < 10; i++ {
		peers2 := remote.RunSuccess("swarm", "peers")

		p2 := peers2.ReadStdout()
		connected2 = strings.Contains(p2, localID)
		if connected2 {
			break
		}
		td.test.Log(p2)
		time.Sleep(delay)
	}

	require.True(td.test, connected1, "failed to connect p1 -> p2")
	require.True(td.test, connected2, "failed to connect p2 -> p1")

	return out
}

// ReadStdout returns a string representation of the stdout of the daemon.
func (td *TestDaemon) ReadStdout() string {
	td.lk.Lock()
	defer td.lk.Unlock()
	out, err := ioutil.ReadAll(td.Stdout)
	if err != nil {
		panic(err)
	}
	return string(out)
}

// ReadStderr returns a string representation of the stderr of the daemon.
func (td *TestDaemon) ReadStderr() string {
	td.lk.Lock()
	defer td.lk.Unlock()
	out, err := ioutil.ReadAll(td.Stderr)
	if err != nil {
		panic(err)
	}
	return string(out)
}

// Start starts up the daemon.
func (td *TestDaemon) Start() *TestDaemon {
	td.createNewProcess()

	require.NoError(td.test, td.process.Start())

	err := td.WaitForAPI()
	if err != nil {
		stdErr, _ := ioutil.ReadAll(td.Stderr)
		stdOut, _ := ioutil.ReadAll(td.Stdout)
		td.test.Errorf("%s\n%s", stdErr, stdOut)
	}

	require.NoError(td.test, err, "Daemon failed to start")

	// on first startup import key pairs, if defined
	if td.firstRun {
		for _, file := range td.keyFiles {
			td.RunSuccess("wallet", "import", file)
		}
	}

	return td
}

// Stop stops the daemon
func (td *TestDaemon) Stop() *TestDaemon {
	if err := td.process.Process.Signal(syscall.SIGINT); err != nil {
		panic(err)
	}
	if _, err := td.process.Process.Wait(); err != nil {
		panic(err)
	}
	return td
}

// Restart restarts the daemon
func (td *TestDaemon) Restart() *TestDaemon {
	td.Stop()
	td.assertNoLogErrors()
	return td.Start()
}

// Shutdown stops the daemon and deletes the repository.
func (td *TestDaemon) Shutdown() {
	if err := td.process.Process.Signal(syscall.SIGTERM); err != nil {
		td.test.Errorf("Daemon Stderr:\n%s", td.ReadStderr())
		td.test.Fatalf("Failed to kill daemon %s", err)
	}

	td.cleanupFilesystem()
}

// ShutdownSuccess stops the daemon, asserting that it exited successfully.
func (td *TestDaemon) ShutdownSuccess() {
	err := td.process.Process.Signal(syscall.SIGTERM)
	assert.NoError(td.test, err)

	td.assertNoLogErrors()
	td.cleanupFilesystem()
}

func (td *TestDaemon) assertNoLogErrors() {
	tdErr := td.ReadStderr()

	// We filter out errors expected from the cleanup associated with SIGTERM
	ExpectedErrors := []string{
		regexp.QuoteMeta("BlockSub.Next(): subscription cancelled by calling sub.Cancel()"),
		regexp.QuoteMeta("MessageSub.Next(): subscription cancelled by calling sub.Cancel()"),
		regexp.QuoteMeta("BlockSub.Next(): context canceled"),
		regexp.QuoteMeta("MessageSub.Next(): context canceled"),
	}

	filteredStdErr := tdErr
	for _, errorMsg := range ExpectedErrors {
		filteredStdErr = regexp.MustCompile("(?m)^.*"+errorMsg+".*$").ReplaceAllString(filteredStdErr, "")
	}

	assert.NotContains(td.test, filteredStdErr, "CRITICAL")
	assert.NotContains(td.test, filteredStdErr, "ERROR")
}

// ShutdownEasy stops the daemon using `SIGINT`.
func (td *TestDaemon) ShutdownEasy() {
	err := td.process.Process.Signal(syscall.SIGINT)
	assert.NoError(td.test, err)
	tdOut := td.ReadStderr()
	assert.NoError(td.test, err, tdOut)

	td.cleanupFilesystem()
}

// WaitForAPI polls if the API on the daemon is available, and blocks until
// it is.
func (td *TestDaemon) WaitForAPI() error {
	var err error
	for i := 0; i < 100; i++ {
		err = tryAPICheck(td)
		if err == nil {
			return nil
		}
		time.Sleep(time.Millisecond * 100)
	}
	return fmt.Errorf("filecoin node failed to come online in given time period (10 seconds); last err = %s", err)
}

// CreateStorageMinerAddr issues a new message to the network, mines the message
// and returns the address of the new miner
// equivalent to:
//     `go-filecoin miner create --from $TEST_ACCOUNT 20`
func (td *TestDaemon) CreateStorageMinerAddr(peer *TestDaemon, fromAddr string) address.Address {
	var wg sync.WaitGroup
	var minerAddr address.Address
	wg.Add(1)
	go func() {
		miner := td.RunSuccess("miner", "create", "--from", fromAddr, "--gas-price", "1", "--gas-limit", "100", "20")
		addr, err := address.NewFromString(strings.Trim(miner.ReadStdout(), "\n"))
		require.NoError(td.test, err)
		require.NotEqual(td.test, addr, address.Undef)
		minerAddr = addr
		wg.Done()
	}()
	wg.Wait()

	return minerAddr
}

// MinerSetPrice creates an ask for a CURRENTLY MINING test daemon and waits for it to appears on chain. It returns the
// cid of the AddAsk message so other daemons can `message wait` for it.
func (td *TestDaemon) MinerSetPrice(minerAddr string, fromAddr string, price string, expiry string) cid.Cid {
	setPriceReturn := td.RunSuccess("miner", "set-price",
		"--from", fromAddr,
		"--miner", minerAddr,
		"--gas-price", "1",
		"--gas-limit", "300",
		"--enc", "json",
		price, expiry).ReadStdout()

	resultStruct := struct {
		MinerSetPriceResponse struct {
			AddAskCid cid.Cid
		}
	}{}

	if err := json.Unmarshal([]byte(setPriceReturn), &resultStruct); err != nil {
		require.NoError(td.test, err)
	}
	return resultStruct.MinerSetPriceResponse.AddAskCid
}

// UpdatePeerID updates a currently mining miner's peer ID
func (td *TestDaemon) UpdatePeerID() {
	var idOutput map[string]interface{}
	peerIDJSON := td.RunSuccess("id").ReadStdout()
	err := json.Unmarshal([]byte(peerIDJSON), &idOutput)
	require.NoError(td.test, err)
	updateCidStr := td.RunSuccess("miner", "update-peerid", "--gas-price=1", "--gas-limit=300", td.GetMinerAddress().String(), idOutput["ID"].(string)).ReadStdoutTrimNewlines()
	updateCid, err := cid.Parse(updateCidStr)
	require.NoError(td.test, err)
	assert.NotNil(td.test, updateCid)

	td.WaitForMessageRequireSuccess(updateCid)
}

// WaitForMessageRequireSuccess accepts a message cid and blocks until a message with matching cid is included in a
// block. The receipt is then inspected to ensure that the corresponding message receipt had a 0 exit code.
func (td *TestDaemon) WaitForMessageRequireSuccess(msgCid cid.Cid) *types.MessageReceipt {
	args := []string{"message", "wait", msgCid.String(), "--receipt=true", "--message=false"}
	trim := strings.Trim(td.RunSuccess(args...).ReadStdout(), "\n")
	rcpt := &types.MessageReceipt{}
	require.NoError(td.test, json.Unmarshal([]byte(trim), rcpt))
	require.Equal(td.test, 0, int(rcpt.ExitCode))
	return rcpt
}

// CreateAddress adds a new address to the daemons wallet and
// returns it.
// equivalent to:
//     `go-filecoin address new`
func (td *TestDaemon) CreateAddress() string {
	td.test.Helper()
	outNew := td.RunSuccess("address", "new")
	addr := strings.Trim(outNew.ReadStdout(), "\n")
	require.NotEmpty(td.test, addr)
	return addr
}

// MineAndPropagate mines a block and ensure the block has propagated to all `peers`
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

	expHeadBlks := td.GetChainHead()
	var expHeadCids []cid.Cid
	for _, blk := range expHeadBlks {
		expHeadCids = append(expHeadCids, blk.Cid())
	}
	expHeadKey := block.NewTipSetKey(expHeadCids...)

	for _, p := range peers {
		wg.Add(1)
		go func(p *TestDaemon) {
			for {
				actHeadBlks := p.GetChainHead()
				var actHeadCids []cid.Cid
				for _, blk := range actHeadBlks {
					actHeadCids = append(actHeadCids, blk.Cid())
				}
				actHeadKey := block.NewTipSetKey(actHeadCids...)
				if expHeadKey.Equals(actHeadKey) {
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

// GetChainHead returns the blocks in the head tipset from `td`
func (td *TestDaemon) GetChainHead() []block.Block {
	out := td.RunSuccess("chain", "ls", "--enc=json")
	bc := td.MustUnmarshalChain(out.ReadStdout())
	return bc[0]
}

// MustUnmarshalChain unmarshals the chain from `input` into a slice of blocks
func (td *TestDaemon) MustUnmarshalChain(input string) [][]block.Block {
	chain := strings.Trim(input, "\n")
	var bs [][]block.Block

	for _, line := range bytes.Split([]byte(chain), []byte{'\n'}) {
		var b []block.Block
		if err := json.Unmarshal(line, &b); err != nil {
			td.test.Fatal(err)
		}
		bs = append(bs, b)
	}

	return bs
}

// MakeMoney mines a block and ensures that the block has been propagated to all peers.
func (td *TestDaemon) MakeMoney(rewards int, peers ...*TestDaemon) {
	for i := 0; i < rewards; i++ {
		td.MineAndPropagate(time.Second*1, peers...)
	}
}

// GetDefaultAddress returns the default sender address for this daemon.
func (td *TestDaemon) GetDefaultAddress() string {
	addrs := td.RunSuccess("address", "default")
	return addrs.ReadStdout()
}

func tryAPICheck(td *TestDaemon) error {
	maddr, err := td.CmdAddr()
	if err != nil {
		return err
	}

	_, host, err := manet.DialArgs(maddr)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s/api/id", host)
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

// ContainerDir sets the `containerDir` path for the daemon.
func ContainerDir(dir string) func(*TestDaemon) {
	return func(td *TestDaemon) {
		td.containerDir = dir
	}
}

// ShouldInit allows setting the `init` config option on the daemon. If
// set, `go-filecoin init` is run before starting up the daemon.
func ShouldInit(i bool) func(*TestDaemon) {
	return func(td *TestDaemon) {
		td.init = i
	}
}

// CmdTimeout allows setting the `cmdTimeout` config option on the daemon.
func CmdTimeout(t time.Duration) func(*TestDaemon) {
	return func(td *TestDaemon) {
		td.cmdTimeout = t
	}
}

// KeyFile specifies a key file for this daemon to add to their wallet during init
func KeyFile(kf string) func(*TestDaemon) {
	return func(td *TestDaemon) {
		td.keyFiles = append(td.keyFiles, kf)
	}
}

// DefaultAddress specifies a key file for this daemon to add to their wallet during init
func DefaultAddress(defaultAddr string) func(*TestDaemon) {
	return func(td *TestDaemon) {
		td.defaultAddress = defaultAddr
	}
}

// AutoSealInterval specifies an interval for automatically sealing
func AutoSealInterval(autoSealInterval string) func(*TestDaemon) {
	return func(td *TestDaemon) {
		td.autoSealInterval = autoSealInterval
	}
}

// GenesisFile allows setting the `genesisFile` config option on the daemon.
func GenesisFile(a string) func(*TestDaemon) {
	return func(td *TestDaemon) {
		td.genesisFile = a
	}
}

// WithMiner allows setting the --with-miner flag on init.
func WithMiner(m string) func(*TestDaemon) {
	return func(td *TestDaemon) {
		td.withMiner = m
	}
}

// IsRelay starts the daemon with the --is-relay option.
func IsRelay(td *TestDaemon) {
	td.isRelay = true
}

// NewDaemon creates a new `TestDaemon`, using the passed in configuration options.
func NewDaemon(t *testing.T, options ...func(*TestDaemon)) *TestDaemon {
	t.Helper()
	// Ensure we have the actual binary
	filecoinBin := MustGetFilecoinBinary()

	td := &TestDaemon{
		test:        t,
		init:        true, // we want to init unless told otherwise
		firstRun:    true,
		cmdTimeout:  DefaultDaemonCmdTimeout,
		genesisFile: GenesisFilePath(), // default file includes all test addresses,
	}

	// configure TestDaemon options
	for _, option := range options {
		option(td)
	}

	// Allocate directory for repo and sectors. If set already it is assumed to exist.
	if td.containerDir == "" {
		newDir, err := ioutil.TempDir("", "daemon-test")
		if err != nil {
			t.Fatal(err)
		}
		td.containerDir = newDir
	}

	repoDirFlag := fmt.Sprintf("--repodir=%s", td.RepoDir())
	sectorDirFlag := fmt.Sprintf("--sectordir=%s", td.SectorDir())

	// build command options
	initopts := []string{repoDirFlag, sectorDirFlag}

	if td.genesisFile != "" {
		initopts = append(initopts, fmt.Sprintf("--genesisfile=%s", td.genesisFile))
	}

	if td.withMiner != "" {
		initopts = append(initopts, fmt.Sprintf("--with-miner=%s", td.withMiner))
	}

	if td.defaultAddress != "" {
		initopts = append(initopts, fmt.Sprintf("--default-address=%s", td.defaultAddress))
	}

	if td.autoSealInterval != "" {
		initopts = append(initopts, fmt.Sprintf("--auto-seal-interval-seconds=%s", td.autoSealInterval))
	}

	if td.init {
		t.Logf("run: go-filecoin init %s", initopts)
		out, err := RunInit(td, initopts...)
		if err != nil {
			t.Log(string(out))
			t.Fatal(err)
		}
	}

	// Defer allocation of a command API port until listening. The node will write the
	// listening address to the "api" file in the repo, from where we can read it when issuing commands.
	cmdAddr := "/ip4/127.0.0.1/tcp/0"
	cmdAPIAddrFlag := fmt.Sprintf("--cmdapiaddr=%s", cmdAddr)

	swarmAddr := "/ip4/127.0.0.1/tcp/0"
	swarmListenFlag := fmt.Sprintf("--swarmlisten=%s", swarmAddr)

	blockTimeFlag := fmt.Sprintf("--block-time=%s", BlockTimeTest)

	td.daemonArgs = []string{filecoinBin, "daemon", repoDirFlag, cmdAPIAddrFlag, swarmListenFlag, blockTimeFlag}

	if td.isRelay {
		td.daemonArgs = append(td.daemonArgs, "--is-relay")
	}

	return td
}

// RunInit is the equivalent of executing `go-filecoin init`.
func RunInit(td *TestDaemon, opts ...string) ([]byte, error) {
	filecoinBin := MustGetFilecoinBinary()

	finalArgs := append([]string{"init"}, opts...)
	td.logRun(finalArgs...)

	process := exec.Command(filecoinBin, finalArgs...)
	return process.CombinedOutput()
}

// GenesisFilePath returns the path to the test genesis
func GenesisFilePath() string {
	return project.Root("/fixtures/test/genesis.car")
}

func (td *TestDaemon) createNewProcess() {
	td.logRun(td.daemonArgs...)

	td.process = exec.Command(td.daemonArgs[0], td.daemonArgs[1:]...)
	// disable REUSEPORT, it creates problems in tests
	td.process.Env = append(os.Environ(), "IPFS_REUSEPORT=false")

	// setup process pipes
	var err error
	td.Stdout, err = td.process.StdoutPipe()
	if err != nil {
		td.test.Fatal(err)
	}
	// uncomment this and comment out the following 4 lines to output daemon stderr to os stderr
	//td.process.Stderr = os.Stderr
	td.Stderr, err = td.process.StderrPipe()
	if err != nil {
		td.test.Fatal(err)
	}
	td.Stdin, err = td.process.StdinPipe()
	if err != nil {
		td.test.Fatal(err)
	}
}

func (td *TestDaemon) cleanupFilesystem() {
	if td.containerDir != "" {
		err := os.RemoveAll(td.containerDir)
		if err != nil {
			td.test.Logf("error removing dir %s: %s", td.containerDir, err)
		}
	} else {
		td.test.Logf("testdaemon has nil container dir")
	}
}

// Logs a message prefixed by a daemon identifier
func (td *TestDaemon) logRun(args ...string) {
	name := path.Base(td.containerDir)
	td.test.Logf("(%s) run: %q\n", name, strings.Join(args, " "))
}

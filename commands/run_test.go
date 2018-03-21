package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// pipe manages a unix pipe. Used to simulate std{in|err|out}
type pipe struct {
	lk     sync.Mutex
	reader *os.File
	writer *os.File
	buffer *bufio.Reader
}

func (p *pipe) Reader() *os.File {
	p.lk.Lock()
	defer p.lk.Unlock()

	return p.reader
}
func (p *pipe) Writer() *os.File {
	p.lk.Lock()
	defer p.lk.Unlock()

	return p.writer
}

// IsReader indicates if we are have "reader" pipe, meaning we expect
// the writer to get written by another party and us just reading, this
// is the case for std{out|err}. For stdout the opposite is the case, e.g.
// we expect for the other party to read from the Reader, and us to write to the
// writer.
func (p *pipe) IsReader() bool {
	return p.buffer != nil
}

func (p *pipe) Close() {
	p.lk.Lock()
	defer p.lk.Unlock()

	if p.IsReader() {
		p.writer.Close()
	} else {
		p.reader.Close()
	}
}

func (p *pipe) ReadAll() ([]byte, error) {
	p.lk.Lock()
	defer p.lk.Unlock()

	if p.buffer == nil {
		return nil, fmt.Errorf("missing buffer to read from")
	}

	return ioutil.ReadAll(p.buffer)
}

func newPipe() (*pipe, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	return &pipe{
		reader: r,
		writer: w,
	}, nil
}

func newReadPipe() (*pipe, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	return &pipe{
		reader: r,
		writer: w,
		buffer: bufio.NewReader(r),
	}, nil
}

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
	Stdin  *pipe
	Stdout *pipe
	Stderr *pipe
	// CloseChan signals that the command stopped.
	CloseChan chan struct{}
}

func (o *Output) Interrupt() {
	// hack to stop the daemon inprocess
	sigCh <- os.Interrupt.(syscall.Signal)
}

func (o *Output) Close(code int, err error) {
	o.lk.Lock()
	defer o.lk.Unlock()

	o.Code = code
	o.Error = err

	o.Stdin.Close()
	o.Stdout.Close()
	o.Stderr.Close()
	o.CloseChan <- struct{}{}
}

func (o *Output) ReadStderr() string {
	o.lk.Lock()
	defer o.lk.Unlock()

	out, err := o.Stderr.ReadAll()
	if err != nil {
		panic(err)
	}
	return string(out)
}

func (o *Output) ReadStdout() string {
	o.lk.Lock()
	defer o.lk.Unlock()

	out, err := o.Stdout.ReadAll()
	if err != nil {
		panic(err)
	}
	return string(out)
}

// run runs a one off command.
func run(input string) *Output {
	o := runNoWait(input)
	<-o.CloseChan
	return o
}

// runNoWait spawns a command and don't wait for it to finish, like running the daemon.
func runNoWait(input string) *Output {
	args := strings.Split(input, " ")

	stdin, err := newPipe()
	if err != nil {
		panic(err)
	}
	stdout, err := newReadPipe()
	if err != nil {
		panic(err)
	}
	stderr, err := newReadPipe()
	if err != nil {
		panic(err)
	}

	o := &Output{
		Input:     input,
		Args:      args,
		Stdin:     stdin,
		Stdout:    stdout,
		Stderr:    stderr,
		CloseChan: make(chan struct{}, 1),
	}

	go func(o *Output) {
		exitCode, err := Run(args, stdin.Reader(), stdout.Writer(), stderr.Writer())
		o.Close(exitCode, err)
	}(o)

	return o
}

type TestDaemon struct {
	cmdAddr   string
	swarmAddr string

	//The filecoin daemon process
	process *exec.Cmd

	lk     sync.Mutex
	Stdin  io.Writer
	Stdout io.Reader
	Stderr io.Reader

	test *testing.T
}

func (td *TestDaemon) Run(input string, args ...string) *Output {
	cmd := fmt.Sprintf("go-filecoin %s --cmdapiaddr=%s", input, td.cmdAddr)
	if len(args) > 0 {
		cmd = fmt.Sprintf("%s %s", cmd, strings.Join(args, " "))
	}
	return run(cmd)
}

func (td *TestDaemon) RunSuccess(cmd string, args ...string) *Output {
	o := td.Run(cmd, args...)
	assert.NoError(td.test, o.Error)
	assert.Equal(td.test, o.Code, 0)
	oErr := o.ReadStderr()
	assert.NotContains(td.test, oErr, "CRITICAL")
	assert.NotContains(td.test, oErr, "ERROR")
	assert.NotContains(td.test, oErr, "WARNING")
	return o
}

func (td *TestDaemon) RunFail(err, cmd string, args ...string) *Output {
	td.test.Helper()
	o := td.Run(cmd, args...)
	assert.NoError(td.test, o.Error)
	assert.Equal(td.test, o.Code, 1)
	assert.Empty(td.test, o.ReadStdout())
	assert.Contains(td.test, o.ReadStderr(), err)
	return o
}

func (td *TestDaemon) GetID() string {
	out := td.RunSuccess("id")
	var parsed map[string]interface{}
	assert.NoError(td.test, json.Unmarshal([]byte(out.ReadStdout()), &parsed))

	return parsed["ID"].(string)
}

func (td *TestDaemon) GetAddress() string {
	out := td.RunSuccess("id")
	var parsed map[string]interface{}
	assert.NoError(td.test, json.Unmarshal([]byte(out.ReadStdout()), &parsed))

	adders := parsed["Addresses"].([]interface{})
	return adders[0].(string)
}

func (td *TestDaemon) ConnectSuccess(remote *TestDaemon) *Output {
	//Connect the nodes
	out := td.RunSuccess("swarm connect", remote.GetAddress())
	peers1 := td.RunSuccess("swarm peers")
	peers2 := remote.RunSuccess("swarm peers")

	td.test.Log("[success] 1 -> 2")
	assert.Contains(td.test, peers1.ReadStdout(), remote.GetID())

	td.test.Log("[success] 2 -> 1")
	assert.Contains(td.test, peers2.ReadStdout(), td.GetID())
	return out
}

func (td *TestDaemon) ReadAllStdout() string {
	td.lk.Lock()
	defer td.lk.Unlock()
	out, err := ioutil.ReadAll(td.Stdout)
	if err != nil {
		panic(err)
	}
	return string(out)
}

func (td *TestDaemon) ReadAllStderr() string {
	td.lk.Lock()
	defer td.lk.Unlock()
	out, err := ioutil.ReadAll(td.Stderr)
	if err != nil {
		panic(err)
	}
	return string(out)
}

func (td *TestDaemon) Start() *TestDaemon {
	if err := td.process.Start(); err != nil {
		td.test.Fatalf("Failed to start filecoin process: %v", err)
	}
	if err := td.WaitForAPI(); err != nil {
		td.test.Error(err)
		td.test.Fatalf("Daemon failed to start")
	}
	return td
}

func (td *TestDaemon) Shutdown() {
	if err := td.process.Process.Signal(syscall.SIGTERM); err != nil {
		td.test.Errorf("Daemon Stderr:\n%s", td.ReadAllStderr())
		td.test.Fatalf("Failed to kill daemon %s", err)
	}
}

func (td *TestDaemon) ShutdownSuccess() {
	err := td.process.Process.Signal(syscall.SIGTERM)
	assert.NoError(td.test, err)
	tdOut := td.ReadAllStderr()
	assert.NotContains(td.test, tdOut, "CRITICAL")
	assert.NotContains(td.test, tdOut, "ERROR")
	assert.NotContains(td.test, tdOut, "WARNING")

}

func (td *TestDaemon) Kill() {
	if err := td.process.Process.Kill(); err != nil {
		td.test.Errorf("Daemon Stderr:\n%s", td.ReadAllStderr())
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

func NewDaemon(t *testing.T, options ...func(*TestDaemon)) *TestDaemon {
	//Ensure we have the actual binary
	filecoinBin := fmt.Sprintf("%s/src/github.com/filecoin-project/go-filecoin/go-filecoin", os.Getenv("GOPATH"))
	if _, err := os.Stat(filecoinBin); os.IsNotExist(err) {
		t.Fatal("You are missing the filecoin binary...try building")
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
	td := &TestDaemon{
		cmdAddr:   fmt.Sprintf(":%d", cmdPort),
		swarmAddr: fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", swarmPort),
		test:      t,
	}

	// configure TestDaemon options
	for _, option := range options {
		option(td)
	}

	// define filecoin daemon process
	td.process = exec.Command(filecoinBin, "daemon",
		fmt.Sprintf("--cmdapiaddr=%s", td.cmdAddr),
		fmt.Sprintf("--swarmlisten=%s", td.swarmAddr),
	)

	//setup process pipes
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

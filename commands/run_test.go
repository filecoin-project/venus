package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
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

// output manages running, inprocess, a filecoin command.
type output struct {
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

func (o *output) Interrupt() {
	// hack to stop the daemon inprocess
	sigCh <- os.Interrupt.(syscall.Signal)
}

func (o *output) Close(code int, err error) {
	o.lk.Lock()
	defer o.lk.Unlock()

	o.Code = code
	o.Error = err

	o.Stdin.Close()
	o.Stdout.Close()
	o.Stderr.Close()
	o.CloseChan <- struct{}{}
}

func (o *output) ReadStderr() string {
	o.lk.Lock()
	defer o.lk.Unlock()

	out, err := o.Stderr.ReadAll()
	if err != nil {
		panic(err)
	}
	return string(out)
}

func (o *output) ReadStdout() string {
	o.lk.Lock()
	defer o.lk.Unlock()

	out, err := o.Stdout.ReadAll()
	if err != nil {
		panic(err)
	}
	return string(out)
}

// run a one off command and make sure it succeeds
func runSuccess(t *testing.T, args string) *output {
	t.Helper()
	out := run(args)
	assert.NoError(t, out.Error)
	assert.Equal(t, out.Code, 0)
	assert.Empty(t, out.ReadStderr())
	return out
}

// run a one off command
func run(input string) *output {
	o := runNoWait(input)
	<-o.CloseChan
	return o
}

// spawn a command and don't wait for it to finish, like running the daemon.
func runNoWait(input string) *output {
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

	o := &output{
		Input:     input,
		Args:      args,
		Stdin:     stdin,
		Stdout:    stdout,
		Stderr:    stderr,
		CloseChan: make(chan struct{}, 1),
	}

	go func(o *output) {
		exitCode, err := Run(args, stdin.Reader(), stdout.Writer(), stderr.Writer())
		o.Close(exitCode, err)
	}(o)

	return o
}

// runWithDaemon runs a command, while a daemon is running.
func runWithDaemon(input string, cb func(*output)) {
	d := withDaemon(func() {
		o := run(input)
		cb(o)
	})

	if d.Error != nil {
		panic(d.Error)
	}

	if err := d.ReadStderr(); err != "" {
		panic(fmt.Sprintf("stderr: %s", err))
	}

	if d.Code != 0 {
		panic(fmt.Sprintf("exit code not 0: %d", d.Code))
	}
}

// withDaemon executes the provided cb during the time a daemon is running.
func withDaemon(cb func()) (daemon *output) {
	return withDaemonArgs("", cb)
}

// withDaemonsArgs executes the provided cb during the time multiple daemons are running and allow to pass
// additional arguments to each daemon.
func withDaemonsArgs(count int, args []string, cb func()) []*output {
	// lock the out array
	var lk sync.Mutex
	out := make([]*output, count)
	var outWg sync.WaitGroup
	outWg.Add(count)

	// synchronize such all daemons get spawened
	var spawnWg sync.WaitGroup
	spawnWg.Add(count)

	// keep daemons running until we are done
	var runWg sync.WaitGroup
	runWg.Add(1)

	for i := 0; i < count; i++ {
		go func(i int) {
			d := withDaemonArgs(args[i], func() {
				spawnWg.Done()
				runWg.Wait()
			})

			lk.Lock()
			defer lk.Unlock()
			out[i] = d
			outWg.Done()
		}(i)
	}

	spawnWg.Wait()
	cb()
	runWg.Done()

	outWg.Wait()
	return out
}

// withDaemonArgs runs cb while a single daemon is running.
func withDaemonArgs(args string, cb func()) (daemon *output) {
	cmd := "go-filecoin daemon"
	if len(args) > 0 {
		cmd = fmt.Sprintf("%s %s", cmd, args)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		daemon = runNoWait(cmd)
		wg.Done()
	}()

	wg.Wait()
	wg.Add(1)

	go func() {
		scanner := bufio.NewScanner(daemon.Stdout.Reader())
	scannerLoop:
		for {
			select {
			case <-daemon.CloseChan:
				// early close
				panic(daemon.Error)
			default:
				if scanner.Scan() && strings.Contains(scanner.Text(), "listening on") {
					// wait a little bit to avoid flakyness...
					time.Sleep(100 * time.Millisecond)
					cb()
					wg.Done()
					break scannerLoop
				}
			}
		}
		if err := scanner.Err(); err != nil {
			panic(err)
		}
	}()

	wg.Wait()
	if daemon != nil {
		daemon.Interrupt()
		<-daemon.CloseChan
	}

	return daemon
}

// getId fetches the id of the daemon running on the passed in address.
func getID(t *testing.T, api string) string {
	out := runSuccess(t, fmt.Sprintf("go-filecoin id --cmdapiaddr=%s", api))
	var parsed map[string]interface{}
	assert.NoError(t, json.Unmarshal([]byte(out.ReadStdout()), &parsed))

	return parsed["ID"].(string)
}

package fast

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	logging "github.com/ipfs/go-log"
	iptb "github.com/ipfs/iptb/testbed"
	"github.com/ipfs/iptb/testbed/interfaces"
	"github.com/libp2p/go-libp2p-peer"
	"golang.org/x/sync/errgroup"

	fcconfig "github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastutil"
	dockerplugin "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/docker"
	localplugin "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/local"
)

var (
	// ErrDoubleInitOpts is returned by InitDaemon when both init options are provided by EnvironmentOpts
	// in NewProcess as well as passed to InitDaemon directly.
	ErrDoubleInitOpts = errors.New("cannot provide both init options through environment and arguments")

	// ErrDoubleDaemonOpts is returned by StartDaemon when both init options are provided by EnvironmentOpts
	// in NewProcess as well as passed to StartDaemon directly.
	ErrDoubleDaemonOpts = errors.New("cannot provide both daemon options through environment and arguments")
)

// must register all filecoin iptb plugins first.
func init() {
	_, err := iptb.RegisterPlugin(iptb.IptbPlugin{
		From:       "<builtin>",
		NewNode:    localplugin.NewNode,
		PluginName: localplugin.PluginName,
		BuiltIn:    true,
	}, false)

	if err != nil {
		panic(err)
	}

	_, err = iptb.RegisterPlugin(iptb.IptbPlugin{
		From:       "<builtin>",
		NewNode:    dockerplugin.NewNode,
		PluginName: dockerplugin.PluginName,
		BuiltIn:    true,
	}, false)

	if err != nil {
		panic(err)
	}
}

// IPTBCoreExt is an extended interface of the iptb.Core. It defines additional requirement.
type IPTBCoreExt interface {
	testbedi.Core
	testbedi.Config

	// StderrReader is require to gather daemon logs during action execution
	StderrReader() (io.ReadCloser, error)
}

// Filecoin represents a wrapper around the iptb Core interface.
type Filecoin struct {
	PeerID peer.ID

	initOpts   []ProcessInitOption
	daemonOpts []ProcessDaemonOption

	Log logging.EventLogger

	core IPTBCoreExt
	ctx  context.Context

	lastCmdOutput testbedi.Output

	stderr io.ReadCloser

	lpCtx    context.Context
	lpCancel context.CancelFunc
	lpErr    error
	lp       *fastutil.LinePuller
	ir       fastutil.IntervalRecorder
}

// NewFilecoinProcess returns a pointer to a Filecoin process that wraps the IPTB core interface `c`.
func NewFilecoinProcess(ctx context.Context, c IPTBCoreExt, eo EnvironmentOpts) *Filecoin {
	return &Filecoin{
		core:       c,
		Log:        logging.Logger(c.String()),
		ctx:        ctx,
		initOpts:   eo.InitOpts,
		daemonOpts: eo.DaemonOpts,
	}
}

// InitDaemon initializes the filecoin daemon process.
func (f *Filecoin) InitDaemon(ctx context.Context, args ...string) (testbedi.Output, error) {
	if len(args) != 0 && len(f.initOpts) != 0 {
		return nil, ErrDoubleInitOpts
	}

	if len(args) == 0 {
		for _, opt := range f.initOpts {
			args = append(args, opt()...)
		}
	}

	return f.core.Init(ctx, args...)
}

// StartDaemon starts the filecoin daemon process.
func (f *Filecoin) StartDaemon(ctx context.Context, wait bool, args ...string) (testbedi.Output, error) {
	if len(args) != 0 && len(f.daemonOpts) != 0 {
		return nil, ErrDoubleDaemonOpts
	}

	if len(args) == 0 {
		for _, opt := range f.daemonOpts {
			args = append(args, opt()...)
		}
	}

	out, err := f.core.Start(ctx, wait, args...)
	if err != nil {
		return nil, err
	}

	if err := f.setupStderrCapturing(); err != nil {
		return nil, err
	}

	idinfo, err := f.ID(ctx)
	if err != nil {
		return nil, err
	}

	f.PeerID = idinfo.ID

	return out, nil
}

// StopDaemon stops the filecoin daemon process.
func (f *Filecoin) StopDaemon(ctx context.Context) error {
	if err := f.core.Stop(ctx); err != nil {
		// TODO this may break the `IsAlive` parameter
		return err
	}

	return f.teardownStderrCapturing()
}

// Shell starts a user shell targeting the filecoin process
func (f *Filecoin) Shell() error {
	return f.core.Shell(f.ctx, []testbedi.Core{})
}

// Dir returns the dirtectory used by the filecoin process
func (f *Filecoin) Dir() string {
	return f.core.Dir()
}

// DumpLastOutput writes all the output (args, exit-code, error, stderr, stdout) of the last ran
// command from RunCmdWithStdin, RunCmdJSONWithStdin, or RunCmdLDJSONWithStdin.
func (f *Filecoin) DumpLastOutput(w io.Writer) {
	if f.lastCmdOutput != nil {
		fastutil.DumpOutput(w, f.lastCmdOutput)
	} else {
		fmt.Fprintln(w, "<nil>") // nolint: errcheck
	}
}

// DumpLastOutputJSON writes all the output (args, exit-code, error, stderr, stdout) of the last ran
// command from RunCmdWithStdin, RunCmdJSONWithStdin, or RunCmdLDJSONWithStdin as json.
func (f *Filecoin) DumpLastOutputJSON(w io.Writer) {
	if f.lastCmdOutput != nil {
		fastutil.DumpOutputJSON(w, f.lastCmdOutput)
	} else {
		fmt.Fprintln(w, "{}") // nolint: errcheck
	}
}

// outputMirror is used to store a copy of output lazily
type outputMirror struct {
	args     []string
	err      error
	exitcode int

	stdout bytes.Buffer
	stderr bytes.Buffer
}

func (o *outputMirror) Args() []string {
	return o.args
}

func (o *outputMirror) Error() error {
	return o.err
}

func (o *outputMirror) ExitCode() int {
	return o.exitcode
}

func (o *outputMirror) Stdout() io.ReadCloser {
	return ioutil.NopCloser(&o.stdout)
}

func (o *outputMirror) Stderr() io.ReadCloser {
	return ioutil.NopCloser(&o.stderr)
}

type output struct {
	out      testbedi.Output
	mirrored *outputMirror

	stdout io.ReadCloser
	stderr io.ReadCloser
}

// newOutput is used to wrap an output mirring data read from stdout / stderr to the outputMirror
func newOutput(out testbedi.Output) *output {
	lastOutput := &outputMirror{}
	lastOutput.args = out.Args()

	// outputMirror contains a bytes.Buffer which we want to mirror any data read
	// from the output.Stdxxx() to be written too. To do this we use a TeeReader
	// which will write any data read from its reader to the writer. We want to
	// limit the amount of data that is mirred, as it's primarily a debug tool,
	// and we don't want to store a large file when performing actions such as
	// client cat, or retrieval-client retrieve-piece.
	wstdout := limitWriter(&lastOutput.stdout, 1024)
	wstderr := limitWriter(&lastOutput.stderr, 1024)

	stdout := out.Stdout()
	stderr := out.Stderr()

	rstdout := io.TeeReader(stdout, wstdout)
	rstderr := io.TeeReader(stderr, wstderr)

	return &output{
		out:      out,
		mirrored: lastOutput,
		stdout: &readCloser{
			r: rstdout,
			closer: func() error {
				return stdout.Close()
			},
		},
		stderr: &readCloser{
			r: rstderr,
			closer: func() error {
				return stderr.Close()
			},
		},
	}
}

func (o *output) Args() []string {
	o.mirrored.args = o.out.Args()
	return o.mirrored.args
}

func (o *output) Error() error {
	o.mirrored.err = o.out.Error()
	return o.mirrored.err
}

func (o *output) ExitCode() int {
	o.mirrored.exitcode = o.out.ExitCode()
	return o.mirrored.exitcode
}

func (o *output) Stdout() io.ReadCloser {
	return o.stdout
}

func (o *output) Stderr() io.ReadCloser {
	return o.stderr
}

type limitedWriter struct {
	w io.Writer
	n int64
}

// Like io.LimitReader, but limits how much is written. The same effect can be
// done with io.Pipe and a LimitReader, but requires a go-routine and some redirection
// to dump data as the pipes are synchronous.
func limitWriter(w io.Writer, n int64) io.Writer {
	return &limitedWriter{w, n}
}

func (l *limitedWriter) Write(b []byte) (int, error) {
	if l.n <= 0 {
		return len(b), nil
	}

	if int64(len(b)) > l.n {
		b = b[0:l.n]
	}

	n, err := l.w.Write(b)
	l.n -= int64(n)
	return n, err
}

// io.TeeReader remove the Closer from our reader, so we need to wrap it again
type readCloser struct {
	r      io.Reader
	closer func() error
}

func (rc *readCloser) Read(b []byte) (int, error) {
	return rc.r.Read(b)
}

func (rc *readCloser) Close() error {
	return rc.closer()
}

// RunCmdWithStdin runs `args` against Filecoin process `f`, a testbedi.Output and an error are returned.
func (f *Filecoin) RunCmdWithStdin(ctx context.Context, stdin io.Reader, args ...string) (testbedi.Output, error) {
	if ctx == nil {
		ctx = f.ctx
	}
	f.Log.Infof("RunCmd: %s %s", f.core.Dir(), args)
	out, err := f.core.RunCmd(ctx, stdin, args...)
	if err != nil {
		return nil, err
	}

	output := newOutput(out)
	f.lastCmdOutput = output.mirrored
	return output, nil
}

// RunCmdJSONWithStdin runs `args` against Filecoin process `f`. The '--enc=json' flag
// is appened to the command specified by `args`, the result of the command is marshaled into `v`.
func (f *Filecoin) RunCmdJSONWithStdin(ctx context.Context, stdin io.Reader, v interface{}, args ...string) error {
	args = append(args, "--enc=json")
	output, err := f.RunCmdWithStdin(ctx, stdin, args...)
	if err != nil {
		return err
	}

	// To get an exit code the process must have exited.
	// For the process to exit, we have to read all of the stdout / stderr, as
	// the process may be blocked from finishing if the kernel buffer fills.
	g, _ := errgroup.WithContext(ctx)

	stdout := output.Stdout()
	stderr := output.Stderr()

	var bstdout []byte

	g.Go(func() (err error) {
		_, err = ioutil.ReadAll(stderr)
		return
	})

	g.Go(func() (err error) {
		bstdout, err = ioutil.ReadAll(stdout)
		return
	})

	if err := g.Wait(); err != nil {
		return err
	}

	if err := getOutputError(output); err != nil {
		return err
	}

	return json.Unmarshal(bstdout, &v)
}

type DecodeCloser interface {
	Decode(v interface{}) error
	Close() error
}

type decoderCloser struct {
	d      *json.Decoder
	closer func() error
}

func (dc *decoderCloser) Decode(v interface{}) error {
	return dc.d.Decode(v)
}

func (dc *decoderCloser) Close() error {
	return dc.closer()
}

// RunCmdLDJSONWithStdin runs `args` against Filecoin process `f`. The '--enc=json' flag
// is appened to the command specified by `args`. The result of the command is returned
// as a json.Decoder that may be used to read and decode JSON values from the result of
// the command.
func (f *Filecoin) RunCmdLDJSONWithStdin(ctx context.Context, stdin io.Reader, args ...string) (DecodeCloser, error) {
	args = append(args, "--enc=json")
	out, err := f.RunCmdWithStdin(ctx, stdin, args...)
	if err != nil {
		return nil, err
	}

	stdout := out.Stdout()

	dec := json.NewDecoder(stdout)
	closer := func() error {
		if err := stdout.Close(); err != nil {
			return err
		}

		if err := getOutputError(out); err != nil {
			return err
		}

		return nil
	}

	dc := &decoderCloser{dec, closer}

	return dc, nil
}

// Config return the config file of the FAST process.
func (f *Filecoin) Config() (*fcconfig.Config, error) {
	fcc, err := f.core.Config()
	if err != nil {
		return nil, err
	}
	cfg, ok := fcc.(*fcconfig.Config)
	if !ok {
		return nil, fmt.Errorf("failed to cast filecoin config struct")
	}

	return cfg, nil
}

// WriteConfig writes the config `cgf` to the FAST process's repo.
func (f *Filecoin) WriteConfig(cfg *fcconfig.Config) error {
	return f.core.WriteConfig(cfg)
}

func getOutputError(output testbedi.Output) error {
	exitcode := output.ExitCode()
	if exitcode > 0 {
		return fmt.Errorf("filecoin command: %s, exited with non-zero exitcode: %d", output.Args(), exitcode)
	} else if exitcode == -1 {
		return output.Error()
	}

	return nil
}

package fat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	iptb "github.com/ipfs/iptb/testbed"
	"github.com/ipfs/iptb/testbed/interfaces"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"

	fatutil "github.com/filecoin-project/go-filecoin/tools/fat/fatutil"
	dockerplugin "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/docker"
	localplugin "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/local"
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

	// StderrReader is require to gather daemon logs during action execution
	StderrReader() (io.ReadCloser, error)
}

// Filecoin represents a wrapper around the iptb Core interface.
type Filecoin struct {
	PeerID peer.ID

	Log logging.EventLogger

	core IPTBCoreExt
	ctx  context.Context

	lastCmdOutput testbedi.Output

	stderr io.ReadCloser

	lpCtx    context.Context
	lpCancel context.CancelFunc
	lpErr    error
	lp       *fatutil.LinePuller
	ir       fatutil.IntervalRecorder
}

// NewFilecoinProcess returns a pointer to a Filecoin process that wraps the IPTB core interface `c`.
func NewFilecoinProcess(ctx context.Context, c IPTBCoreExt) *Filecoin {
	return &Filecoin{
		core: c,
		Log:  logging.Logger(fmt.Sprintf("%s", c.String())),
		ctx:  ctx,
	}
}

// InitDaemon initializes the filecoin daemon process.
func (f *Filecoin) InitDaemon(ctx context.Context, args ...string) (testbedi.Output, error) {
	return f.core.Init(ctx, args...)
}

// StartDaemon starts the filecoin daemon process.
func (f *Filecoin) StartDaemon(ctx context.Context, wait bool, args ...string) (testbedi.Output, error) {
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

	f.PeerID, err = peer.IDFromString(strings.Trim(idinfo.ID, `"`))
	if err != nil {
		return nil, err
	}

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

// DumpLastOutput writes all the output (args, exit-code, error, stderr, stdout) of the last ran
// command from RunCmdWithStdin, RunCmdJSONWithStdin, or RunCmdLDJSONWithStdin.
func (f *Filecoin) DumpLastOutput(w io.Writer) {
	fatutil.DumpOutput(w, f.lastCmdOutput)
}

// DumpLastOutputJSON writes all the output (args, exit-code, error, stderr, stdout) of the last ran
// command from RunCmdWithStdin, RunCmdJSONWithStdin, or RunCmdLDJSONWithStdin as json.
func (f *Filecoin) DumpLastOutputJSON(w io.Writer) {
	fatutil.DumpOutputJSON(w, f.lastCmdOutput)
}

// RunCmdWithStdin runs `args` against Filecoin process `f`, a testbedi.Output and an error are returned.
func (f *Filecoin) RunCmdWithStdin(ctx context.Context, stdin io.Reader, args ...string) (testbedi.Output, error) {
	if ctx == nil {
		ctx = f.ctx
	}
	f.Log.Infof("RunCmd: %s", args)
	out, err := f.core.RunCmd(ctx, stdin, args...)
	if err != nil {
		return nil, err
	}

	f.lastCmdOutput = out
	return out, nil
}

// RunCmdJSONWithStdin runs `args` against Filecoin process `f`. The '--enc=json' flag
// is appened to the command specified by `args`, the result of the command is marshaled into `v`.
func (f *Filecoin) RunCmdJSONWithStdin(ctx context.Context, stdin io.Reader, v interface{}, args ...string) error {
	args = append(args, "--enc=json")
	out, err := f.RunCmdWithStdin(ctx, stdin, args...)
	if err != nil {
		return err
	}

	// check command exit code
	if out.ExitCode() > 0 {
		return fmt.Errorf("filecoin command: %s, exited with non-zero exitcode: %d", out.Args(), out.ExitCode())
	}

	dec := json.NewDecoder(out.Stdout())
	return dec.Decode(v)
}

// RunCmdLDJSONWithStdin runs `args` against Filecoin process `f`. The '--enc=json' flag
// is appened to the command specified by `args`. The result of the command is returned
// as a json.Decoder that may be used to read and decode JSON values from the result of
// the command.
func (f *Filecoin) RunCmdLDJSONWithStdin(ctx context.Context, stdin io.Reader, args ...string) (*json.Decoder, error) {
	args = append(args, "--enc=json")
	out, err := f.RunCmdWithStdin(ctx, stdin, args...)
	if err != nil {
		return nil, err
	}

	// check command exit code
	if out.ExitCode() > 0 {
		return nil, fmt.Errorf("filecoin command: %s, exited with non-zero exitcode: %d", out.Args(), out.ExitCode())
	}

	return json.NewDecoder(out.Stdout()), nil
}

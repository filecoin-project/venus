package pluginlocalfilecoin

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/config"

	"github.com/ipfs/iptb/testbed/interfaces"
	"github.com/ipfs/iptb/util"
	"golang.org/x/sync/errgroup"

	"github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin"
)

// PluginName is the name of the plugin
var PluginName = "localfilecoin"

var log = logging.Logger(PluginName)

// errIsAlive will be returned by Start if the node is already running
var errIsAlive = errors.New("node is already running")
var errTimeout = errors.New("timeout")

// defaultFilecoinBinary is the name or full path of the binary that will be used
const defaultFilecoinBinary = "go-filecoin"

// defaultRepoPath is the name of the repo path relative to the plugin root directory
const defaultRepoPath = "repo"

// defaultSectorsPath is the name of the sector path relative to the plugin root directory
const defaultSectorsPath = "sectors"

// defaultLogLevel is the value that will be used for GO_FILECOIN_LOG_LEVEL
const defaultLogLevel = "3"

// defaultLogJSON is the value that will be used for GO_FILECOIN_LOG_JSON
const defaultLogJSON = "false"

var (
	// AttrFilecoinBinary is the key used to set which binary to use in the plugin through NewNode attrs
	AttrFilecoinBinary = "filecoinBinary"

	// AttrLogLevel is the key used to set the log level through NewNode attrs
	AttrLogLevel = "logLevel"

	// AttrLogJSON is the key used to set the node to output json logs
	AttrLogJSON = "logJSON"

	// AttrSectorsPath is the key used to set the sectors path
	AttrSectorsPath = "sectorsPath"
)

// Localfilecoin represents a filecoin node
type Localfilecoin struct {
	iptbPath string // Absolute path for all process data
	peerid   cid.Cid
	apiaddr  multiaddr.Multiaddr

	binPath     string // Absolute path to binary
	repoPath    string // Absolute path to repo
	sectorsPath string // Absolute path to sectors
	logLevel    string
	logJSON     string
}

var NewNode testbedi.NewNodeFunc // nolint: golint

func init() {
	NewNode = func(dir string, attrs map[string]string) (testbedi.Core, error) {
		dir, err := filepath.Abs(dir)
		if err != nil {
			return nil, err
		}
		var (
			binPath     = ""
			repoPath    = filepath.Join(dir, defaultRepoPath)
			sectorsPath = filepath.Join(dir, defaultSectorsPath)
			logLevel    = defaultLogLevel
			logJSON     = defaultLogJSON
		)

		if v, ok := attrs[AttrFilecoinBinary]; ok {
			binPath = v
		}

		if v, ok := attrs[AttrLogLevel]; ok {
			logLevel = v
		}

		if v, ok := attrs[AttrLogJSON]; ok {
			logJSON = v
		}

		if v, ok := attrs[AttrSectorsPath]; ok {
			sectorsPath = v
		}

		if len(binPath) == 0 {
			if binPath, err = exec.LookPath(defaultFilecoinBinary); err != nil {
				return nil, err
			}
		}

		if err := os.Mkdir(filepath.Join(dir, "bin"), 0755); err != nil {
			return nil, fmt.Errorf("could not make dir: %s", err)
		}

		dst := filepath.Join(dir, "bin", filepath.Base(binPath))
		if err := copyFileContents(binPath, dst); err != nil {
			return nil, err
		}

		if err := os.Chmod(dst, 0755); err != nil {
			return nil, err
		}

		return &Localfilecoin{
			iptbPath:    dir,
			binPath:     dst,
			repoPath:    repoPath,
			sectorsPath: sectorsPath,
			logLevel:    logLevel,
			logJSON:     logJSON,
		}, nil
	}
}

/** Core Interface **/

// Init runs the node init process.
func (l *Localfilecoin) Init(ctx context.Context, args ...string) (testbedi.Output, error) {
	// The repo path is provided by the environment
	args = append([]string{l.binPath, "init"}, args...)
	output, oerr := l.RunCmd(ctx, nil, args...)
	if oerr != nil {
		return nil, oerr
	}
	if output.ExitCode() != 0 {
		return output, errors.Errorf("%s exited with non-zero code %d", output.Args(), output.ExitCode())
	}

	icfg, err := l.Config()
	if err != nil {
		return nil, err
	}

	lcfg := icfg.(*config.Config)

	if err := lcfg.Set("api.address", `"/ip4/127.0.0.1/tcp/0"`); err != nil {
		return nil, err
	}

	if err := lcfg.Set("swarm.address", `"/ip4/127.0.0.1/tcp/0"`); err != nil {
		return nil, err
	}

	// only set sectors path to l.sectorsPath if init command does not set
	isectorsPath, err := lcfg.Get("sectorbase.rootdir")
	if err != nil {
		return nil, err
	}
	lsectorsPath := isectorsPath.(string)
	if lsectorsPath == "" {
		if err := lcfg.Set("sectorbase.rootdir", l.sectorsPath); err != nil {
			return nil, err
		}
	}

	if err := l.WriteConfig(lcfg); err != nil {
		return nil, err
	}

	return output, oerr
}

// Start starts the node process.
func (l *Localfilecoin) Start(ctx context.Context, wait bool, args ...string) (testbedi.Output, error) {
	alive, err := l.isAlive()
	if err != nil {
		return nil, err
	}

	if alive {
		return nil, errIsAlive
	}

	repoFlag := fmt.Sprintf("--repodir=%s", l.repoPath) // Not provided by environment here
	dargs := append([]string{"daemon", repoFlag}, args...)
	cmd := exec.CommandContext(ctx, l.binPath, dargs...)
	cmd.Dir = l.iptbPath

	cmd.Env, err = l.env()
	if err != nil {
		return nil, err
	}

	iptbutil.SetupOpt(cmd)

	stdout, err := os.Create(filepath.Join(l.iptbPath, "daemon.stdout"))
	if err != nil {
		return nil, err
	}

	stderr, err := os.Create(filepath.Join(l.iptbPath, "daemon.stderr"))
	if err != nil {
		return nil, err
	}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	pid := cmd.Process.Pid
	if pid == 0 {
		panic("here")
	}

	l.Infof("Started daemon: %s, pid: %d", l, pid)

	if err := ioutil.WriteFile(filepath.Join(l.iptbPath, "daemon.pid"), []byte(fmt.Sprint(pid)), 0666); err != nil {
		return nil, err
	}
	if wait {
		if err := filecoin.WaitOnAPI(l); err != nil {
			return nil, err
		}
	}
	return iptbutil.NewOutput(dargs, []byte{}, []byte{}, 0, err), nil
}

// Stop stops the node process.
func (l *Localfilecoin) Stop(ctx context.Context) error {
	pid, err := l.getPID()
	if err != nil {
		return fmt.Errorf("error killing daemon %s: %s", l.iptbPath, err)
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("error killing daemon %s: %s", l.iptbPath, err)
	}

	waitch := make(chan struct{}, 1)
	go func() {
		// TODO pass return state
		p.Wait() // nolint: errcheck
		waitch <- struct{}{}
	}()

	defer func() {
		err := os.Remove(filepath.Join(l.iptbPath, "daemon.pid"))
		if err != nil && !os.IsNotExist(err) {
			panic(fmt.Errorf("error removing pid file for daemon at %s: %s", l.iptbPath, err))
		}
		err = os.Remove(filepath.Join(l.repoPath, "api"))
		if err != nil && !os.IsNotExist(err) {
			panic(fmt.Errorf("error removing API file for daemon at %s: %s", l.repoPath, err))
		}
	}()

	if err := l.signalAndWait(p, waitch, syscall.SIGTERM, 1*time.Second); err != errTimeout {
		return err
	}

	if err := l.signalAndWait(p, waitch, syscall.SIGTERM, 2*time.Second); err != errTimeout {
		return err
	}

	if err := l.signalAndWait(p, waitch, syscall.SIGQUIT, 5*time.Second); err != errTimeout {
		return err
	}

	if err := l.signalAndWait(p, waitch, syscall.SIGKILL, 5*time.Second); err != errTimeout {
		return err
	}

	for {
		err := p.Signal(syscall.Signal(0))
		if err != nil {
			break
		}
		time.Sleep(time.Millisecond * 10)
	}

	return nil
}

// RunCmd runs a command in the context of the node.
func (l *Localfilecoin) RunCmd(ctx context.Context, stdin io.Reader, args ...string) (testbedi.Output, error) {
	env, err := l.env()
	if err != nil {
		return nil, fmt.Errorf("error getting env: %s", err)
	}

	firstArg := args[0]
	if firstArg == "go-filecoin" {
		firstArg = l.binPath
	}

	cmd := exec.CommandContext(ctx, firstArg, args[1:]...)
	cmd.Env = env
	cmd.Stdin = stdin

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	g, ctx := errgroup.WithContext(ctx)

	var stderrbytes []byte
	var stdoutbytes []byte

	g.Go(func() error {
		var err error
		stderrbytes, err = ioutil.ReadAll(stderr)
		return err
	})

	g.Go(func() error {
		var err error
		stdoutbytes, err = ioutil.ReadAll(stdout)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	exiterr := cmd.Wait()

	var exitcode = 0
	switch oerr := exiterr.(type) {
	case *exec.ExitError:
		if ctx.Err() == context.DeadlineExceeded {
			err = errors.Wrapf(oerr, "context deadline exceeded for command: %q", strings.Join(cmd.Args, " "))
		}

		exitcode = 1
	case nil:
		err = oerr
	}

	return iptbutil.NewOutput(args, stdoutbytes, stderrbytes, exitcode, err), nil
}

// Connect connects the node to another testbed node.
func (l *Localfilecoin) Connect(ctx context.Context, n testbedi.Core) error {
	swarmaddrs, err := n.SwarmAddrs()
	if err != nil {
		return err
	}

	output, err := l.RunCmd(ctx, nil, l.binPath, "swarm", "connect", swarmaddrs[0])

	if err != nil {
		return err
	}

	if output.ExitCode() != 0 {
		out, err := ioutil.ReadAll(output.Stderr())
		if err != nil {
			return err
		}

		return fmt.Errorf("%s", string(out))
	}

	return err
}

// Shell starts a user shell in the context of a node setting FIL_PATH to ensure calls to
// go-filecoin will be ran agasint the target node. Stderr, stdout will be set to os.Stderr
// and os.Stdout. If env TTY is set, it will be used for stdin, otherwise os.Stdin will be used.
//
// If FIL_PATH is already set, an error will be returned.
//
// The shell environment will have the follow variables set in the shell for the user.
//
// NODE0-NODE# - set to the PeerID for each value in ns passed.
// FIL_PATH    - The value is set to the directory for the Filecoin node.
// FIL_PID     - The value is set to the pid for the Filecoin daemon
// FIL_BINARY  - The value is set to the path of the binary used for running the Filecoin daemon.
// PATH        - The users PATH will be updated to include a location that contains the FIL_BINARY.
//
// Note: user shell configuration may lead to the `go-filecoin` command not pointing to FIL_BINARY,
// due to PATH ordering.
func (l *Localfilecoin) Shell(ctx context.Context, ns []testbedi.Core) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return fmt.Errorf("no shell found")
	}

	if len(os.Getenv("FIL_PATH")) != 0 {
		// If the users shell sets FIL_PATH, it will just be overridden by the shell again
		return fmt.Errorf("shell has FIL_PATH set, please unset before trying to use iptb shell")
	}

	nenvs, err := l.env()
	if err != nil {
		return err
	}

	// TODO(tperson): It would be great if we could guarantee that the shell
	// is using the same binary. However, the users shell may prepend anything
	// we change in the PATH

	for i, n := range ns {
		peerid, err := n.PeerID()

		if err != nil {
			return err
		}

		nenvs = append(nenvs, fmt.Sprintf("NODE%d=%s", i, peerid))
	}

	pid, err := l.getPID()
	if err != nil {
		return err
	}

	nenvs = filecoin.UpdateOrAppendEnv(nenvs, "FIL_PID", fmt.Sprintf("%d", pid))
	nenvs = filecoin.UpdateOrAppendEnv(nenvs, "FIL_BINARY", l.binPath)

	cmd := exec.CommandContext(ctx, shell)
	cmd.Env = nenvs

	stdin := os.Stdin

	// When running code with `go test`, the os.Stdin is not connected to the shell
	// where `go test` was ran. This makes the shell exit immediately and it's not
	// possible to run it. To get around this issue we can let the user tell us the
	// TTY their shell is using by setting the TTY env. This will allow the shell
	// to use the same TTY the user started running `go test` in.
	tty := os.Getenv("TTY")
	if len(tty) != 0 {
		f, err := os.Open(tty)
		if err != nil {
			return err
		}

		stdin = f
	}

	cmd.Stdin = stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Start(); err != nil {
		return err
	}

	return cmd.Wait()
}

// Infof writes an info log.
func (l *Localfilecoin) Infof(format string, args ...interface{}) {
	log.Infof("Node: %s %s", l, fmt.Sprintf(format, args...))
}

// Errorf writes an error log.
func (l *Localfilecoin) Errorf(format string, args ...interface{}) {
	log.Errorf("Node: %s %s", l, fmt.Sprintf(format, args...))
}

// Dir returns the IPTB directory the node is using.
func (l *Localfilecoin) Dir() string {
	return l.iptbPath
}

// Type returns the type of the node.
func (l *Localfilecoin) Type() string {
	return PluginName
}

// String implements the stringr interface.
func (l *Localfilecoin) String() string {
	return l.iptbPath
}

/** Libp2p Interface **/

// PeerID returns the nodes peerID.
func (l *Localfilecoin) PeerID() (string, error) {
	/*
		if l.peerid != nil {
			return l.peerid, nil
		}
	*/

	var err error
	l.peerid, err = l.GetPeerID()
	if err != nil {
		return "", err
	}

	return l.peerid.String(), err
}

// APIAddr returns the api address of the node.
func (l *Localfilecoin) APIAddr() (string, error) {
	/*
		if l.apiaddr != nil {
			return l.apiaddr, nil
		}
	*/

	var err error
	l.apiaddr, err = filecoin.GetAPIAddrFromRepo(l.repoPath)
	if err != nil {
		return "", err
	}

	return l.apiaddr.String(), err
}

// SwarmAddrs returns the addresses a node is listening on for swarm connections.
func (l *Localfilecoin) SwarmAddrs() ([]string, error) {
	out, err := l.RunCmd(context.Background(), nil, l.binPath, "id", "--format=<addrs>")
	if err != nil {
		return nil, err
	}

	outStr, err := ioutil.ReadAll(out.Stdout())
	if err != nil {
		return nil, err
	}

	addrs := strings.Split(string(outStr), "\n")
	return addrs, nil
}

/** Config Interface **/

// Config returns the nodes config.
func (l *Localfilecoin) Config() (interface{}, error) {
	return config.ReadFile(filepath.Join(l.repoPath, "config.json"))
}

// WriteConfig writes a nodes config file.
func (l *Localfilecoin) WriteConfig(cfg interface{}) error {
	lcfg := cfg.(*config.Config)
	return lcfg.WriteFile(filepath.Join(l.repoPath, "config.json"))
}

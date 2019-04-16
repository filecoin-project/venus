package pluginlocalfilecoin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/ipfs/iptb/testbed/interfaces"
	"github.com/ipfs/iptb/util"
)

// PluginName is the name of the plugin.
var PluginName = "dockerfilecoin"
var log = logging.Logger(PluginName)

// DefaultDockerHost is the hostname used when connecting to a docker daemon.
var DefaultDockerHost = client.DefaultDockerHost

// DefaultDockerImage is the image name the plugin will use when deploying a container>
var DefaultDockerImage = "go-filecoin"

// DefaultDockerUser is the user that will run the command(s) inside the container.
var DefaultDockerUser = "filecoin"

// DefaultDockerEntryPoint is the entrypoint to run when starting the container.
var DefaultDockerEntryPoint = []string{"/usr/local/bin/go-filecoin"}

// DefaultDockerVolumePrefix is a prefix added when using docker volumes
// e.g. when running against a remote docker daemon a prefix like `/var/iptb/`
// is usful wrt permissions
var DefaultDockerVolumePrefix = ""

// DefaultLogLevel is the value that will be used for GO_FILECOIN_LOG_LEVEL
var DefaultLogLevel = "3"

// DefaultLogJSON is the value that will be used for GO_FILECOIN_LOG_JSON
var DefaultLogJSON = "false"

var (
	// AttrLogLevel is the key used to set the log level through NewNode attrs
	AttrLogLevel = "logLevel"

	// AttrLogJSON is the key used to set the node to output json logs
	AttrLogJSON = "logJSON"
)

// Dockerfilecoin represents attributes of a dockerized filecoin node.
type Dockerfilecoin struct {
	Image        string
	ID           string
	Host         string
	User         string
	EntryPoint   []string
	VolumePrefix string
	dir          string
	peerid       cid.Cid
	apiaddr      multiaddr.Multiaddr
	swarmaddr    multiaddr.Multiaddr

	logLevel string
	logJSON  string
}

var NewNode testbedi.NewNodeFunc // nolint: golint

func init() {
	NewNode = func(dir string, attrs map[string]string) (testbedi.Core, error) {
		dockerHost := DefaultDockerHost
		dockerImage := DefaultDockerImage
		dockerUser := DefaultDockerUser
		dockerEntry := DefaultDockerEntryPoint
		dockerVolumePrefix := DefaultDockerVolumePrefix
		logLevel := DefaultLogLevel
		logJSON := DefaultLogJSON

		// the dockerid file is present once the container has started the daemon process,
		// iptb uses the dockerid file to keep track of the containers its running
		idb, err := ioutil.ReadFile(filepath.Join(dir, "dockerid"))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		dockerID := string(idb)

		apiaddr, err := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/3453")
		if err != nil {
			return nil, err
		}

		swarmaddr, err := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/6000")
		if err != nil {
			return nil, err
		}

		if v, ok := attrs["dockerHost"]; ok {
			dockerHost = v
		}

		if v, ok := attrs["dockerImage"]; ok {
			dockerImage = v
		}

		if v, ok := attrs["dockerUser"]; ok {
			dockerUser = v
		}

		if v, ok := attrs["dockerEntry"]; ok {
			dockerEntry[0] = v
		}

		if v, ok := attrs["dockerVolumePrefix"]; ok {
			dockerVolumePrefix = v
		}

		if v, ok := attrs[AttrLogLevel]; ok {
			logLevel = v
		}

		if v, ok := attrs[AttrLogJSON]; ok {
			logJSON = v
		}

		return &Dockerfilecoin{
			EntryPoint:   dockerEntry,
			Host:         dockerHost,
			ID:           dockerID,
			Image:        dockerImage,
			User:         dockerUser,
			VolumePrefix: dockerVolumePrefix,

			logLevel: logLevel,
			logJSON:  logJSON,

			dir:       dir,
			apiaddr:   apiaddr,
			swarmaddr: swarmaddr,
		}, nil
	}
}

/** Core Interface **/

// Init runs the node init process.
func (l *Dockerfilecoin) Init(ctx context.Context, args ...string) (testbedi.Output, error) {
	// Get the docker client
	cli, err := l.GetClient()
	if err != nil {
		return nil, err
	}

	// define entrypoint command
	// TODO use an env var
	cmds := []string{"init"}
	cmds = append(cmds, args...)

	envs, err := l.env()
	if err != nil {
		return nil, err
	}

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Entrypoint: l.EntryPoint,
			User:       l.User,
			Image:      l.Image,
			Env:        envs,
			Cmd:        cmds,
			Tty:        false,
		},
		&container.HostConfig{
			Binds: []string{
				fmt.Sprintf("%s%s:%s", l.VolumePrefix, l.Dir(), "/data/filecoin"),
			},
		},
		&network.NetworkingConfig{},
		"")
	if err != nil {
		return nil, err
	}

	// This runs the init command, when the command completes the container will stop running, since we
	// have the added the bindings above the repo will be persisted, meaning we can create a second container
	// during the `iptb start` command that will use the repo created here.
	// TODO find a better way to do that above..
	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, err
	}

	var exitCode int64
	// wait here until the init command completes and get an exit code
	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return nil, err
		}
	case exitStatus := <-statusCh:
		exitCode = exitStatus.StatusCode
	}

	// collect logs generated during container execution
	out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		panic(err)
	}
	defer out.Close() // nolint: errcheck

	var outBuf, errBuf bytes.Buffer
	outputDone := make(chan error)
	go func() {
		// StdCopy demultiplexes the stream into two buffers
		_, err = stdcopy.StdCopy(&outBuf, &errBuf, out)
		outputDone <- err
	}()

	select {
	case err := <-outputDone:
		if err != nil {
			return nil, err
		}
		break
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return iptbutil.NewOutput(cmds, outBuf.Bytes(), errBuf.Bytes(), int(exitCode), err), err
}

// Start starts the node process.
func (l *Dockerfilecoin) Start(ctx context.Context, wait bool, args ...string) (testbedi.Output, error) {
	// check if we already have a container running in the testbed dir
	if _, err := os.Stat(filepath.Join(l.Dir(), "dockerid")); err == nil {
		return nil, errors.Errorf("container already running in testbed dir: %s", l.Dir())
	}

	cli, err := l.GetClient()
	if err != nil {
		return nil, err
	}

	// TODO use an env var
	cmds := []string{"daemon"}
	cmds = append(cmds, args...)

	envs, err := l.env()
	if err != nil {
		return nil, err
	}

	// Create the container, first command needs to be daemon, now we have an ID for it
	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Entrypoint:   l.EntryPoint,
			User:         l.User,
			Image:        l.Image,
			Env:          envs,
			Cmd:          cmds,
			Tty:          false,
			AttachStdout: true,
			AttachStderr: true,
		},
		&container.HostConfig{
			Binds: []string{fmt.Sprintf("%s%s:%s", l.VolumePrefix, l.Dir(), "/data/filecoin")},
		},
		&network.NetworkingConfig{},
		"")
	if err != nil {
		return nil, err
	}

	// this runs the daemon command, the container will now remain running until stop is called
	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, err
	}

	// TODO this when it would be nice to have filecoin log to a file, then we could just mount that..
	// Sleep for a bit, else we don't see any logs
	time.Sleep(2 * time.Second)

	out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return nil, err
	}
	defer out.Close() // nolint: errcheck

	var outBuf, errBuf bytes.Buffer
	outputDone := make(chan error)
	go func() {
		// StdCopy demultiplexes the stream into two buffers
		_, err = stdcopy.StdCopy(&outBuf, &errBuf, out)
		outputDone <- err
	}()

	select {
	case err := <-outputDone:
		if err != nil {
			return nil, err
		}
		break
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// save the dockerid to a file
	if err = ioutil.WriteFile(filepath.Join(l.Dir(), "dockerid"), []byte(resp.ID), 0664); err != nil {
		return nil, err
	}

	return iptbutil.NewOutput(cmds, outBuf.Bytes(), errBuf.Bytes(), int(0), err), err
}

// Stop stops the node process.
func (l *Dockerfilecoin) Stop(ctx context.Context) error {
	cli, err := l.GetClient()
	if err != nil {
		return err
	}

	// "2" is the same as Ctrl+C
	if err := cli.ContainerKill(ctx, l.ID, "2"); err != nil {
		return err
	}

	// remove the dockerid file since we use this in `Start` as a liveness check
	if err := os.Remove(filepath.Join(l.Dir(), "dockerid")); err != nil {
		return err
	}

	return nil
}

// RunCmd runs a command in the context of the node.
func (l *Dockerfilecoin) RunCmd(ctx context.Context, stdin io.Reader, args ...string) (testbedi.Output, error) {
	// TODO pass opts here to control a docker daemon on a remote host
	cli, err := l.GetClient()
	if err != nil {
		return nil, err
	}

	// TODO use an env var
	return Exec(ctx, cli, l.ID, false, "/data/filecoin", args...)
}

// Connect connects the node to another testbed node.
func (l *Dockerfilecoin) Connect(ctx context.Context, n testbedi.Core) error {
	swarmaddrs, err := n.SwarmAddrs()
	if err != nil {
		return err
	}

	for _, a := range swarmaddrs {
		// we should try all addresses
		// TODO(frrist) libp2p has a better way to do this built in iirc
		output, err := l.RunCmd(ctx, nil, "go-filecoin", "swarm", "connect", a)
		if err != nil {
			return err
		}

		if output.ExitCode() != 0 {
			out, err := ioutil.ReadAll(output.Stderr())
			if err != nil {
				return err
			}
			l.Errorf("%s", string(out))
		}
	}
	return nil
}

func (l *Dockerfilecoin) env() ([]string, error) {
	envs := os.Environ()

	envs = filecoin.UpdateOrAppendEnv(envs, "FIL_PATH", "/data/filecoin")
	envs = filecoin.UpdateOrAppendEnv(envs, "GO_FILECOIN_LOG_LEVEL", l.logLevel)
	envs = filecoin.UpdateOrAppendEnv(envs, "GO_FILECOIN_LOG_JSON", l.logJSON)

	return envs, nil
}

// Shell starts a shell in the context of a node.
func (l *Dockerfilecoin) Shell(ctx context.Context, ns []testbedi.Core) error {
	panic("NYI")
}

// Infof writes an info log.
func (l *Dockerfilecoin) Infof(format string, args ...interface{}) {
	log.Infof("Node: %s %s", l, fmt.Sprintf(format, args...))
}

// Errorf writes an error log.
func (l *Dockerfilecoin) Errorf(format string, args ...interface{}) {
	log.Errorf("Node: %s %s", l, fmt.Sprintf(format, args...))
}

// Dir returns the directory the node is using.
func (l *Dockerfilecoin) Dir() string {
	return l.dir
}

// Type returns the type of the node.
func (l *Dockerfilecoin) Type() string {
	return PluginName
}

// String implements the stringer interface.
func (l *Dockerfilecoin) String() string {
	return l.dir
}

/** Libp2p Interface **/

// PeerID returns the nodes peerID.
func (l *Dockerfilecoin) PeerID() (string, error) {
	var err error
	l.peerid, err = l.GetPeerID()
	if err != nil {
		return "", err
	}

	return l.peerid.String(), err
}

// APIAddr returns the api address of the node.
func (l *Dockerfilecoin) APIAddr() (string, error) {
	return l.apiaddr.String(), nil
}

// SwarmAddrs returns the addresses a node is listening on for swarm connections.
func (l *Dockerfilecoin) SwarmAddrs() ([]string, error) {
	out, err := l.RunCmd(context.Background(), nil, "go-filecoin", "id", "--format='<addrs>'")
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

// GetConfig returns the nodes config.
func (l *Dockerfilecoin) GetConfig() (interface{}, error) {
	panic("NYI")
}

// WriteConfig writes a nodes config file.
func (l *Dockerfilecoin) WriteConfig(cfg interface{}) error {
	panic("NYI")
}

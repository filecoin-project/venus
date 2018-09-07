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

	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/ipfs/iptb/testbed/interfaces"
	"github.com/ipfs/iptb/util"
)

// PluginName is the name of the plugin
var PluginName = "dockerfilecoin"

var ErrIsAlive = errors.New("node is already running") // nolint: golint
var errTimeout = errors.New("timeout")

// Dockerfilecoin represents a filecoin node
type Dockerfilecoin struct {
	img       string
	id        string
	dir       string
	peerid    *cid.Cid
	apiaddr   multiaddr.Multiaddr
	swarmaddr multiaddr.Multiaddr
}

var NewNode testbedi.NewNodeFunc // nolint: golint

func init() {
	NewNode = func(dir string, attrs map[string]string) (testbedi.Core, error) {

		apiaddr, err := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/3453")
		if err != nil {
			return nil, err
		}

		swarmaddr, err := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/6000")
		if err != nil {
			return nil, err
		}

		return &Dockerfilecoin{
			img: "go-filecoin",

			dir:       dir,
			apiaddr:   apiaddr,
			swarmaddr: swarmaddr,
		}, nil
	}
}

func GetClient() (*client.Client, error) {
	// TODO pass opts here to control a docker daemon on a remote host
	// E.g. -- yes the below works
	//return client.NewClientWithOpts(client.WithVersion("1.37"), client.WithHost("tcp://192.168.0.200:9090")) // client requires this version or lower
	return client.NewClientWithOpts(client.WithVersion("1.37"))
}

/** Core Interface **/

// Init runs the node init process.
func (l *Dockerfilecoin) Init(ctx context.Context, args ...string) (testbedi.Output, error) {
	// Get the docker client
	cli, err := GetClient()
	if err != nil {
		return nil, err
	}

	// define entrypoint command
	cmds := []string{"init"}
	cmds = append(cmds, args...)
	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Entrypoint: []string{"/usr/local/bin/go-filecoin"},
			User:       "filecoin",
			Image:      "go-filecoin",
			Cmd:        cmds,
			Tty:        false,
		},
		&container.HostConfig{
			Binds: []string{fmt.Sprintf("%s:%s", l.dir, "/data/filecoin")},
		}, nil, "")
	if err != nil {
		return nil, err
	}

	// This runs the init command, when the command completes the container will stop running, since we
	// have the `Bind`ings above the repo will be persisted, meaning we can create a second container
	// during the `iptb start` command that will use the repo created here.
	// TODO find a better way to do that above..
	if err := cli.ContainerStart(ctx, string(resp.ID), types.ContainerStartOptions{}); err != nil {
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
	defer out.Close()

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
	// IsAlive check
	if idb, err := ioutil.ReadFile(filepath.Join(l.dir, "dockerid")); !os.IsNotExist(err) {
		return nil, errors.Wrapf(err, "container already running in testbed dir with ID: %s", string(idb))
	}

	cli, err := GetClient()
	if err != nil {
		return nil, err
	}

	cmds := []string{"daemon"}
	cmds = append(cmds, args...)
	// Create the container, first command needs to be daemon, now we have an ID for it
	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Entrypoint:   []string{"/usr/local/bin/go-filecoin"},
			User:         "filecoin",
			Image:        "go-filecoin",
			Cmd:          cmds,
			Tty:          false,
			AttachStdout: true,
			AttachStderr: true,
		},
		&container.HostConfig{
			Binds: []string{fmt.Sprintf("%s:%s", l.dir, "/data/filecoin")},
		}, nil, "")
	if err != nil {
		return nil, err
	}

	// this runs the daemon command, the container will now remain running until stop is called
	if err := cli.ContainerStart(ctx, string(resp.ID), types.ContainerStartOptions{}); err != nil {
		return nil, err
	}

	// TODO this when it would be nice to have filecoin log to a file, then we could just mount that..
	// Sleep for a bit, else we don't see any logs
	time.Sleep(2 * time.Second)
	out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return nil, err
	}
	defer out.Close()

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

	// save the dockerid
	if err = ioutil.WriteFile(filepath.Join(l.dir, "dockerid"), []byte(resp.ID), 0664); err != nil {
		return nil, err
	}

	return iptbutil.NewOutput(cmds, outBuf.Bytes(), errBuf.Bytes(), int(0), err), err
}

// Stop stops the node process.
func (l *Dockerfilecoin) Stop(ctx context.Context) error {
	cli, err := GetClient()
	if err != nil {
		return err
	}

	idb, err := ioutil.ReadFile(filepath.Join(l.dir, "dockerid"))
	if err != nil {
		return err
	}

	if err := cli.ContainerKill(ctx, string(idb), "2"); err != nil {
		return err
	}

	if err := os.Remove(filepath.Join(l.dir, "dockerid")); err != nil {
		return err
	}

	return nil
}

// RunCmd runs a command in the context of the node.
func (l *Dockerfilecoin) RunCmd(ctx context.Context, stdin io.Reader, args ...string) (testbedi.Output, error) {
	// TODO pass opts here to control a docker daemon on a remote host
	cli, err := GetClient()
	if err != nil {
		return nil, err
	}

	idb, err := ioutil.ReadFile(filepath.Join(l.dir, "dockerid"))
	if err != nil {
		return nil, err
	}

	cmd := []string{"go-filecoin"}
	cmd = append(cmd, args...)
	return Exec(ctx, cli, string(idb), false, cmd...)
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
		output, err := l.RunCmd(ctx, nil, "swarm", "connect", a)
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

// Shell starts a shell in the context of a node.
func (l *Dockerfilecoin) Shell(ctx context.Context, ns []testbedi.Core) error {
	panic("NYI")
}

// Infof writes an info log.
func (l *Dockerfilecoin) Infof(format string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, "INFO-[%s]  %s\n", l.Dir(), fmt.Sprintf(format, args...)) // nolint: errcheck
}

// Errorf writes an error log.
func (l *Dockerfilecoin) Errorf(format string, args ...interface{}) {
	nformat := fmt.Sprintf("[ERROR]-[%s]  %s\n", l, format)
	fmt.Fprintf(os.Stderr, nformat, args...) // nolint: errcheck
}

// StderrReader returns a reader to the nodes stderr.
func (l *Dockerfilecoin) StderrReader() (io.ReadCloser, error) {
	panic("NYI")
}

// StdoutReader returns a reader to the nodes stdout.
func (l *Dockerfilecoin) StdoutReader() (io.ReadCloser, error) {
	panic("NYI")
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
	panic("NYI")
}

// SwarmAddrs returns the addresses a node is listening on for swarm connections.
func (l *Dockerfilecoin) SwarmAddrs() ([]string, error) {
	out, err := l.RunCmd(context.Background(), nil, "id", "--format=<addrs>")
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

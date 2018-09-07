package pluginlocalfilecoin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

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
	img        string
	imgAddr    string
	serverAddr string
	id         string
	dir        string
	peerid     *cid.Cid
	apiaddr    multiaddr.Multiaddr
	swarmaddr  multiaddr.Multiaddr
}

var NewNode testbedi.NewNodeFunc // nolint: golint

// ExecResult represents a result returned from Exec()
type ExecResult struct {
	ExitCode  int
	outBuffer *bytes.Buffer
	errBuffer *bytes.Buffer
}

// Stdout returns stdout output of a command run by Exec()
func (res *ExecResult) Stdout() string {
	return res.outBuffer.String()
}

// Stderr returns stderr output of a command run by Exec()
func (res *ExecResult) Stderr() string {
	return res.errBuffer.String()
}

// Combined returns combined stdout and stderr output of a command run by Exec()
func (res *ExecResult) Combined() string {
	return res.outBuffer.String() + res.errBuffer.String()
}

// Exec executes a command inside a container, returning the result
// containing stdout, stderr, and exit code. Note:
//  - this is a synchronous operation;
//  - cmd stdin is closed.
func Exec(ctx context.Context, cli client.APIClient, containerID string, detach bool, cmd ...string) (testbedi.Output, error) {
	// prepare exec
	execConfig := types.ExecConfig{
		User:         "filecoin",
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
		Detach:       detach,
	}
	cresp, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return nil, err
	}
	execID := cresp.ID

	// run it, with stdout/stderr attached
	aresp, err := cli.ContainerExecAttach(ctx, execID, types.ExecStartCheck{})
	if err != nil {
		return nil, err
	}
	defer aresp.Close()

	// read the output
	var outBuf, errBuf bytes.Buffer
	outputDone := make(chan error)

	go func() {
		// StdCopy demultiplexes the stream into two buffers
		_, err = stdcopy.StdCopy(&outBuf, &errBuf, aresp.Reader)
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

	// get the exit code
	iresp, err := cli.ContainerExecInspect(ctx, execID)
	if err != nil {
		return nil, err
	}

	return iptbutil.NewOutput(cmd, outBuf.Bytes(), errBuf.Bytes(), iresp.ExitCode, err), err
}

func (l *Dockerfilecoin) pullImage(cli client.APIClient) error {
	// https://docs.docker.com/develop/sdk/examples/#pull-an-image-with-authentication
	// might need an IPTB config file
	// use the output of `aws --profile filecoin --region us-east-1 ecr --no-include-email get-login`
	authConfig := types.AuthConfig{
		Username:      "username",
		Password:      "password",
		ServerAddress: l.serverAddr,
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		panic(err)
	}
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)

	_, err = cli.ImagePull(context.TODO(), l.imgAddr, types.ImagePullOptions{RegistryAuth: authStr})
	if err != nil {
		return err
	}
	return nil
}

func init() {
	NewNode = func(dir string, attrs map[string]string) (testbedi.Core, error) {
		imagename := "go-filecoin"

		apiaddr, err := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/3453")
		if err != nil {
			return nil, err
		}

		swarmaddr, err := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/6000")
		if err != nil {
			return nil, err
		}

		return &Dockerfilecoin{
			img:        imagename,
			imgAddr:    "657871693752.dkr.ecr.us-east-1.amazonaws.com/filecoin:latest",
			serverAddr: "https://657871693752.dkr.ecr.us-east-1.amazonaws.com",

			dir:       dir,
			apiaddr:   apiaddr,
			swarmaddr: swarmaddr,
		}, nil
	}
}

/** Core Interface **/

// Init runs the node init process.
func (l *Dockerfilecoin) Init(ctx context.Context, args ...string) (testbedi.Output, error) {
	// TODO pass opts here to control a docker daemon on a remote host
	cli, err := client.NewClientWithOpts(client.WithVersion("1.37")) // client requires this version or lower
	if err != nil {
		return nil, err
	}

	/*
		// get the image THIS IS REALLY ANNOYING
		// TODO figure out a better way, e.g. get it locally
		if err := l.pullImage(cli); err != nil {
			return nil, err
		}
	*/

	cmds := []string{"init"}
	cmds = append(cmds, args...)
	// Create the container, first command needs to be daemon, now we have an ID for it
	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Entrypoint: []string{"/usr/local/bin/go-filecoin"},
			User:       "filecoin",
			Image:      "657871693752.dkr.ecr.us-east-1.amazonaws.com/filecoin",
			Cmd:        cmds,
			Tty:        false,
		},
		&container.HostConfig{
			Binds: []string{l.dir + ":" + "/data/filecoin"},
		}, nil, "")
	if err != nil {
		fmt.Println(resp.Warnings)
		return nil, err
	}

	// this runs the init command
	if err := cli.ContainerStart(ctx, string(resp.ID), types.ContainerStartOptions{}); err != nil {
		return nil, err
	}

	return nil, nil
}

// Start starts the node process.
func (l *Dockerfilecoin) Start(ctx context.Context, wait bool, args ...string) (testbedi.Output, error) {
	// TODO check is alive!
	// TODO pass opts here to control a docker daemon on a remote host
	cli, err := client.NewClientWithOpts(client.WithVersion("1.37")) // client requires this version or lower
	if err != nil {
		return nil, err
	}

	/*
		if err := l.pullImage(cli); err != nil {
			return nil, err
		}
	*/

	cmds := []string{"daemon"}
	cmds = append(cmds, args...)
	// Create the container, first command needs to be daemon, now we have an ID for it
	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Entrypoint: []string{"/usr/local/bin/go-filecoin"},
			User:       "filecoin",
			Image:      "657871693752.dkr.ecr.us-east-1.amazonaws.com/filecoin",
			Cmd:        cmds,
			Tty:        false,
		},
		&container.HostConfig{
			Binds: []string{l.dir + ":" + "/data/filecoin"},
		}, nil, "")

	if err != nil {
		fmt.Println(resp.Warnings)
		return nil, err
	}

	idfile := filepath.Join(l.dir, "dockerid")
	err = ioutil.WriteFile(idfile, []byte(resp.ID), 0664)
	if err != nil {
		return nil, err
	}

	// this runs the init command
	if err := cli.ContainerStart(ctx, string(resp.ID), types.ContainerStartOptions{}); err != nil {
		return nil, err
	}

	return nil, nil
}

// Stop stops the node process.
func (l *Dockerfilecoin) Stop(ctx context.Context) error {
	// TODO pass opts here to control a docker daemon on a remote host
	cli, err := client.NewClientWithOpts(client.WithVersion("1.37")) // client requires this version or lower
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

	return nil
}

// RunCmd runs a command in the context of the node.
func (l *Dockerfilecoin) RunCmd(ctx context.Context, stdin io.Reader, args ...string) (testbedi.Output, error) {
	// TODO pass opts here to control a docker daemon on a remote host
	cli, err := client.NewClientWithOpts(client.WithVersion("1.37")) // client requires this version or lower
	if err != nil {
		return nil, err
	}

	idb, err := ioutil.ReadFile(filepath.Join(l.dir, "dockerid"))
	if err != nil {
		return nil, err
	}

	return Exec(ctx, cli, string(idb), false, args...)
}

// Connect connects the node to another testbed node.
func (l *Dockerfilecoin) Connect(ctx context.Context, n testbedi.Core) error {
	panic("NYI")
}

// Shell starts a shell in the context of a node.
func (l *Dockerfilecoin) Shell(ctx context.Context, ns []testbedi.Core) error {
	panic("NYI")
}

// Infof writes an info log.
func (l *Dockerfilecoin) Infof(format string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, "INFO-[%s]\t %s\n", l.Dir(), fmt.Sprintf(format, args...)) // nolint: errcheck
}

// Errorf writes an error log.
func (l *Dockerfilecoin) Errorf(format string, args ...interface{}) {
	nformat := fmt.Sprintf("[ERROR]\t%s %s\n", l, format)
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
	panic("NYI")
}

// Type returns the type of the node.
func (l *Dockerfilecoin) Type() string {
	return PluginName
}

// String implements the stringr interface.
func (l *Dockerfilecoin) String() string {
	panic("NYI")
}

/** Libp2p Interface **/

// PeerID returns the nodes peerID.
func (l *Dockerfilecoin) PeerID() (string, error) {
	panic("NYI")
}

// APIAddr returns the api address of the node.
func (l *Dockerfilecoin) APIAddr() (string, error) {
	panic("NYI")
}

// SwarmAddrs returns the addresses a node is listening on for swarm connections.
func (l *Dockerfilecoin) SwarmAddrs() ([]string, error) {
	panic("NYI")
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

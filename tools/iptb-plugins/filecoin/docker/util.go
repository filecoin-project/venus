package pluginlocalfilecoin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ipfs/go-cid"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/ipfs/iptb/util"
	"github.com/pkg/errors"

	"github.com/ipfs/iptb/testbed/interfaces"
)

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
func Exec(ctx context.Context, cli client.APIClient, containerID string, detach bool, repoDir string, args ...string) (testbedi.Output, error) {
	// prepare exec
	cmd := []string{"sh", "-c"}
	cmd = append(cmd, strings.Join(args, " "))
	execConfig := types.ExecConfig{
		User:         "filecoin",
		AttachStdout: true,
		AttachStderr: true,
		Env:          []string{fmt.Sprintf("FIL_PATH=%s", repoDir)},
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

// GetPeerID returns the nodes peerID by running its `id` command.
// TODO this a temp fix, should read the nodes keystore instead
func (l *Dockerfilecoin) GetPeerID() (cid.Cid, error) {
	// run the id command
	out, err := l.RunCmd(context.TODO(), nil, "id", "--format=<id>")
	if err != nil {
		return cid.Undef, err
	}

	if out.ExitCode() != 0 {
		return cid.Undef, errors.New("Could not get PeerID, non-zero exit code")
	}

	_, err = io.Copy(os.Stdout, out.Stderr())
	if err != nil {
		return cid.Undef, err
	}

	// convert the reader to a string TODO this is annoying
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(out.Stdout())
	if err != nil {
		return cid.Undef, err
	}
	cidStr := strings.TrimSpace(buf.String())

	// decode the parsed string to a cid...maybe
	return cid.Decode(cidStr)
}

// GetClient creates a new docker sdk client
func (l *Dockerfilecoin) GetClient() (*client.Client, error) {
	return client.NewClientWithOpts(client.WithVersion("1.37"), client.WithHost(l.Host))
}

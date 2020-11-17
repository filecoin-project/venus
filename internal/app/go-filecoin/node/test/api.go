package test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/internal/app/go-filecoin/node"
	th "github.com/filecoin-project/venus/internal/pkg/testhelpers"

	commands "github.com/filecoin-project/venus/cmd/go-filecoin"
)

// NodeAPI wraps an in-process Node to provide a command API server and client for testing.
type NodeAPI struct {
	node *node.Node
	tb   testing.TB
}

// NewNodeAPI creates a wrangler for a node.
func NewNodeAPI(node *node.Node, tb testing.TB) *NodeAPI {
	return &NodeAPI{node, tb}
}

// RunNodeAPI creates a new API server and `Run()`s it.
func RunNodeAPI(ctx context.Context, node *node.Node, tb testing.TB) (client *Client, stop func()) {
	api := NewNodeAPI(node, tb)
	return api.Run(ctx)
}

// Node returns the node backing the API.
func (a *NodeAPI) Node() *node.Node {
	return a.node
}

// Run start s a command API server for the node.
// Returns a client proxy and a function to terminate the NodeAPI server.
func (a *NodeAPI) Run(ctx context.Context) (client *Client, stop func()) {
	ready := make(chan interface{})
	terminate := make(chan os.Signal, 1)

	go func() {
		err := commands.RunAPIAndWait(ctx, a.node, a.node.Repo.Config().API, ready, terminate)
		require.NoError(a.tb, err)
	}()
	<-ready

	addr, err := a.node.Repo.APIAddr()
	require.NoError(a.tb, err)
	require.NotEmpty(a.tb, addr, "empty API address")

	return &Client{addr, a.tb}, func() {
		close(terminate)
	}
}

// Client is an in-process client to a command API.
type Client struct {
	address string
	tb      testing.TB
}

// Address returns the address string to which the client sends command RPCs.
func (c *Client) Address() string {
	return c.address
}

func (c *Client) run(ctx context.Context, command ...string) (*th.CmdOutput, int, error) {
	c.tb.Helper()
	args := []string{
		"venus", // A dummy first arg is required, simulating shell invocation.
		fmt.Sprintf("--cmdapiaddr=%s", c.address),
	}
	args = append(args, command...)

	// Create pipes for the client to write stdout and stderr.
	readStdOut, writeStdOut, err := os.Pipe()
	require.NoError(c.tb, err)
	readStdErr, writeStdErr, err := os.Pipe()
	require.NoError(c.tb, err)
	var readStdin *os.File // no stdin needed

	exitCode, err := commands.Run(ctx, args, readStdin, writeStdOut, writeStdErr)
	// Close the output side of the pipes so that ReadAll() on the read ends can complete.
	require.NoError(c.tb, writeStdOut.Close())
	require.NoError(c.tb, writeStdErr.Close())

	out := th.ReadOutput(c.tb, command, readStdOut, readStdErr)

	return out, exitCode, err
}

// Run runs a CLI command and returns its output.
func (c *Client) Run(ctx context.Context, command ...string) *th.CmdOutput {
	out, exitCode, err := c.run(ctx, command...)
	if err != nil {
		out.SetInvocationError(err)
	} else {
		out.SetStatus(exitCode)
	}
	require.NoError(c.tb, err, "client execution error")

	return out
}

// RunSuccess runs a command and asserts that it succeeds (status of zero and logs no errors).
func (c *Client) RunSuccess(ctx context.Context, command ...string) *th.CmdOutput {
	output := c.Run(ctx, command...)
	output.AssertSuccess()
	return output
}

// RunFail runs a command and asserts that it fails with a specified message on stderr.
func (c *Client) RunFail(ctx context.Context, err string, command ...string) *th.CmdOutput {
	output, exitCode, _ := c.run(ctx, command...)
	output.SetStatus(exitCode)
	output.AssertFail(err)
	return output
}

// RunJSON runs a command, asserts success, and parses the response as JSON.
func (c *Client) RunJSON(ctx context.Context, command ...string) map[string]interface{} {
	out := c.RunSuccess(ctx, command...)
	var parsed map[string]interface{}
	require.NoError(c.tb, json.Unmarshal([]byte(out.ReadStdout()), &parsed))
	return parsed
}

// RunMarshaledJSON runs a command, asserts success, and marshals the JSON response.
func (c *Client) RunMarshaledJSON(ctx context.Context, result interface{}, command ...string) {
	out := c.RunSuccess(ctx, command...)
	require.NoError(c.tb, json.Unmarshal([]byte(out.ReadStdout()), &result))
}

// RunSuccessFirstLine executes the given command, asserts success and returns
// the first line of stdout.
func (c *Client) RunSuccessFirstLine(ctx context.Context, args ...string) string {
	return c.RunSuccessLines(ctx, args...)[0]
}

// RunSuccessLines executes the given command, asserts success and returns
// an array of lines of the stdout.
func (c *Client) RunSuccessLines(ctx context.Context, args ...string) []string {
	output := c.RunSuccess(ctx, args...)
	result := output.ReadStdoutTrimNewlines()
	return strings.Split(result, "\n")
}

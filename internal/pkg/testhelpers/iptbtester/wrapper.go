package iptbtester

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"reflect"
	"testing"

	logging "github.com/ipfs/go-log"
	"github.com/ipfs/iptb/testbed/interfaces"

	iptb "github.com/ipfs/iptb/testbed"

	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"

	localplugin "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/local"
)

var log = logging.Logger("iptbtester")

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
}

// NewIPTBTestbed creates an iptb testebed of size `count`. "localfilecoin" or "dockerfilecoin" may be passed
// for `type`.
func NewIPTBTestbed(count int, typ, dir string, attrs map[string]string) (iptb.Testbed, error) {
	log.Infof("Creating IPTB Testbed. count: %d, type: %s, dir: %s", count, typ, dir)
	tbd, err := ioutil.TempDir("", dir)
	if err != nil {
		return nil, err
	}

	testbed := iptb.NewTestbed(tbd)

	nodeSpecs, err := iptb.BuildSpecs(testbed.Dir(), count, typ, attrs)
	if err != nil {
		return nil, err
	}

	if err := iptb.WriteNodeSpecs(testbed.Dir(), nodeSpecs); err != nil {
		return nil, err
	}

	return &testbed, nil
}

// TestNode is a wrapper around an iptb core node interface.
type TestNode struct {
	iptb.Testbed
	testbedi.Core

	T *testing.T
}

// NewTestNodes returns `count` TestNodes, and error is returned if a failure is
// encoundered.
func NewTestNodes(t *testing.T, count int, attrs map[string]string) ([]*TestNode, error) {

	if attrs == nil {
		attrs = make(map[string]string)
	}

	if _, ok := attrs[localplugin.AttrFilecoinBinary]; !ok {
		binaryPath, err := th.GetFilecoinBinary()
		if err != nil {
			return nil, err
		}

		attrs[localplugin.AttrFilecoinBinary] = binaryPath
	}

	// create a testbed
	tb, err := NewIPTBTestbed(count, "localfilecoin", "iptb-testnode", attrs)
	if err != nil {
		return nil, err
	}

	// get the nodes from the testbed
	nodes, err := tb.Nodes()
	if err != nil {
		return nil, err
	}

	// we should fail if and ERROR is written to the daemons stderr

	var testnodes []*TestNode
	for _, n := range nodes {

		tn := &TestNode{
			Testbed: tb,
			Core:    n,
			T:       t,
		}
		testnodes = append(testnodes, tn)
	}
	return testnodes, nil
}

// MustInit inits TestNode, passing `args` to the init command. testing.Fatal is called if initing fails, or exits with
// and exitcode > 0.
func (tn *TestNode) MustInit(ctx context.Context, args ...string) *TestNode {
	tn.T.Logf("TestNode[%s] Init with args: %s", tn.String(), args)
	out, err := tn.Init(ctx, args...)
	// Did IPTB fail to function correctly?
	if err != nil {
		tn.T.Fatalf("IPTB init function failed: %s", err)
	}
	// did the command exit with nonstandard exit code?
	if out.ExitCode() > 0 {
		tn.T.Fatalf("TestNode command: %s, exited with non-zero exitcode: %d", out.Args(), out.ExitCode())
	}
	return tn
}

// MustStart starts TestNode, testing.Fatal is called if starting fails, or exits with
// and exitcode > 0.
func (tn *TestNode) MustStart(ctx context.Context, args ...string) *TestNode {
	tn.T.Logf("TestNode[%s] Start with args: %s", tn.String(), args)
	out, err := tn.Start(ctx, true, args...)
	if err != nil {
		tn.T.Fatalf("IPTB start function failed: %s", err)
	}
	// did the command exit with nonstandard exit code?
	if out.ExitCode() > 0 {
		tn.T.Fatalf("TestNode command: %s, exited with non-zero exitcode: %d", out.Args(), out.ExitCode())
	}
	return tn
}

// MustStop stops TestNode, testing.Fatal is called if stopping fails.
func (tn *TestNode) MustStop(ctx context.Context) {
	tn.T.Logf("TestNode[%s] Stop", tn.String())
	if err := tn.Stop(ctx); err != nil {
		tn.T.Fatalf("IPTB stop function failed: %s", err)
	}
}

// MustConnect will connect TestNode to TestNode `peer`, testing.Fatal will be called
// if connecting fails.
func (tn *TestNode) MustConnect(ctx context.Context, peer *TestNode) {
	tn.T.Logf("TestNode[%s] Connect to peer: %s", tn.String(), peer.String())
	if err := tn.Connect(ctx, peer); err != nil {
		tn.T.Fatalf("IPTB connect function failed: %s", err)
	}
}

// MustRunCmd runs `args` against TestNode.  MustRunCmd returns stderr and stdout after running the command successfully.
// If the command exits with an exitcode > 0, the MustRunCmd will call testing.Fatal and print the error.
func (tn *TestNode) MustRunCmd(ctx context.Context, args ...string) (stdout, stderr string) {
	tn.T.Logf("TestNode[%s] RunCmd with args: %s", tn.String(), args)
	out, err := tn.RunCmd(ctx, nil, args...)
	if err != nil {
		tn.T.Fatalf("IPTB runCmd function failed: %s", err)
	}

	stdo, err := ioutil.ReadAll(out.Stdout())
	if err != nil {
		tn.T.Fatal("Failed to read stdout")
	}
	stdos := string(stdo)
	stde, err := ioutil.ReadAll(out.Stderr())
	if err != nil {
		tn.T.Fatal("Failed to read stderr")
	}
	stdes := string(stde)

	// did the command exit with nonstandard exit code?
	if out.ExitCode() > 0 {
		tn.T.Fatalf("TestNode command: %s, exited with non-zero exitcode: %d\nStdout: %s\nStderr:%s\n", out.Args(), out.ExitCode(), stdos, stdes)
	}

	return stdos, stdes
}

// MustRunCmdJSON runs `args` against TestNode. The '--enc=json' flag is appened to the command specified by `args`,
// the result of the command is marshaled into `expOut`.
func (tn *TestNode) MustRunCmdJSON(ctx context.Context, expOut interface{}, args ...string) {
	args = append(args, "--enc=json")
	out, err := tn.RunCmd(ctx, nil, args...)
	if err != nil {
		tn.T.Fatalf("IPTB runCmd function failed: %s", err)
	}
	// did the command exit with nonstandard exit code?
	if out.ExitCode() > 0 {
		tn.T.Fatalf("TestNode command: %s, exited with non-zero exitcode: %d", out.Args(), out.ExitCode())
	}

	dec := json.NewDecoder(out.Stdout())
	if err := dec.Decode(expOut); err != nil {
		tn.T.Fatalf("Failed to decode output from command: %s to struct: %s", out.Args(), reflect.TypeOf(expOut).Name())
	}
}

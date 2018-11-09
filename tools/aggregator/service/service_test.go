package aggregator

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	crypto "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"

	plugin "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/local"
	iptb "github.com/ipfs/iptb/testbed"
	"github.com/ipfs/iptb/testbed/interfaces"
)

// this ensures the filecoin plugin has been loaded into iptb
func init() {
	_, err := iptb.RegisterPlugin(iptb.IptbPlugin{
		From:       "<builtin>",
		NewNode:    plugin.NewNode,
		PluginName: plugin.PluginName,
		BuiltIn:    true,
	}, false)
	if err != nil {
		panic(err)
	}
}

func mustMakeIPTBNodes(t *testing.T, count int) []testbedi.Core {
	dir, err := ioutil.TempDir("", "aggregator-test")
	if err != nil {
		t.Fatal(err)
	}

	testbed := iptb.NewTestbed(dir)

	specs, err := iptb.BuildSpecs(testbed.Dir(), count, plugin.PluginName, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := iptb.WriteNodeSpecs(testbed.Dir(), specs); err != nil {
		t.Fatal(err)
	}

	nodes, err := testbed.Nodes()
	if err != nil {
		t.Fatal(err)
	}
	return nodes
}

func TestService(t *testing.T) {
	aggCtx := context.Background()
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	agg, err := New(aggCtx, 8889, priv)
	if err != nil {
		t.Fatal(err)
	}
	agg.Run(aggCtx)

	ctx := context.Background()
	nodes := mustMakeIPTBNodes(t, 2)

	// configure all nodes to connect to aggregator
	for _, n := range nodes {
		o, err := n.Init(ctx)
		assertIPTBSuccess(t, o, err)

		o, err = n.Start(ctx, false)
		assertIPTBSuccess(t, o, err)
		defer n.Stop(ctx)

		o, err = n.RunCmd(ctx, nil, "go-filecoin", "config", "heartbeat.beatTarget", fmt.Sprintf(`"%s"`, agg.FullAddress.String()))
		assertIPTBSuccess(t, o, err)
		o, err = n.RunCmd(ctx, nil, "go-filecoin", "config", "heartbeat.beatPeriod", `"1s"`)
		assertIPTBSuccess(t, o, err)
		o, err = n.RunCmd(ctx, nil, "go-filecoin", "config", "heartbeat.reconnectPeriod", `"1s"`)
		assertIPTBSuccess(t, o, err)
	}

	testFail := time.NewTicker(10 * time.Second)
TESTLOOP:
	for {
		select {
		case <-testFail.C:
			t.Fatal("Timeout waiting for tracker to pickup nodes")
		default:
			if len(agg.Tracker.TrackedNodes) == 2 {
				break TESTLOOP
			}
		}
	}
	testFail.Stop()

	nodes[1].Stop(ctx)
	testFail = time.NewTicker(10 * time.Second)
	for {
		select {
		case <-testFail.C:
			t.Fatal("Timeout waiting for tracker to pickup nodes")
		default:
			if len(agg.Tracker.TrackedNodes) == 1 {
				return
			}
		}
	}

}

func assertIPTBSuccess(t *testing.T, out testbedi.Output, err error) {
	if err != nil {
		t.Fatal(err)
	}
	if out.ExitCode() > 0 {
		t.Fatalf("command: %s, exited with non-zero exit code: %d", out.Args(), out.ExitCode())
	}
}

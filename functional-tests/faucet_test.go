package functional

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/commands"
	"github.com/filecoin-project/go-filecoin/testhelpers/iptbtester"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

var faucetBinary = "../tools/faucet/faucet"

func TestFaucetSendFunds(t *testing.T) {
	tf.FunctionalTest(t)

	if _, err := os.Stat(faucetBinary); os.IsNotExist(err) {
		panic("faucet not found, run `go run build/*.go build` to fix")
	}

	ctx := context.Background()

	tns, err := iptbtester.NewTestNodes(t, 2, nil)
	require.NoError(t, err)

	node0 := tns[0]
	node1 := tns[1]

	fundAmount := int64(532)
	blockTime := time.Second * 5

	// Setup first node, note: Testbed.Name() is the directory
	genesis := iptbtester.MustGenerateGenesis(t, 10000, node0.Testbed.Name())

	node0.MustInitWithGenesis(ctx, genesis)
	node0.MustStart(ctx, "--block-time="+blockTime.String())
	defer node0.MustStop(ctx)

	// Import the funded wallet
	iptbtester.MustImportGenesisMiner(node0, genesis)

	// Start mining
	node0.MustRunCmd(ctx, "go-filecoin", "mining", "start")

	// Setup second node

	// Init && Start
	node1.MustInitWithGenesis(ctx, genesis)
	node1.MustStart(ctx, "--block-time="+blockTime.String())
	defer node1.MustStop(ctx)

	// Connect nodes together
	node0.MustConnect(ctx, node1)

	// Setup faucet

	faucetctx, faucetcancel := context.WithCancel(context.Background())
	MustStartFaucet(t, faucetctx, node0, fundAmount, time.Second*30)
	defer faucetcancel()

	// Get address for target node
	var targetAddr commands.AddressLsResult
	node1.MustRunCmdJSON(ctx, &targetAddr, "go-filecoin", "address", "ls")

	// Start Tests

	// Make request for funds
	msgcid := MustSendFundsFaucet(t, "localhost:9797", targetAddr.Addresses[0])

	// Wait around for message to appear
	msgctx, msgcancel := context.WithTimeout(context.Background(), blockTime*3)
	node1.MustRunCmd(msgctx, "go-filecoin", "message", "wait", msgcid)
	msgcancel()

	// Read wallet balance
	var balanceStr string
	node1.MustRunCmdJSON(ctx, &balanceStr, "go-filecoin", "wallet", "balance", targetAddr.Addresses[0])
	balance, err := strconv.ParseInt(balanceStr, 10, 64)
	require.NoError(t, err)

	// Assert funds have arrived
	assert.Equal(t, fundAmount, balance)
}

// MustStartFaucet runs the faucet using the given node. It sends funds from the nodes default wallet
func MustStartFaucet(t *testing.T, ctx context.Context, node *iptbtester.TestNode, faucetVal int64, limiterExpiry time.Duration) { // nolint: golint
	api, err := node.APIAddr()
	if err != nil {
		t.Fatal(err)
	}

	filWallet := ""
	node.MustRunCmdJSON(ctx, &filWallet, "go-filecoin", "config", "wallet.defaultAddress")

	parts := strings.Split(api, "/")
	filAPI := fmt.Sprintf("%s:%s", parts[2], parts[4])

	faucetValStr := strconv.FormatInt(faucetVal, 10)
	limiterExpiryStr := limiterExpiry.String()

	cmd := exec.CommandContext(ctx,
		faucetBinary,
		"-fil-api="+filAPI,
		"-fil-wallet="+filWallet,
		"-limiter-expiry="+limiterExpiryStr,
		"-faucet-val="+faucetValStr,
	)

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
}

// MustMustSendFundsFaucet sends funds to the given wallet address
func MustSendFundsFaucet(t *testing.T, host, target string) string {
	data := url.Values{}
	data.Set("target", target)

	resp, err := http.PostForm("http://"+host+"/tap", data)
	if err != nil {
		t.Fatal(err)
	}

	msgcid := resp.Header.Get("Message-Cid")

	return msgcid
}

package functional

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/drand"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

var faucetBinary = "../tools/faucet/faucet"

func TestFaucetSendFunds(t *testing.T) {
	tf.FunctionalTest(t)
	if _, err := os.Stat(faucetBinary); os.IsNotExist(err) {
		panic("faucet not found, run `go run build/*.go build` to fix")
	}

	ctx := context.Background()
	genTime := int64(1000000000)
	blockTime := 30 * time.Second
	// Set clock ahead so the miner can produce catch-up blocks as quickly as possible.
	fakeClock := clock.NewFake(time.Unix(genTime, 0).Add(1 * time.Hour))

	genCfg := loadGenesisConfig(t, fixtureGenCfg())
	seed := node.MakeChainSeed(t, genCfg)
	chainClock := clock.NewChainClockFromClock(uint64(genTime), blockTime, fakeClock)
	drandImpl := &drand.Fake{
		GenesisTime:   time.Unix(genTime, 0).Add(-1 * blockTime),
		FirstFilecoin: 0,
	}

	nd := makeNode(ctx, t, seed, chainClock, drandImpl)
	api, stopAPI := test.RunNodeAPI(ctx, nd, t)
	defer stopAPI()

	_, owner, err := initNodeGenesisMiner(ctx, t, nd, seed, genCfg.Miners[0].Owner, fixturePresealPath())
	require.NoError(t, err)
	err = nd.Start(ctx)
	require.NoError(t, err)
	defer nd.Stop(ctx)

	// Start faucet server
	faucetctx, faucetcancel := context.WithCancel(context.Background())
	faucetDripFil := uint64(123)
	MustStartFaucet(faucetctx, t, api.Address(), owner, faucetDripFil)
	defer faucetcancel()
	// Wait for faucet to be ready
	time.Sleep(1 * time.Second)

	// Generate an address to receive funds.
	targetKi, err := crypto.NewSecpKeyFromSeed(bytes.NewReader(bytes.Repeat([]byte{1, 2, 3, 4}, 16)))
	require.NoError(t, err)
	targetAddr, err := targetKi.Address()
	require.NoError(t, err)

	// Make request for funds
	msgcid := MustSendFundsFaucet(t, "localhost:9797", targetAddr)
	assert.NotEmpty(t, msgcid)

	// Mine the block containing the message, and another one to evaluate that state.
	for i := 0; i < 2; i++ {
		_, err := nd.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)
	}

	// Check that funds have been transferred
	expectedBalance := types.NewAttoFILFromFIL(faucetDripFil)
	actr, err := nd.PorcelainAPI.ActorGet(ctx, targetAddr)
	require.NoError(t, err)
	assert.True(t, actr.Balance.Equals(expectedBalance))
}

// MustStartFaucet runs the faucet with a provided node API endpoint and wallet from which to source transfers.
func MustStartFaucet(ctx context.Context, t *testing.T, endpoint string, sourceWallet address.Address, faucetVal uint64) {
	parts := strings.Split(endpoint, "/")
	filAPI := fmt.Sprintf("%s:%s", parts[2], parts[4])

	cmd := exec.CommandContext(ctx,
		faucetBinary,
		"-fil-api="+filAPI,
		"-fil-wallet="+sourceWallet.String(),
		"-faucet-val="+strconv.FormatUint(faucetVal, 10),
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
}

// MustMustSendFundsFaucet sends funds to the given wallet address
func MustSendFundsFaucet(t *testing.T, host string, target address.Address) string {
	data := url.Values{}
	data.Set("target", target.String())

	resp, err := http.PostForm("http://"+host+"/tap", data)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		all, _ := ioutil.ReadAll(resp.Body)
		t.Fatalf("faucet request failed: %d %s", resp.StatusCode, all)
	}

	msgcid := resp.Header.Get("Message-Cid")
	return msgcid
}

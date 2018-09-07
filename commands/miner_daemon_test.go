package commands

import (
	"bufio"
	"encoding/json"
	"io/ioutil"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/gengen/util"

	"gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/api/impl"
	"github.com/filecoin-project/go-filecoin/core"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
)

func TestMinerCreate(t *testing.T) {
	t.Skip("FIXME: also depends on mining once")
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)

	testAddr, err := address.NewFromString(th.TestAddress3)
	require.NoError(err)

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		var err error
		var addr address.Address

		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		tf := func(fromAddress address.Address, pid peer.ID, expectSuccess bool) {
			args := []string{"miner", "create"}
			if !fromAddress.Empty() {
				args = append(args, "--from", fromAddress.String())
			}

			if pid.Pretty() != peer.ID("").Pretty() {
				args = append(args, "--peerid", pid.Pretty())
			}

			args = append(args, "1000000", "20")
			if !expectSuccess {
				d.RunFail(impl.ErrCouldNotDefaultFromAddress.Error(), args...)
				return
			}

			var wg sync.WaitGroup

			wg.Add(1)
			go func() {
				miner := d.RunSuccess(args...)
				addr, err = address.NewFromString(strings.Trim(miner.ReadStdout(), "\n"))
				assert.NoError(err)
				assert.NotEqual(addr, address.Address{})
				wg.Done()
			}()

			// ensure mining runs after the command in our goroutine
			d.RunSuccess("mpool --wait-for-count=1")

			d.RunSuccess("mining once")
			wg.Wait()

			// expect address to have been written in config
			config := d.RunSuccess("config mining.minerAddresses")
			assert.Contains(config.ReadStdout(), addr.String())
		}

		// If there's one address, --from can be omitted and we should default
		tf(testAddr, peer.ID(""), true)
		tf(address.Address{}, peer.ID(""), true)

		// If there's more than one address, then --from must be specified
		d.CreateWalletAddr()
		tf(testAddr, peer.ID(""), true)

		// Will accept a peer ID if one is provided
		tf(testAddr, core.RequireRandomPeerID(), true)
	})

	/* TODO: probably delete this test, we have a notion of a 'default' address now in all cases
	t.Run("from address failure", func(t *testing.T) {
		t.Parallel()

		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		tf := func(fromAddress address.Address, pid peer.ID) {
			args := []string{"miner", "create"}

			if pid.Pretty() != peer.ID("").Pretty() {
				args = append(args, "--peerid", pid.Pretty())
			}

			args = append(args, "1000000", "20")
			d.RunFail(impl.ErrCouldNotDefaultFromAddress.Error(), args...)
		}

		// If there's more than one address, then --from must be specified
		d.CreateWalletAddr()
		tf(address.Address{}, peer.ID(""))

	})
	*/

	t.Run("validation failure", func(t *testing.T) {
		t.Parallel()
		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		d.CreateWalletAddr()

		d.RunFail("invalid peer id",
			"miner", "create",
			"--from", testAddr.String(), "--peerid", "flarp", "1000000", "20",
		)
		d.RunFail("invalid from address",
			"miner", "create",
			"--from", "hello", "1000000", "20",
		)
		d.RunFail("invalid pledge",
			"miner", "create",
			"--from", testAddr.String(), "'-123'", "20",
		)
		d.RunFail("invalid pledge",
			"miner", "create",
			"--from", testAddr.String(), "1f", "20",
		)
		d.RunFail("invalid collateral",
			"miner", "create",
			"--from", testAddr.String(), "100", "2f",
		)
	})

	t.Run("creation failure", func(t *testing.T) {
		t.Parallel()
		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			d.RunFail("pledge must be at least",
				"miner", "create",
				"--from", testAddr.String(), "10", "10",
			)
			wg.Done()
		}()

		// ensure mining runs after the command in our goroutine
		d.RunSuccess("mpool --wait-for-count=1")

		d.RunSuccess("mining once")
		wg.Wait()
	})
}

func TestMinerAddAskSuccess(t *testing.T) {
	t.Skip("FIXME: also depends on mining once")
	t.Parallel()
	assert := assert.New(t)

	d := th.NewDaemon(t, th.WalletAddr(th.TestAddress3)).Start()
	defer d.ShutdownSuccess()

	d.CreateWalletAddr()

	var wg sync.WaitGroup
	var minerAddr address.Address

	wg.Add(1)
	go func() {
		miner := d.RunSuccess("miner", "create", "--from", th.TestAddress3, "1000000", "20")
		addr, err := address.NewFromString(strings.Trim(miner.ReadStdout(), "\n"))
		assert.NoError(err)
		assert.NotEqual(addr, address.Address{})
		minerAddr = addr
		wg.Done()
	}()

	// ensure mining runs after the command in our goroutine
	d.RunSuccess("mpool --wait-for-count=1")

	d.RunSuccess("mining once")

	wg.Wait()

	wg.Add(1)
	go func() {
		ask := d.RunSuccess("miner", "add-ask", minerAddr.String(), "2000", "10",
			"--from", th.TestAddress3,
		)
		askCid, err := cid.Parse(strings.Trim(ask.ReadStdout(), "\n"))
		require.NoError(t, err)
		assert.NotNil(askCid)

		wg.Done()
	}()

	wg.Wait()
}

func TestMinerAddAskFail(t *testing.T) {
	t.Skip("FIXME: also depends on mining once")
	t.Parallel()
	assert := assert.New(t)

	d := th.NewDaemon(t, th.CmdTimeout(time.Second*90), th.WalletAddr(th.TestAddress3)).Start()
	defer d.ShutdownSuccess()

	d.CreateWalletAddr()

	var wg sync.WaitGroup
	var minerAddr address.Address

	wg.Add(1)
	go func() {
		miner := d.RunSuccess("miner", "create",
			"--from", th.TestAddress3,
			"--peerid", core.RequireRandomPeerID().Pretty(),
			"1000000", "20",
		)
		addr, err := address.NewFromString(strings.Trim(miner.ReadStdout(), "\n"))
		assert.NoError(err)
		assert.NotEqual(addr, address.Address{})
		minerAddr = addr
		wg.Done()
	}()

	// ensure mining runs after the command in our goroutine
	d.RunSuccess("mpool --wait-for-count=1")

	d.RunSuccess("mining once")

	wg.Wait()

	d.RunFail(
		"invalid from address",
		"miner", "add-ask", minerAddr.String(), "2000", "10",
		"--from", "hello",
	)
	d.RunFail(
		"invalid miner address",
		"miner", "add-ask", "hello", "2000", "10",
		"--from", th.TestAddress3,
	)
	d.RunFail(
		"invalid size",
		"miner", "add-ask", minerAddr.String(), "2f", "10",
		"--from", th.TestAddress3,
	)
	d.RunFail(
		"invalid price",
		"miner", "add-ask", minerAddr.String(), "10", "3f",
		"--from", th.TestAddress3,
	)
}

func TestMinerOwner(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	fi, err := ioutil.TempFile("", "gengentest")
	if err != nil {
		t.Fatal(err)
	}

	if _, err = gengen.GenGenesisCar(testConfig, fi); err != nil {
		t.Fatal(err)
	}

	_ = fi.Close()

	d := th.NewDaemon(t, th.GenesisFile(fi.Name())).Start()
	defer d.ShutdownSuccess()

	actorLsOutput := d.RunSuccess("actor", "ls")

	scanner := bufio.NewScanner(strings.NewReader(actorLsOutput.ReadStdout()))
	var addressStruct struct{ Address string }

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "MinerActor") {
			json.Unmarshal([]byte(line), &addressStruct)
			break
		}
	}

	ownerOutput := d.RunSuccess("miner", "owner", addressStruct.Address)

	_, err = address.NewFromString(ownerOutput.ReadStdoutTrimNewlines())

	assert.NoError(err)
}

var testConfig = &gengen.GenesisCfg{
	Keys: []string{"bob", "hank", "steve", "laura"},
	PreAlloc: map[string]string{
		"bob":  "10",
		"hank": "50",
	},
	Miners: []gengen.Miner{
		{
			Owner: "bob",
			Power: 5000,
		},
		{
			Owner: "laura",
			Power: 1000,
		},
	},
}

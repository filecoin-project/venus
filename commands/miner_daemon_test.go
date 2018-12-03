package commands

import (
	"bufio"
	"encoding/json"
	"io/ioutil"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/gengen/util"

	"gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/fixtures"

	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
)

func TestMinerHelp(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	t.Run("--help shows general miner help", func(t *testing.T) {
		t.Parallel()

		expected := []string{
			"miner add-ask <miner> <price> <expiry> - Add an ask to the storage market",
			"miner create <pledge> <collateral>     - Create a new file miner with <pledge> 1GB sectors and <collateral> FIL",
			"miner owner <miner>                    - Show the actor address of <miner>",
			"miner pledge <miner>                   - View number of pledged 1GB sectors for <miner>",
			"miner power <miner>                    - Get the power of a miner versus the total storage market power",
			"miner update-peerid <address> <peerid> - Change the libp2p identity that a miner is operating",
		}

		result := runHelpSuccess(t, "miner", "--help")
		for _, elem := range expected {
			assert.Contains(result, elem)
		}
	})

	t.Run("pledge --help shows pledge help", func(t *testing.T) {
		t.Parallel()
		result := runHelpSuccess(t, "miner", "pledge", "--help")
		assert.Contains(result, "Shows the number of pledged 1GB sectors for the given miner address")
	})

	t.Run("update-peerid --help shows update-peerid help", func(t *testing.T) {
		t.Parallel()
		result := runHelpSuccess(t, "miner", "update-peerid", "--help")
		assert.Contains(result, "Issues a new message to the network to update the miner's libp2p identity.")
	})
	t.Run("add-ask --help shows add-ask help", func(t *testing.T) {
		t.Parallel()
		result := runHelpSuccess(t, "miner", "add-ask", "--help")
		assert.Contains(result, " Add an ask to the storage market")
	})

	t.Run("owner --help shows owner help", func(t *testing.T) {
		t.Parallel()
		result := runHelpSuccess(t, "miner", "owner", "--help")
		assert.Contains(result, "Given <miner> miner address, output the address of the actor that owns the miner.")
	})

	t.Run("power --help shows power help", func(t *testing.T) {
		t.Parallel()
		result := runHelpSuccess(t, "miner", "power", "--help")
		expected := []string{
			"Check the current power of a given miner and total power of the storage market.",
			"Values will be output as a ratio where the first number is the miner power and second is the total market power.",
		}
		for _, elem := range expected {
			assert.Contains(result, elem)
		}
	})

	t.Run("create --help shows create help", func(t *testing.T) {
		t.Parallel()

		expected := []string{
			"Issues a new message to the network to create the miner, then waits for the",
			"message to be mined as this is required to return the address of the new miner",
			"Collateral must be greater than 0.001 FIL per pledged sector.",
		}

		result := runHelpSuccess(t, "miner", "create", "--help")
		for _, elem := range expected {
			assert.Contains(result, elem)
		}
	})

}

func runHelpSuccess(t *testing.T, args ...string) string {
	fi, err := ioutil.TempFile("", "gengentest")
	if err != nil {
		t.Fatal(err)
	}

	if _, err = gengen.GenGenesisCar(testConfig, fi, 0); err != nil {
		t.Fatal(err)
	}

	_ = fi.Close()
	d := th.NewDaemon(t, th.GenesisFile(fi.Name())).Start()
	defer d.ShutdownSuccess()

	op := d.RunSuccess(args...)
	return op.ReadStdoutTrimNewlines()
}

func TestMinerPledge(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	fi, err := ioutil.TempFile("", "gengentest")
	if err != nil {
		t.Fatal(err)
	}

	if _, err = gengen.GenGenesisCar(testConfig, fi, 0); err != nil {
		t.Fatal(err)
	}

	_ = fi.Close()

	t.Run("shows error with no miner address", func(t *testing.T) {
		t.Parallel()
		d := th.NewDaemon(t, th.GenesisFile(fi.Name())).Start()
		defer d.ShutdownSuccess()

		d.RunFail("argument \"miner\" is required", "miner", "pledge")
	})

	t.Run("shows pledge amount for miner", func(t *testing.T) {
		t.Parallel()
		d := th.NewDaemon(t, th.GenesisFile(fi.Name())).Start()
		defer d.ShutdownSuccess()

		// get Miner address
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

		op1 := d.RunSuccess("miner", "pledge", addressStruct.Address)
		result1 := op1.ReadStdoutTrimNewlines()
		assert.Contains(result1, "10000")
	})
}

func TestMinerCreate(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)

	testAddr, err := address.NewFromString(fixtures.TestAddresses[2])
	require.NoError(err)

	t.Run("create --help includes pledge text", func(t *testing.T) {
		t.Parallel()
		d := th.NewDaemon(t, th.WithMiner(fixtures.TestMiners[0])).Start()
		defer d.ShutdownSuccess()

		op1 := d.RunSuccess("miner", "create", "--help")
		result1 := op1.ReadStdoutTrimNewlines()
		assert.Contains(result1, "<pledge>     - the size of the pledge (in 1GB sectors) for the miner")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		var err error
		var addr address.Address

		tf := func(fromAddress address.Address, pid peer.ID) {
			d1 := th.NewDaemon(t, th.WithMiner(fixtures.TestMiners[0]), th.KeyFile(fixtures.KeyFilePaths()[2])).Start()
			defer d1.ShutdownSuccess()

			d := th.NewDaemon(t, th.KeyFile(fixtures.KeyFilePaths()[2])).Start()
			defer d.ShutdownSuccess()

			d1.ConnectSuccess(d)

			args := []string{"miner", "create", "--from", fromAddress.String()}

			if pid.Pretty() != peer.ID("").Pretty() {
				args = append(args, "--peerid", pid.Pretty())
			}

			args = append(args, "1000000", storagemarket.MinimumCollateral(big.NewInt(1000000)).String())

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
			d1.MineAndPropagate(time.Second, d)
			wg.Wait()

			// expect address to have been written in config
			config := d.RunSuccess("config mining.minerAddress")
			assert.Contains(config.ReadStdout(), addr.String())
		}

		tf(testAddr, peer.ID(""))

		// Will accept a peer ID if one is provided
		tf(testAddr, th.RequireRandomPeerID())
	})

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

	t.Run("insufficient pledge", func(t *testing.T) {
		t.Parallel()
		d1 := th.NewDaemon(t, th.WithMiner(fixtures.TestMiners[0]), th.KeyFile(fixtures.KeyFilePaths()[2])).Start()
		defer d1.ShutdownSuccess()

		d := th.NewDaemon(t, th.KeyFile(fixtures.KeyFilePaths()[2])).Start()
		defer d.ShutdownSuccess()

		d1.ConnectSuccess(d)

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			d.RunFail("pledge must be at least",
				"miner", "create",
				"--from", testAddr.String(), "1", "10",
			)
			wg.Done()
		}()

		// ensure mining runs after the command in our goroutine
		d1.MineAndPropagate(time.Second, d)
		wg.Wait()
	})

	t.Run("insufficient funds", func(t *testing.T) {
		t.Parallel()
		d1 := th.NewDaemon(t, th.WithMiner(fixtures.TestMiners[0]), th.KeyFile(fixtures.KeyFilePaths()[2])).Start()
		defer d1.ShutdownSuccess()

		d := th.NewDaemon(t, th.KeyFile(fixtures.KeyFilePaths()[2])).Start()
		defer d.ShutdownSuccess()

		d1.ConnectSuccess(d)

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			d.RunFail("not enough balance",
				"miner", "create",
				"--from", testAddr.String(), "10", "10000000000000000",
			)
			wg.Done()
		}()

		// ensure mining runs after the command in our goroutine
		d1.MineAndPropagate(time.Second, d)
		wg.Wait()
	})
}

func TestMinerAddAskSuccess(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	d1 := th.NewDaemon(t, th.WithMiner(fixtures.TestMiners[0]), th.KeyFile(fixtures.KeyFilePaths()[2])).Start()
	defer d1.ShutdownSuccess()
	d := th.NewDaemon(t, th.KeyFile(fixtures.KeyFilePaths()[2])).Start()
	defer d.ShutdownSuccess()
	d1.ConnectSuccess(d)
	var wg sync.WaitGroup
	var minerAddr address.Address
	wg.Add(1)
	go func() {
		miner := d.RunSuccess("miner", "create", "--from", fixtures.TestAddresses[2], "100", "20")
		addr, err := address.NewFromString(strings.Trim(miner.ReadStdout(), "\n"))
		assert.NoError(err)
		assert.NotEqual(addr, address.Address{})
		minerAddr = addr
		wg.Done()
	}()
	// ensure mining runs after the command in our goroutine
	d1.MineAndPropagate(time.Second, d)
	wg.Wait()
	wg.Add(1)
	go func() {
		ask := d.RunSuccess("miner", "add-ask", minerAddr.String(), "20", "10",
			"--from", fixtures.TestAddresses[2],
		)
		askCid, err := cid.Parse(strings.Trim(ask.ReadStdout(), "\n"))
		require.NoError(t, err)
		assert.NotNil(askCid)
		wg.Done()
	}()
	wg.Wait()
}
func TestMinerAddAskFail(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	d1 := th.NewDaemon(t, th.WithMiner(fixtures.TestMiners[0]), th.KeyFile(fixtures.KeyFilePaths()[2])).Start()
	defer d1.ShutdownSuccess()
	d := th.NewDaemon(t, th.CmdTimeout(time.Second*90), th.KeyFile(fixtures.KeyFilePaths()[2])).Start()
	defer d.ShutdownSuccess()
	d1.ConnectSuccess(d)
	var wg sync.WaitGroup
	var minerAddr address.Address
	wg.Add(1)
	go func() {
		miner := d.RunSuccess("miner", "create",
			"--from", fixtures.TestAddresses[2],
			"--peerid", th.RequireRandomPeerID().Pretty(),
			"100", "20",
		)
		addr, err := address.NewFromString(strings.Trim(miner.ReadStdout(), "\n"))
		assert.NoError(err)
		assert.NotEqual(addr, address.Address{})
		minerAddr = addr
		wg.Done()
	}()
	// ensure mining runs after the command in our goroutine
	d1.MineAndPropagate(time.Second, d)
	wg.Wait()
	d.RunFail(
		"invalid from address",
		"miner", "add-ask", minerAddr.String(), "20", "10",
		"--from", "hello",
	)
	d.RunFail(
		"invalid miner address",
		"miner", "add-ask", "hello", "20", "10",
		"--from", fixtures.TestAddresses[2],
	)
	d.RunFail(
		"invalid price",
		"miner", "add-ask", minerAddr.String(), "2f", "10",
		"--from", fixtures.TestAddresses[2],
	)
	d.RunFail(
		"expiry must be a valid integer",
		"miner", "add-ask", minerAddr.String(), "10", "3f",
		"--from", fixtures.TestAddresses[2],
	)
}

func TestMinerOwner(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	fi, err := ioutil.TempFile("", "gengentest")
	if err != nil {
		t.Fatal(err)
	}

	if _, err = gengen.GenGenesisCar(testConfig, fi, 0); err != nil {
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

func TestMinerPower(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	fi, err := ioutil.TempFile("", "gengentest")
	if err != nil {
		t.Fatal(err)
	}

	if _, err = gengen.GenGenesisCar(testConfig, fi, 0); err != nil {
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

	powerOutput := d.RunSuccess("miner", "power", addressStruct.Address)

	power := powerOutput.ReadStdoutTrimNewlines()

	assert.NoError(err)
	assert.Equal("3 / 6", power)
}

var testConfig = &gengen.GenesisCfg{
	Keys: 4,
	PreAlloc: []string{
		"10",
		"50",
	},
	Miners: []gengen.Miner{
		{
			Owner: 0,
			Power: 3,
		},
		{
			Owner: 1,
			Power: 3,
		},
	},
}

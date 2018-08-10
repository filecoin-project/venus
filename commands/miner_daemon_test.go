package commands

import (
	"strings"
	"sync"
	"testing"
	"time"

	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/api/impl"
	"github.com/filecoin-project/go-filecoin/core"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testfiles"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestMinerCreate(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)

	wtf := tf.WalletFilePath()
	testAddr, err := types.NewAddressFromString(testAddress3)
	require.NoError(err)

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		var err error
		var addr types.Address

		d := NewDaemon(t, WalletFile(wtf), WalletAddr(testAddr.String())).Start()
		defer d.ShutdownSuccess()

		tf := func(fromAddress types.Address, pid peer.ID, expectSuccess bool) {
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
				addr, err = types.NewAddressFromString(strings.Trim(miner.ReadStdout(), "\n"))
				assert.NoError(err)
				assert.NotEqual(addr, types.Address{})
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
		tf(types.Address{}, peer.ID(""), true)

		// If there's more than one address, then --from must be specified
		d.CreateWalletAddr()
		tf(testAddr, peer.ID(""), true)

		// Will accept a peer ID if one is provided
		tf(testAddr, core.RequireRandomPeerID(), true)
	})

	t.Run("from address failure", func(t *testing.T) {
		t.Parallel()

		d := NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		tf := func(fromAddress types.Address, pid peer.ID) {
			args := []string{"miner", "create"}

			if pid.Pretty() != peer.ID("").Pretty() {
				args = append(args, "--peerid", pid.Pretty())
			}

			args = append(args, "1000000", "20")
			d.RunFail(impl.ErrCouldNotDefaultFromAddress.Error(), args...)
		}

		// If there's more than one address, then --from must be specified
		d.CreateWalletAddr()
		tf(types.Address{}, peer.ID(""))

	})

	t.Run("validation failure", func(t *testing.T) {
		t.Parallel()
		d := NewDaemon(t, WalletFile(wtf), WalletAddr(testAddr.String())).Start()
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
		d := NewDaemon(t, WalletFile(wtf), WalletAddr(testAddr.String())).Start()
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
	t.Parallel()
	assert := assert.New(t)

	wtf := tf.WalletFilePath()
	d := NewDaemon(t, WalletFile(wtf), WalletAddr(testAddress3)).Start()
	defer d.ShutdownSuccess()

	d.CreateWalletAddr()

	var wg sync.WaitGroup
	var minerAddr types.Address

	wg.Add(1)
	go func() {
		miner := d.RunSuccess("miner", "create", "--from", testAddress3, "1000000", "20")
		addr, err := types.NewAddressFromString(strings.Trim(miner.ReadStdout(), "\n"))
		assert.NoError(err)
		assert.NotEqual(addr, types.Address{})
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
			"--from", testAddress3,
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

	wtf := tf.WalletFilePath()
	d := NewDaemon(t, CmdTimeout(time.Second*90), WalletFile(wtf), WalletAddr(testAddress3)).Start()
	defer d.ShutdownSuccess()

	d.CreateWalletAddr()

	var wg sync.WaitGroup
	var minerAddr types.Address

	wg.Add(1)
	go func() {
		miner := d.RunSuccess("miner", "create",
			"--from", testAddress3,
			"--peerid", core.RequireRandomPeerID().Pretty(),
			"1000000", "20",
		)
		addr, err := types.NewAddressFromString(strings.Trim(miner.ReadStdout(), "\n"))
		assert.NoError(err)
		assert.NotEqual(addr, types.Address{})
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
		"--from", testAddress3,
	)
	d.RunFail(
		"invalid size",
		"miner", "add-ask", minerAddr.String(), "2f", "10",
		"--from", testAddress3,
	)
	d.RunFail(
		"invalid price",
		"miner", "add-ask", minerAddr.String(), "10", "3f",
		"--from", testAddress3,
	)
}

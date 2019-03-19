package porcelain_test

import (
	"context"
	"testing"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/plumbing/cfg"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

type wbTestPlumbing struct {
	balance *types.AttoFIL
}

type wdaTestPlumbing struct {
	config *cfg.Config
	wallet *wallet.Wallet
}

func newWdaTestPlumbing(require *require.Assertions) *wdaTestPlumbing {
	repo := repo.NewInMemoryRepo()
	backend, err := wallet.NewDSBackend(repo.WalletDatastore())
	require.NoError(err)
	return &wdaTestPlumbing{
		config: cfg.NewConfig(repo),
		wallet: wallet.New(backend),
	}
}

func (wbtp *wbTestPlumbing) ActorGet(ctx context.Context, addr address.Address) (*actor.Actor, error) {
	testActor := actor.NewActor(cid.Undef, wbtp.balance)
	return testActor, nil
}

func (wdatp *wdaTestPlumbing) ConfigGet(dottedPath string) (interface{}, error) {
	return wdatp.config.Get(dottedPath)
}

func (wdatp *wdaTestPlumbing) ConfigSet(dottedPath string, paramJSON string) error {
	return wdatp.config.Set(dottedPath, paramJSON)
}

func (wdatp *wdaTestPlumbing) WalletAddresses() []address.Address {
	return wdatp.wallet.Addresses()
}

func (wdatp *wdaTestPlumbing) WalletNewAddress() (address.Address, error) {
	return wallet.NewAddress(wdatp.wallet)
}

func TestWalletBalance(t *testing.T) {
	t.Run("Returns the correct value for wallet balance", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)
		ctx := context.Background()

		expectedBalance := types.NewAttoFILFromFIL(20)
		plumbing := &wbTestPlumbing{
			balance: expectedBalance,
		}
		balance, err := porcelain.WalletBalance(ctx, plumbing, address.Undef)
		require.NoError(err)

		assert.Equal(expectedBalance, balance)
	})
}

func TestWalletDefaultAddress(t *testing.T) {
	t.Parallel()

	t.Run("it returns the configured wallet default if it exists", func(t *testing.T) {
		require := require.New(t)

		wdatp := newWdaTestPlumbing(require)

		addr, err := wdatp.WalletNewAddress()
		require.NoError(err)
		wdatp.ConfigSet("wallet.defaultAddress", addr.String())

		_, err = porcelain.WalletDefaultAddress(wdatp)
		require.NoError(err)
	})

	t.Run("default is consistent if none configured", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		wdatp := newWdaTestPlumbing(require)

		addresses := []address.Address{}
		for i := 0; i < 10; i++ {
			a, err := wdatp.WalletNewAddress()
			require.NoError(err)
			addresses = append(addresses, a)
		}

		expected, err := porcelain.WalletDefaultAddress(wdatp)
		require.NoError(err)
		require.True(isInList(expected, addresses))
		for i := 0; i < 30; i++ {
			got, err := porcelain.WalletDefaultAddress(wdatp)
			require.NoError(err)
			assert.Equal(expected, got)
		}
	})
}

func isInList(needle address.Address, haystack []address.Address) bool {
	for _, a := range haystack {
		if a == needle {
			return true
		}
	}
	return false
}

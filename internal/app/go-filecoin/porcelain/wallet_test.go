package porcelain_test

import (
	"context"
	"testing"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cfg"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type wbTestPlumbing struct {
	balance types.AttoFIL
}

type wdaTestPlumbing struct {
	config *cfg.Config
	wallet *wallet.Wallet
}

func newWdaTestPlumbing(t *testing.T) *wdaTestPlumbing {
	repo := repo.NewInMemoryRepo()
	backend, err := wallet.NewDSBackend(repo.WalletDatastore())
	require.NoError(t, err)
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
	return wallet.NewAddress(wdatp.wallet, address.SECP256K1)
}

func TestWalletBalance(t *testing.T) {
	tf.UnitTest(t)

	t.Run("Returns the correct value for wallet balance", func(t *testing.T) {
		ctx := context.Background()

		expectedBalance := types.NewAttoFILFromFIL(20)
		plumbing := &wbTestPlumbing{
			balance: expectedBalance,
		}
		balance, err := porcelain.WalletBalance(ctx, plumbing, address.Undef)
		require.NoError(t, err)

		assert.Equal(t, expectedBalance, balance)
	})
}

func TestWalletDefaultAddress(t *testing.T) {
	tf.UnitTest(t)

	t.Run("it returns the configured wallet default if it exists", func(t *testing.T) {
		wdatp := newWdaTestPlumbing(t)

		addr, err := wdatp.WalletNewAddress()
		require.NoError(t, err)
		err = wdatp.ConfigSet("wallet.defaultAddress", addr.String())
		require.NoError(t, err)

		_, err = porcelain.WalletDefaultAddress(wdatp)
		require.NoError(t, err)
	})

	t.Run("default is consistent if none configured", func(t *testing.T) {
		wdatp := newWdaTestPlumbing(t)

		addresses := []address.Address{}
		for i := 0; i < 10; i++ {
			a, err := wdatp.WalletNewAddress()
			require.NoError(t, err)
			addresses = append(addresses, a)
		}

		expected, err := porcelain.WalletDefaultAddress(wdatp)
		require.NoError(t, err)
		require.True(t, isInList(expected, addresses))
		for i := 0; i < 30; i++ {
			got, err := porcelain.WalletDefaultAddress(wdatp)
			require.NoError(t, err)
			assert.Equal(t, expected, got)
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

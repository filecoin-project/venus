package porcelain_test

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/plumbing/cfg"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/wallet"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

type fakeGetAndMaybeSetDefaultSenderAddressPlumbing struct {
	config *cfg.Config
	wallet *wallet.Wallet
}

func newFakeGetAndMaybeSetDefaultSenderAddressPlumbing(require *require.Assertions) *fakeGetAndMaybeSetDefaultSenderAddressPlumbing {
	repo := repo.NewInMemoryRepo()
	backend, err := wallet.NewDSBackend(repo.WalletDatastore())
	require.NoError(err)
	return &fakeGetAndMaybeSetDefaultSenderAddressPlumbing{
		config: cfg.NewConfig(repo),
		wallet: wallet.New(backend),
	}
}

func (fgamsdsap *fakeGetAndMaybeSetDefaultSenderAddressPlumbing) ConfigGet(dottedPath string) (interface{}, error) {
	return fgamsdsap.config.Get(dottedPath)
}

func (fgamsdsap *fakeGetAndMaybeSetDefaultSenderAddressPlumbing) ConfigSet(dottedPath string, paramJSON string) error {
	return fgamsdsap.config.Set(dottedPath, paramJSON)
}

func (fgamsdsap *fakeGetAndMaybeSetDefaultSenderAddressPlumbing) WalletAddresses() []address.Address {
	return fgamsdsap.wallet.Addresses()
}

func (fgamsdsap *fakeGetAndMaybeSetDefaultSenderAddressPlumbing) WalletNewAddress() (address.Address, error) {
	return wallet.NewAddress(fgamsdsap.wallet)
}

func TestGetAndMaybeSetDefaultSenderAddress(t *testing.T) {
	t.Parallel()

	t.Run("it returns the configured wallet default if it exists", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		fp := newFakeGetAndMaybeSetDefaultSenderAddressPlumbing(require)

		addrA, err := fp.WalletNewAddress()
		require.NoError(err)
		assert.NoError(fp.ConfigSet("wallet.defaultAddress", addrA.String()))

		addrB, err := porcelain.GetAndMaybeSetDefaultSenderAddress(fp)
		require.NoError(err)
		assert.Equal(addrA.String(), addrB.String())
	})

	t.Run("default is consistent if none configured", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		fp := newFakeGetAndMaybeSetDefaultSenderAddressPlumbing(require)

		addresses := []address.Address{}
		for i := 0; i < 10; i++ {
			a, err := fp.WalletNewAddress()
			require.NoError(err)
			addresses = append(addresses, a)
		}

		expected, err := porcelain.GetAndMaybeSetDefaultSenderAddress(fp)
		require.NoError(err)
		require.True(isInList(expected, addresses))
		for i := 0; i < 30; i++ {
			got, err := porcelain.GetAndMaybeSetDefaultSenderAddress(fp)
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

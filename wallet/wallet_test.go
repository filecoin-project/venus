package wallet

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/types"
)

func TestWalletSimple(t *testing.T) {
	assert := assert.New(t)

	w := New()
	count := 10
	addrs := make([]types.Address, count)

	for i := 0; i < count; i++ {
		addrs[i] = w.NewAddress()
	}

	assert.ElementsMatch(addrs, w.GetAddresses())
}

func TestWalletDefaultAddress(t *testing.T) {
	assert := assert.New(t)

	w := New()

	addr, err := w.GetDefaultAddress()
	assert.EqualError(err, "no default address in local wallet")
	assert.Equal(addr, types.Address{})

	addrNew := w.NewAddress()
	addr, err = w.GetDefaultAddress()
	assert.NoError(err)
	assert.Equal(addr, addrNew)
}

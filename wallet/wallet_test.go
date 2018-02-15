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

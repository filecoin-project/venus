package types

import (
	"testing"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"

	"github.com/stretchr/testify/assert"
)

func TestAddrSetRoundtrip(t *testing.T) {
	assert := assert.New(t)
	addrGetter := NewAddressForTestGetter()

	addrs := make([]Address, 10)
	for i := range addrs {
		addrs[i] = addrGetter()
	}

	set := AddrSet{}

	for _, addr := range addrs {
		set[addr] = struct{}{}
	}

	bytes, err := cbor.DumpObject(set)
	assert.NoError(err)

	var setBack AddrSet
	assert.NoError(cbor.DecodeInto(bytes, &setBack))

	assert.Equal(len(addrs), len(setBack))
	for _, addr := range addrs {
		_, ok := setBack[addr]
		assert.True(ok)
	}
}

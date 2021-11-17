package testutil

import (
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
)

func TestDefaultCid(t *testing.T) {
	var c cid.Cid
	Provide(t, &c)
	assert.NotEqual(t, cid.Undef, c)
}

func TestDefaultCidSlice(t *testing.T) {
	cs := make([]cid.Cid, 16)
	Provide(t, &cs)
	for ci := range cs {
		assert.NotEqual(t, cid.Undef, cs[ci])
	}
}

func TestDefaultAddresses(t *testing.T) {
	addrs := make([]address.Address, 256)
	protos := map[address.Protocol]struct{}{}
	Provide(t, &addrs)
	for i := range addrs {
		protos[addrs[i].Protocol()] = struct{}{}
	}

	assert.True(t, len(protos) == 4)
}

func TestDefaultIDAddresses(t *testing.T) {
	addrs := make([]address.Address, 256)
	protos := map[address.Protocol]struct{}{}
	Provide(t, &addrs, IDAddressProvider())
	for i := range addrs {
		protos[addrs[i].Protocol()] = struct{}{}
	}

	assert.True(t, len(protos) == 1)
}

func TestDefaultBigs(t *testing.T) {
	bigs := make([]big.Int, 256)
	Provide(t, &bigs)
	hasPositive := false
	hasNegative := false
	zero := big.Zero()
	for bi := range bigs {
		assert.NotNil(t, bigs[bi].Int)
		hasPositive = hasPositive || bigs[bi].GreaterThan(zero)
		hasNegative = hasNegative || bigs[bi].LessThan(zero)
	}

	assert.True(t, hasPositive)
	assert.True(t, hasNegative)
}

func TestDefaultSigTypes(t *testing.T) {
	sigtyps := make([]crypto.SigType, 256)
	Provide(t, &sigtyps)
	typs := map[crypto.SigType]struct{}{}
	for i := range sigtyps {
		typs[sigtyps[i]] = struct{}{}
	}

	assert.True(t, len(typs) == 2)
}

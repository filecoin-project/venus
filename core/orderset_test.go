package core

import (
	"math/big"
	"testing"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"

	"github.com/stretchr/testify/assert"
)

func TestAskSetMarshaling(t *testing.T) {
	assert := assert.New(t)
	as := make(AskSet)
	ask4 := &Ask{ID: 4, Owner: "foo", Price: big.NewInt(19), Size: big.NewInt(105)}
	ask5 := &Ask{ID: 5, Owner: "bar", Price: big.NewInt(909), Size: big.NewInt(435)}
	as[4] = ask4
	as[5] = ask5

	data, err := cbor.DumpObject(as)
	assert.NoError(err)

	var asout AskSet
	assert.NoError(cbor.DecodeInto(data, &asout))
	assert.Len(asout, 2)
	ask4out, ok := as[4]
	assert.True(ok)
	assert.Equal(ask4, ask4out)
	ask5out, ok := as[5]
	assert.True(ok)
	assert.Equal(ask5, ask5out)
}

func TestBidSetMarshaling(t *testing.T) {
	assert := assert.New(t)
	bs := make(BidSet)
	bid4 := &Bid{ID: 4, Owner: "foo", Price: big.NewInt(19), Size: big.NewInt(105)}
	bid5 := &Bid{ID: 5, Owner: "bar", Price: big.NewInt(909), Size: big.NewInt(435)}
	bs[4] = bid4
	bs[5] = bid5

	data, err := cbor.DumpObject(bs)
	assert.NoError(err)

	var bsout BidSet
	assert.NoError(cbor.DecodeInto(data, &bsout))
	assert.Len(bsout, 2)
	bid4out, ok := bs[4]
	assert.True(ok)
	assert.Equal(bid4, bid4out)
	bid5out, ok := bs[5]
	assert.True(ok)
	assert.Equal(bid5, bid5out)
}

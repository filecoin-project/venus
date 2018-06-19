package storagemarket

import (
	"testing"

	cbor "gx/ipfs/QmRiRJhn427YVuufBEHofLreKWNw7P7BWNq86Sb9kzqdbd/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
)

func TestAskSetMarshaling(t *testing.T) {
	assert := assert.New(t)
	addrGetter := types.NewAddressForTestGetter()

	as := make(AskSet)
	ask4 := &Ask{ID: 4, Owner: addrGetter(), Price: types.NewAttoFILFromFIL(19), Size: types.NewBytesAmount(105)}
	ask5 := &Ask{ID: 5, Owner: addrGetter(), Price: types.NewAttoFILFromFIL(909), Size: types.NewBytesAmount(435)}
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
	addrGetter := types.NewAddressForTestGetter()

	bid4 := &Bid{ID: 4, Owner: addrGetter(), Price: types.NewAttoFILFromFIL(19), Size: types.NewBytesAmount(105)}
	bid5 := &Bid{ID: 5, Owner: addrGetter(), Price: types.NewAttoFILFromFIL(909), Size: types.NewBytesAmount(435)}
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

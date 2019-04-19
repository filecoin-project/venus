package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
)

func TestEncodeThenDecode(t *testing.T){
	require := require.New(t)
	assert := assert.New(t)

	addrGetter := address.NewForTestGetter()
	addr1 := addrGetter()
	addr2 := addrGetter()

	paymentVoucher := &PaymentVoucher{
		Channel:   *NewChannelID(5),
		Payer:     addr1,
		Target:    addr2,
		Amount:    *NewAttoFILFromFIL(100),
		ValidAt:   *NewBlockHeight(25),
	}
	
	rawPaymentVoucher,err:=paymentVoucher.Encode()
	require.NoError(err)

	DecodedPaymentVoucher,err:=DecodeVoucher(rawPaymentVoucher)
	require.NoError(err)

	assert.Equal((*paymentVoucher).Channel,DecodedPaymentVoucher.Channel)
	assert.Equal((*paymentVoucher).Payer,DecodedPaymentVoucher.Payer)
	assert.Equal((*paymentVoucher).Target,DecodedPaymentVoucher.Target)
	assert.Equal((*paymentVoucher).Amount,DecodedPaymentVoucher.Amount)
	assert.Equal((*paymentVoucher).ValidAt,DecodedPaymentVoucher.ValidAt)
}
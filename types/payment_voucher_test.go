package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
)

func TestPaymentVoucherEncodingRoundTrip(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	addrGetter := address.NewForTestGetter()
	addr1 := addrGetter()
	addr2 := addrGetter()

	paymentVoucher := &PaymentVoucher{
		Channel: *NewChannelID(5),
		Payer:   addr1,
		Target:  addr2,
		Amount:  *NewAttoFILFromFIL(100),
		ValidAt: *NewBlockHeight(25),
	}

	rawPaymentVoucher, err := paymentVoucher.Encode()
	require.NoError(err)
	decodedPaymentVoucher, err := DecodeVoucher(rawPaymentVoucher)
	require.NoError(err)

	assert.Equal((*paymentVoucher).Channel, decodedPaymentVoucher.Channel)
	assert.Equal((*paymentVoucher).Payer, decodedPaymentVoucher.Payer)
	assert.Equal((*paymentVoucher).Target, decodedPaymentVoucher.Target)
	assert.Equal((*paymentVoucher).Amount, decodedPaymentVoucher.Amount)
	assert.Equal((*paymentVoucher).ValidAt, decodedPaymentVoucher.ValidAt)
}

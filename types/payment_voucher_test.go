package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
)

func TestPaymentVoucherEncodingRoundTrip(t *testing.T) {
	addrGetter := address.NewForTestGetter()
	addr1 := addrGetter()
	addr2 := addrGetter()

	condition := &Predicate{
		To:     addrGetter(),
		Method: "someMethod",
		Params: []interface{}{"some encoded parameters"},
	}

	paymentVoucher := &PaymentVoucher{
		Channel:   *NewChannelID(5),
		Payer:     addr1,
		Target:    addr2,
		Amount:    *NewAttoFILFromFIL(100),
		ValidAt:   *NewBlockHeight(25),
		Condition: condition,
	}

	rawPaymentVoucher, err := paymentVoucher.Encode()
	require.NoError(t, err)
	decodedPaymentVoucher, err := DecodeVoucher(rawPaymentVoucher)
	require.NoError(t, err)

	assert.Equal(t, (*paymentVoucher).Channel, decodedPaymentVoucher.Channel)
	assert.Equal(t, (*paymentVoucher).Payer, decodedPaymentVoucher.Payer)
	assert.Equal(t, (*paymentVoucher).Target, decodedPaymentVoucher.Target)
	assert.Equal(t, (*paymentVoucher).Amount, decodedPaymentVoucher.Amount)
	assert.Equal(t, (*paymentVoucher).ValidAt, decodedPaymentVoucher.ValidAt)

	assert.Equal(t, condition.To, decodedPaymentVoucher.Condition.To)
	assert.Equal(t, condition.Method, decodedPaymentVoucher.Condition.Method)
	assert.Equal(t, condition.Params, decodedPaymentVoucher.Condition.Params)
}

package types_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	. "github.com/filecoin-project/go-filecoin/types"
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
		Amount:    NewAttoFILFromFIL(100),
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

func TestSortVouchersByValidAt(t *testing.T) {
	var pvs []*PaymentVoucher
	addrGetter := address.NewForTestGetter()

	validAts := []uint64{8, 2, 9, 22, 1}

	expected := []uint64{1, 2, 8, 9, 22}

	for i := 0; i < 5; i++ {
		condition := &Predicate{
			To:     addrGetter(),
			Method: "someMethod",
			Params: []interface{}{"some encoded parameters"},
		}
		pv := &PaymentVoucher{
			Channel:   *NewChannelID(uint64(5)),
			Payer:     addrGetter(),
			Target:    addrGetter(),
			Amount:    NewAttoFILFromFIL(100),
			ValidAt:   *NewBlockHeight(validAts[i]),
			Condition: condition,
		}
		pvs = append(pvs, pv)
	}

	sorted := SortVouchersByValidAt(pvs)
	for i := 0; i < 5; i++ {
		assert.True(t, sorted[i].ValidAt.Equal(NewBlockHeight(expected[i])))
	}
}

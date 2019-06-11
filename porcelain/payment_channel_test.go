package porcelain_test

import (
	"context"
	"testing"

	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/porcelain"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

type testPaymentChannelLsPlumbing struct {
	testing  *testing.T
	channels map[string]*paymentbroker.PaymentChannel
}

func (p *testPaymentChannelLsPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
	chnls, err := cbor.DumpObject(p.channels)
	require.NoError(p.testing, err)
	return [][]byte{chnls}, nil
}

func (p *testPaymentChannelLsPlumbing) WalletDefaultAddress() (address.Address, error) {
	return address.Undef, nil
}

func TestPaymentChannelLs(t *testing.T) {
	tf.UnitTest(t)

	t.Run("succeeds", func(t *testing.T) {
		expectedChannels := map[string]*paymentbroker.PaymentChannel{}

		plumbing := &testPaymentChannelLsPlumbing{
			channels: expectedChannels,
			testing:  t,
		}
		ctx := context.Background()

		channels, err := porcelain.PaymentChannelLs(ctx, plumbing, address.Undef, address.Undef)
		require.NoError(t, err)
		assert.Equal(t, expectedChannels, channels)
	})
}

type testPaymentChannelVoucherPlumbing struct {
	testing *testing.T
	voucher *types.PaymentVoucher
}

func (p *testPaymentChannelVoucherPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
	result, err := actor.MarshalStorage(p.voucher)
	require.NoError(p.testing, err)
	return [][]byte{result}, nil
}

func (p *testPaymentChannelVoucherPlumbing) SignBytes(data []byte, addr address.Address) (types.Signature, error) {
	return []byte("test"), nil
}

func (p *testPaymentChannelVoucherPlumbing) WalletDefaultAddress() (address.Address, error) {
	return address.Undef, nil
}

func TestPaymentChannelVoucher(t *testing.T) {
	tf.UnitTest(t)

	t.Run("succeeds", func(t *testing.T) {
		expectedVoucher := &types.PaymentVoucher{
			Channel:   *types.NewChannelID(5),
			Payer:     address.Undef,
			Target:    address.Undef,
			Amount:    types.NewAttoFILFromFIL(10),
			ValidAt:   *types.NewBlockHeight(0),
			Signature: []byte{},
			Condition: &types.Predicate{
				To:     address.Undef,
				Method: "someMethod",
				Params: []interface{}{"params"},
			},
		}

		plumbing := &testPaymentChannelVoucherPlumbing{
			testing: t,
			voucher: expectedVoucher,
		}
		ctx := context.Background()

		voucher, err := porcelain.PaymentChannelVoucher(
			ctx,
			plumbing,
			address.Undef,
			types.NewChannelID(5),
			types.NewAttoFILFromFIL(10),
			types.NewBlockHeight(0),
			&types.Predicate{
				To:     address.Undef,
				Method: "someMethod",
				Params: []interface{}{"params"},
			},
		)
		require.NoError(t, err)
		assert.Equal(t, expectedVoucher.Channel, voucher.Channel)
		assert.Equal(t, expectedVoucher.Payer, voucher.Payer)
		assert.Equal(t, expectedVoucher.Target, voucher.Target)
		assert.Equal(t, expectedVoucher.Amount, voucher.Amount)
		assert.Equal(t, expectedVoucher.ValidAt, voucher.ValidAt)
		assert.Equal(t, expectedVoucher.Condition.To, voucher.Condition.To)
		assert.Equal(t, expectedVoucher.Condition.Method, voucher.Condition.Method)
		assert.Equal(t, expectedVoucher.Condition.Params, voucher.Condition.Params)
		assert.NotEqual(t, expectedVoucher.Signature, voucher.Signature)
	})
}

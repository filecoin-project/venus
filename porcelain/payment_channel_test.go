package porcelain_test

import (
	"context"
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/types"
)

type testPaymentChannelLsPlumbing struct {
	require  *require.Assertions
	channels map[string]*paymentbroker.PaymentChannel
}

func (p *testPaymentChannelLsPlumbing) GetAndMaybeSetDefaultSenderAddress() (address.Address, error) {
	return address.Undef, nil
}

func (p *testPaymentChannelLsPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error) {
	chnls, err := cbor.DumpObject(p.channels)
	p.require.NoError(err)
	return [][]byte{chnls}, nil, nil
}

func TestPaymentChannelLs(t *testing.T) {
	t.Parallel()

	t.Run("succeeds", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		expectedChannels := map[string]*paymentbroker.PaymentChannel{}

		plumbing := &testPaymentChannelLsPlumbing{
			channels: expectedChannels,
			require:  require,
		}
		ctx := context.Background()

		channels, err := porcelain.PaymentChannelLs(ctx, plumbing, address.Undef, address.Undef)
		require.NoError(err)
		assert.Equal(expectedChannels, channels)
	})
}

type testPaymentChannelVoucherPlumbing struct {
	require *require.Assertions
	voucher *paymentbroker.PaymentVoucher
}

func (p *testPaymentChannelVoucherPlumbing) GetAndMaybeSetDefaultSenderAddress() (address.Address, error) {
	return address.Undef, nil
}

func (p *testPaymentChannelVoucherPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error) {
	result, err := actor.MarshalStorage(p.voucher)
	p.require.NoError(err)
	return [][]byte{result}, nil, nil
}

func (p *testPaymentChannelVoucherPlumbing) SignBytes(data []byte, addr address.Address) (types.Signature, error) {
	return []byte("test"), nil
}

func TestPaymentChannelVoucher(t *testing.T) {
	t.Parallel()

	t.Run("succeeds", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		expectedVoucher := &paymentbroker.PaymentVoucher{
			Channel:   *types.NewChannelID(5),
			Payer:     address.Undef,
			Target:    address.Undef,
			Amount:    *types.NewAttoFILFromFIL(10),
			ValidAt:   *types.NewBlockHeight(0),
			Signature: []byte{},
		}

		plumbing := &testPaymentChannelVoucherPlumbing{
			require: require,
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
		)
		require.NoError(err)
		assert.Equal(expectedVoucher.Channel, voucher.Channel)
		assert.Equal(expectedVoucher.Payer, voucher.Payer)
		assert.Equal(expectedVoucher.Target, voucher.Target)
		assert.Equal(expectedVoucher.Amount, voucher.Amount)
		assert.Equal(expectedVoucher.ValidAt, voucher.ValidAt)
		assert.NotEqual(expectedVoucher.Signature, voucher.Signature)
	})
}

package porcelain_test

import (
	"context"
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/porcelain"
)

type testPaymentChannelLsPlumbing struct {
	require  *require.Assertions
	channels map[string]*paymentbroker.PaymentChannel
}

func (p *testPaymentChannelLsPlumbing) GetAndMaybeSetDefaultSenderAddress() (address.Address, error) {
	return address.Address{}, nil
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

		channels, err := porcelain.PaymentChannelLs(ctx, plumbing, address.Address{}, address.Address{})
		require.NoError(err)
		assert.Equal(expectedChannels, channels)
	})
}

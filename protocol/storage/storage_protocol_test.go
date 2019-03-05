package storage_test

import (
	"testing"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/types"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

func TestSerializeProposal(t *testing.T) {
	require := require.New(t)

	t.Parallel()

	ag := address.NewForTestGetter()
	cg := types.NewCidForTestGetter()
	p := &storagedeal.Proposal{}
	p.Size = types.NewBytesAmount(5)
	cmc := cg()
	p.Payment.ChannelMsgCid = &cmc
	p.Payment.Channel = types.NewChannelID(4)
	voucher := &paymentbroker.PaymentVoucher{
		Channel:   *types.NewChannelID(4),
		Payer:     ag(),
		Target:    ag(),
		Amount:    *types.NewAttoFILFromFIL(3),
		ValidAt:   *types.NewBlockHeight(3),
		Signature: types.Signature{},
	}
	p.Payment.Vouchers = []*paymentbroker.PaymentVoucher{voucher}
	v, _ := cid.Decode("QmcrriCMhjb5ZWzmPNxmP53px47tSPcXBNaMtLdgcKFJYk")
	p.PieceRef = v
	chunk, err := cbor.DumpObject(p)
	require.NoError(err)

	err = cbor.DecodeInto(chunk, &storagedeal.Proposal{})
	require.NoError(err)
}

package storage_test

import (
	"testing"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/require"
)

func TestSerializeProposal(t *testing.T) {
	tf.UnitTest(t)

	ag := address.NewForTestGetter()
	cg := types.NewCidForTestGetter()
	p := &storagedeal.Proposal{}
	p.Size = types.NewBytesAmount(5)
	cmc := cg()
	p.Payment.ChannelMsgCid = &cmc
	p.Payment.Channel = types.NewChannelID(4)
	voucher := &types.PaymentVoucher{
		Channel:   *types.NewChannelID(4),
		Payer:     ag(),
		Target:    ag(),
		Amount:    types.NewAttoFILFromFIL(3),
		ValidAt:   *types.NewBlockHeight(3),
		Signature: types.Signature{},
	}
	p.Payment.Vouchers = []*types.PaymentVoucher{voucher}
	v, _ := cid.Decode("QmcrriCMhjb5ZWzmPNxmP53px47tSPcXBNaMtLdgcKFJYk")
	p.PieceRef = v
	chunk, err := cbor.DumpObject(p)
	require.NoError(t, err)

	err = cbor.DecodeInto(chunk, &storagedeal.Proposal{})
	require.NoError(t, err)
}

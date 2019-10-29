package storage_test

import (
	"testing"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/storage/storagedeal"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
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
	chunk, err := encoding.Encode(p)
	require.NoError(t, err)

	err = encoding.Decode(chunk, &storagedeal.Proposal{})
	require.NoError(t, err)
}

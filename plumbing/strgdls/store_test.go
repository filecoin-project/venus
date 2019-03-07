package strgdls_test

import (
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	"testing"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/plumbing/strgdls"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/util/convert"
)

func TestDealStoreRoundTrip(t *testing.T) {
	addressMaker := address.NewForTestGetter()

	store := strgdls.New(repo.NewInMemoryRepo().DealsDs)
	minerAddr := addressMaker()
	pieceRefCid, err := convert.ToCid("pieceRef")
	require.NoError(t, err)
	size := types.NewBytesAmount(12)
	totalPrice := types.NewAttoFILFromFIL(13)
	duration := uint64(23)
	channelID := *types.NewChannelID(19)
	channelMessageCid, err := convert.ToCid(&types.Message{})
	require.NoError(t, err)
	clientAddr := addressMaker()
	validAt := types.NewBlockHeight(231)
	responseMessage := "Success!"

	payment := storagedeal.PaymentInfo{PayChActor: addressMaker(),
		Payer:         addressMaker(),
		Channel:       &channelID,
		ChannelMsgCid: &channelMessageCid,
		Vouchers: []*paymentbroker.PaymentVoucher{{
			Channel: channelID,
			Payer:   clientAddr,
			Target:  minerAddr,
			Amount:  *totalPrice,
			ValidAt: *validAt,
		}}}

	proposal := &storagedeal.Proposal{
		PieceRef:     pieceRefCid,
		Size:         size,
		TotalPrice:   totalPrice,
		Duration:     duration,
		MinerAddress: minerAddr,
		Payment:      payment,
	}

	proposalCid, err := convert.ToCid(proposal)
	require.NoError(t, err)

	storageDeal := &storagedeal.Deal{
		Miner:    minerAddr,
		Proposal: proposal,
		Response: &storagedeal.Response{
			State:       storagedeal.Accepted,
			Message:     responseMessage,
			ProposalCid: proposalCid,
			ProofInfo:   &storagedeal.ProofInfo{},
			Signature:   []byte("signature"),
		},
	}

	require.NoError(t, store.Put(storageDeal))
	deals, err := store.Ls()
	require.NoError(t, err)

	retrievedDeal := deals[0]

	assert.Equal(t, minerAddr, retrievedDeal.Miner)

	assert.Equal(t, pieceRefCid, retrievedDeal.Proposal.PieceRef)
	assert.Equal(t, size, retrievedDeal.Proposal.Size)
	assert.Equal(t, totalPrice, retrievedDeal.Proposal.TotalPrice)
	assert.Equal(t, duration, retrievedDeal.Proposal.Duration)
	assert.Equal(t, minerAddr, retrievedDeal.Proposal.MinerAddress)

	assert.Equal(t, channelID, retrievedDeal.Proposal.Payment.Vouchers[0].Channel)
	assert.Equal(t, clientAddr, retrievedDeal.Proposal.Payment.Vouchers[0].Payer)
	assert.Equal(t, minerAddr, retrievedDeal.Proposal.Payment.Vouchers[0].Target)
	assert.Equal(t, *totalPrice, retrievedDeal.Proposal.Payment.Vouchers[0].Amount)
	assert.Equal(t, *validAt, retrievedDeal.Proposal.Payment.Vouchers[0].ValidAt)
}

package retrieval_market_connector_test

import (
	"context"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-fil-markets/shared/tokenamount"
	gfm_types "github.com/filecoin-project/go-fil-markets/shared/types"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/paymentbroker"
	fcaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

type retrievalMarketClientTestAPI struct {
	allocateLaneErr error

	expectedLanes    map[fcaddr.Address]uint64       // mock payment broker lane store
	expectedPmtChans map[fcaddr.Address]pmtChanEntry // mock payment broker's payment channel store
	actualPmtChans   map[fcaddr.Address]pmtChanEntry // validate that the payment channels were created

	balance       tokenamount.TokenAmount
	balanceErr    error
	workerAddr    fcaddr.Address
	workerAddrErr error
	nextNonce     uint64
	nextNonceErr  error
	walletAddr    fcaddr.Address
	walletAddrErr error
	sig           *gfm_types.Signature
	sigErr        error
	msgSendCid    cid.Cid
	msgSendErr    error
	waitErr       error

	expectedBlock      *block.Block
	expectedMsgReceipt *types.MessageReceipt
	expectedSignedMsg  *types.SignedMessage
}

func (tapa *retrievalMarketClientTestAPI) Wait(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	if tapa.waitErr != nil {
		return tapa.waitErr
	}

	clientAddr := tapa.expectedSignedMsg.Message.From
	tapa.actualPmtChans[clientAddr] = tapa.expectedPmtChans[clientAddr]

	cb(tapa.expectedBlock, tapa.expectedSignedMsg, tapa.expectedMsgReceipt)
	return nil
}

// pmtChanEntry is a record of a created payment channel with funds available.
type pmtChanEntry struct {
	payee      fcaddr.Address
	channelID  fcaddr.Address
	fundsAvail tokenamount.TokenAmount
}

func NewRetrievalMarketClientTestAPI(t *testing.T, bal tokenamount.TokenAmount) *retrievalMarketClientTestAPI {
	return &retrievalMarketClientTestAPI{
		balance:          bal,
		workerAddr:       requireMakeTestFcAddr(t),
		nextNonce:        rand.Uint64(),
		walletAddr:       requireMakeTestFcAddr(t),
		expectedLanes:    make(map[fcaddr.Address]uint64),
		expectedPmtChans: make(map[fcaddr.Address]pmtChanEntry),
		actualPmtChans:   make(map[fcaddr.Address]pmtChanEntry),
	}
}

func (tapa *retrievalMarketClientTestAPI) GetBalance(_ context.Context, _ fcaddr.Address) (types.AttoFIL, error) {
	return types.NewAttoFIL(tapa.balance.Int), tapa.balanceErr
}
func (tapa *retrievalMarketClientTestAPI) GetWorkerAddress(_ context.Context, _ fcaddr.Address, _ block.TipSetKey) (fcaddr.Address, error) {
	return tapa.workerAddr, tapa.workerAddrErr
}
func (tapa *retrievalMarketClientTestAPI) NextNonce(_ context.Context, _ fcaddr.Address) (uint64, error) {
	tapa.nextNonce++
	return tapa.nextNonce, tapa.nextNonceErr
}
func (tapa *retrievalMarketClientTestAPI) GetDefaultWalletAddress() (fcaddr.Address, error) {
	return tapa.walletAddr, tapa.walletAddrErr
}
func (tapa *retrievalMarketClientTestAPI) SignBytes(data []byte, addr fcaddr.Address) (types.Signature, error) {
	return tapa.sig.Data, tapa.sigErr
}

func (tapa *retrievalMarketClientTestAPI) Send(ctx context.Context, from, to fcaddr.Address, value types.AttoFIL,
	gasPrice types.AttoFIL, gasLimit types.GasUnits, bcast bool, method types.MethodID, params ...interface{}) (out cid.Cid, pubErrCh chan error, err error) {
	tapa.nextNonce++

	if err != nil {
		return cid.Undef, nil, err
	}
	return tapa.msgSendCid, nil, tapa.msgSendErr
}

// GetPaymentChannelIDByPayee searches for a payment channel for a payer + payee.
// It does not assume the payment channel has been created. If not found, returns
// 0 channel ID and nil.
func (tapa *retrievalMarketClientTestAPI) GetPaymentChannelAddress(ctx context.Context, payer, _ fcaddr.Address) (fcaddr.Address, error) {
	entry, ok := tapa.actualPmtChans[payer]
	if !ok {
		return fcaddr.Undef, nil
	}
	// assuming only one client for test purposes
	return entry.channelID, nil
}

// // GetPaymentChannelByChannelID searches for a payment channel by its ID
// // It assumes the PaymentChannel has been created, and returns empty payment channel
// // + error if the channel ID is not found
// func (tapa *retrievalMarketClientTestAPI) GetPaymentChannelByChannelID(_ context.Context, payer fcaddr.Address, id *types.ChannelID) (*paymentbroker.PaymentChannel, error) {
// 	entry, ok := tapa.expectedPmtChans[payer]
// 	if !ok || !entry.channelID.Equal(id) {
// 		return &paymentbroker.PaymentChannel{}, errors.New("payment channel not found")
// 	}
//
// 	return &paymentbroker.PaymentChannel{
// 		Target: entry.payee,
// 		Amount: types.NewAttoFIL(entry.fundsAvail.Int),
// 	}, nil
// }

func (tapa *retrievalMarketClientTestAPI) AllocateLane(_ context.Context, _ fcaddr.Address, chid fcaddr.Address) (uint64, error) {
	lane, ok := tapa.expectedLanes[chid]
	if ok {
		tapa.expectedLanes[chid] = lane+1
	}
	return lane, nil
}

func (tapa *retrievalMarketClientTestAPI) StubMessageResponse(t *testing.T, fcClient, fcMiner fcaddr.Address) {
	params, err := abi.ToEncodedValues(fcMiner, uint64(1))
	require.NoError(t, err)
	unsignedMsg := types.UnsignedMessage{
		To:         fcaddr.LegacyPaymentBrokerAddress,
		From:       fcClient,
		CallSeqNum: 0,
		Value:      types.ZeroAttoFIL,
		Method:     paymentbroker.CreateChannel,
		Params:     params,
		GasPrice:   types.AttoFIL{},
		GasLimit:   0,
	}

	newAddr, err := fcaddr.NewIDAddress(rand.Uint64())
	require.NoError(t, err)
	tapa.expectedPmtChans[fcClient] = pmtChanEntry{
		channelID:  newAddr,
		fundsAvail: tapa.balance,
	}

	require.NoError(t, err)
	tapa.expectedMsgReceipt = &types.MessageReceipt{
		ExitCode:   0,
		Return:     [][]byte{newAddr.Bytes()},
		GasAttoFIL: types.AttoFIL{},
	}

	mockSigner, _ := types.NewMockSignersAndKeyInfo(1)
	addr1 := mockSigner.Addresses[0]

	marshaled, err := unsignedMsg.Marshal()
	require.NoError(t, err)
	sig, err := mockSigner.SignBytes(marshaled, addr1)
	require.NoError(t, err)
	signature := &gfm_types.Signature{
		Type: gfm_types.KTBLS,
		Data: sig,
	}
	tapa.sig = signature
	tapa.expectedSignedMsg = &types.SignedMessage{
		Message:   unsignedMsg,
		Signature: sig,
	}
}

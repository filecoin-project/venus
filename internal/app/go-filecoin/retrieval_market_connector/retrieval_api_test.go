package retrieval_market_connector_test

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared/tokenamount"
	gfm_types "github.com/filecoin-project/go-fil-markets/shared/types"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/retrieval_market_connector"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/paymentbroker"
	fcaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

type retrievalMarketClientTestAPI struct {
	allocateLaneErr error

	expectedLanes    map[address.Address]uint64       // mock payment broker lane store
	expectedPmtChans map[address.Address]pmtChanEntry // mock payment broker's payment channel store
	actualPmtChans   map[address.Address]pmtChanEntry // to check that the payment channels were created

	payChBalanceErr error

	balance       tokenamount.TokenAmount
	balanceErr    error
	workerAddr    address.Address
	workerAddrErr error
	nextNonce     uint64
	nextNonceErr  error
	walletAddr    address.Address
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
// pmtChanEntry is a record of a created payment channel with funds available.
type pmtChanEntry struct {
	payee      address.Address
	redeemed tokenamount.TokenAmount
	channelID  address.Address
	fundsAvail tokenamount.TokenAmount
}

func (tapa *retrievalMarketClientTestAPI) GetPaymentChannelInfo(_ context.Context, paymentChannel address.Address) (address.Address, paymentbroker.PaymentChannel, error) {
	for payer, entry := range tapa.actualPmtChans {
		if entry.channelID == paymentChannel {
			fcTarget, _ := fcaddr.NewFromBytes(entry.payee.Bytes())
			pch := paymentbroker.PaymentChannel{
				Target:         fcTarget,
				Amount:         types.NewAttoFIL(entry.fundsAvail.Int),
				AmountRedeemed: types.NewAttoFIL(entry.redeemed.Int),
			}

			return payer, pch, nil
		}
	}
	return address.Undef, paymentbroker.PaymentChannel{}, errors.New("no such channelID")
}

func (tapa *retrievalMarketClientTestAPI) Wait(_ context.Context, _ cid.Cid, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	if tapa.waitErr != nil {
		return tapa.waitErr
	}

	clientAddr, err := retrieval_market_connector.GoAddrFromFcAddr(tapa.expectedSignedMsg.Message.From)
	if err != nil {
		return err
	}
	tapa.actualPmtChans[clientAddr] = tapa.expectedPmtChans[clientAddr]

	cb(tapa.expectedBlock, tapa.expectedSignedMsg, tapa.expectedMsgReceipt)
	return nil
}

func NewRetrievalMarketClientTestAPI(t *testing.T, bal tokenamount.TokenAmount) *retrievalMarketClientTestAPI {
	return &retrievalMarketClientTestAPI{
		balance:          bal,
		workerAddr:       requireMakeTestFcAddr(t),
		nextNonce:        rand.Uint64(),
		walletAddr:       requireMakeTestFcAddr(t),
		expectedLanes:    make(map[address.Address]uint64),
		expectedPmtChans: make(map[address.Address]pmtChanEntry),
		actualPmtChans:   make(map[address.Address]pmtChanEntry),
	}
}

func (tapa *retrievalMarketClientTestAPI) GetBalance(_ context.Context, _ address.Address) (types.AttoFIL, error) {
	return types.NewAttoFIL(tapa.balance.Int), tapa.balanceErr
}
func (tapa *retrievalMarketClientTestAPI) GetWorkerAddress(_ context.Context, _ address.Address, _ block.TipSetKey) (address.Address, error) {
	return tapa.workerAddr, tapa.workerAddrErr
}
func (tapa *retrievalMarketClientTestAPI) NextNonce(_ context.Context, _ address.Address) (uint64, error) {
	tapa.nextNonce++
	return tapa.nextNonce, tapa.nextNonceErr
}
func (tapa *retrievalMarketClientTestAPI) GetDefaultWalletAddress() (address.Address, error) {
	return tapa.walletAddr, tapa.walletAddrErr
}
func (tapa *retrievalMarketClientTestAPI) SignBytes(_ []byte, _ address.Address) (types.Signature, error) {
	return tapa.sig.Data, tapa.sigErr
}

func (tapa *retrievalMarketClientTestAPI) Send(_ context.Context, _, _ address.Address, _ types.AttoFIL,
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
func (tapa *retrievalMarketClientTestAPI) GetPaymentChannelID(ctx context.Context, payer, _ address.Address) (address.Address, error) {
	entry, ok := tapa.actualPmtChans[payer]
	if !ok {
		return address.Undef, nil
	}
	// assuming only one client for test purposes
	return entry.channelID, nil
}

func (tapa *retrievalMarketClientTestAPI) AllocateLane(_ context.Context, _ address.Address, chid address.Address) (uint64, error) {
	lane, ok := tapa.expectedLanes[chid]
	if ok {
		tapa.expectedLanes[chid] = lane+1
	}
	return lane, nil
}

func (tapa *retrievalMarketClientTestAPI) StubMessageResponse(t *testing.T, from, to address.Address, value types.AttoFIL) {
	fcTo, err := retrieval_market_connector.FcAddrFromGoAddr(to)
	require.NoError(t, err)
	fcFrom, err := retrieval_market_connector.FcAddrFromGoAddr(from)
	require.NoError(t, err)

	params, err := abi.ToEncodedValues(fcTo, uint64(1))
	require.NoError(t, err)

	unsignedMsg := types.UnsignedMessage{
		To:         fcaddr.LegacyPaymentBrokerAddress,
		From:       fcFrom,
		CallSeqNum: 0,
		Value:      value,
		Method:     paymentbroker.CreateChannel,
		Params:     params,
		GasPrice:   types.AttoFIL{},
		GasLimit:   0,
	}

	newAddr, err := address.NewIDAddress(rand.Uint64())
	require.NoError(t, err)
	tapa.expectedPmtChans[from] = pmtChanEntry{
		channelID:  newAddr,
		fundsAvail: tokenamount.TokenAmount{Int: value.AsBigInt()},
		redeemed: tokenamount.FromInt(0),
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

func requireMakeTestFcAddr(t *testing.T) address.Address {
	res, err := address.NewIDAddress(rand.Uint64())
	require.NoError(t, err)
	return res
}

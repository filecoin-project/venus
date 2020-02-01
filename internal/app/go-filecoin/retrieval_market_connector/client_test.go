package retrieval_market_connector_test

import (
	"context"
	"errors"
	"math/big"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared/tokenamount"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/retrieval_market_connector"
	tut "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/shared_testutils"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/paymentbroker"
	fcaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

func TestNewRetrievalClientNodeConnector(t *testing.T) {
	ctx := context.Background()
	tapa := &testAllPowerfulAPI{
		balance:    types.NewAttoFILFromFIL(1),
		workerAddr: requireMakeTestFcAddr(t),
		nextNonce:  rand.Uint64(),
		walletAddr: requireMakeTestFcAddr(t),
		sig:        big.NewInt(rand.Int63()).Bytes(),
	}

	bs := bstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	cs := newChainStore(ctx, t)

	ps := tut.RequireMakeTestPieceStore(t)
	rcnc := retrieval_market_connector.NewRetrievalClientNodeConnector(&bs, cs, tapa, tapa, ps, tapa, tapa, tapa, tapa)
	assert.NotNil(t, rcnc)
}

func TestRetrievalClientNodeConnector_GetOrCreatePaymentChannel(t *testing.T) {
	ctx := context.Background()

	tapa := &testAllPowerfulAPI{
		balance:    types.NewAttoFILFromFIL(1000),
		workerAddr: requireMakeTestFcAddr(t),
		nextNonce:  rand.Uint64(),
		walletAddr: requireMakeTestFcAddr(t),
		sig:        big.NewInt(rand.Int63()).Bytes(),
	}

	bs := bstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	cs := newChainStore(ctx, t)

	ps := tut.RequireMakeTestPieceStore(t)
	clientAddr, err := address.NewIDAddress(rand.Uint64())
	require.NoError(t, err)
	fcClient, err := fcaddr.NewFromBytes(clientAddr.Bytes())
	require.NoError(t, err)
	
	minerAddr, err := address.NewIDAddress(rand.Uint64())
	require.NoError(t, err)
	fMiner, err := fcaddr.NewFromBytes(minerAddr.Bytes())
	require.NoError(t, err)

	t.Run("Errors if clientWallet get balance fails", func(t *testing.T) {
		tapa.balanceErr = errors.New("boom")
		rcnc := retrieval_market_connector.NewRetrievalClientNodeConnector(&bs, cs, tapa, tapa, ps, tapa, tapa, tapa, tapa)
		res, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, tokenamount.FromInt(500))
		assert.EqualError(t, err, "boom")
		assert.Equal(t, address.Undef, res)
		tapa.balanceErr = nil
	})

	t.Run("if the payment channel does not exist", func(t *testing.T) {
		t.Run("creates a new payment channel registry entry", func(t *testing.T) {
			params, err := abi.ToEncodedValues(fMiner, uint64(1))
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

			chid := types.NewChannelID(12345)
			chidEnc, err := abi.ToEncodedValues(chid)
			require.NoError(t, err)
			codeEnc, err := abi.ToEncodedValues(uint64(0))
			require.NoError(t, err)
			tapa.expectedMsgReceipt = &types.MessageReceipt{
				ExitCode:   0,
				Return:     [][]byte{chidEnc, codeEnc},
				GasAttoFIL: types.AttoFIL{},
			}
			tapa.expectedSignedMsg = &types.SignedMessage{
				Message:   unsignedMsg,
				Signature: nil,
			}
			rcnc := retrieval_market_connector.NewRetrievalClientNodeConnector(&bs, cs, tapa, tapa, ps, tapa, tapa, tapa, tapa)

			res, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, tokenamount.FromInt(rand.Uint64()))
			assert.NoError(t, err)
			assert.NotNil(t, address.Undef, res)
			chans := rcnc.ListRegisteredChannels()
			assert.Equal(t, []address.Address{clientAddr}, chans)

		})
		t.Run("new payment channel message is posted on chain", func(t *testing.T) {})
		t.Run("returns address.Undef and nil", func(t *testing.T) {})
		t.Run("Errors if there aren't enough funds in wallet", func(t *testing.T) {
			tapa.balance = types.NewAttoFILFromFIL(0)
			rcnc := retrieval_market_connector.NewRetrievalClientNodeConnector(&bs, cs, tapa, tapa, ps, tapa, tapa, tapa, tapa)
			res, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, tokenamount.FromInt(500))
			assert.EqualError(t, err, "not enough funds in wallet")
			assert.Equal(t, address.Undef, res)
		})
		t.Run("Errors if minerWallet addr is invalid", func(t *testing.T) {})
		t.Run("Errors if cant get head tipset", func(t *testing.T) {})
		t.Run("Errors if cant get chain height", func(t *testing.T) {})
	})

	t.Run("if payment channel exists", func(t *testing.T) {
		t.Run("Retrieves existing payment channel address", func(t *testing.T) {})
		t.Run("Returns a payment channel type of address and nil", func(t *testing.T) {})
	})

	t.Run("When message posts to chain", func(t *testing.T) {
		t.Run("results added to payment channel registry", func(t *testing.T) {})
		t.Run("sets payment channel id in registry", func(t *testing.T) {})
		t.Run("sets error in registry if receipt can't be deserialized", func(t *testing.T) {})
		t.Run("sets error in registry if address in message is not in the registry", func(t *testing.T) {})
	})
}

func TestRetrievalClientNodeConnector_AllocateLane(t *testing.T) {
	t.Run("Errors if payment channel does not exist", func(t *testing.T) {})
	t.Run("Increments and returns lastLane val", func(t *testing.T) {})
}

func TestRetrievalClientNodeConnector_CreatePaymentVoucher(t *testing.T) {
	t.Run("Returns a voucher with a signature", func(t *testing.T) {})
	t.Run("Errors if payment channel does not exist", func(t *testing.T) {})
	t.Run("Errors if there aren't enough funds", func(t *testing.T) {})
	t.Run("Errors if payment channel lane doesn't exist", func(t *testing.T) {})
	t.Run("Errors if can't get nonce", func(t *testing.T) {})
	t.Run("Errors if can't get wallet addr", func(t *testing.T) {})
	t.Run("Errors if can't sign bytes", func(t *testing.T) {})
	t.Run("Errors if can't get signature from bytes", func(t *testing.T) {})
}

type testAllPowerfulAPI struct {
	balance       types.AttoFIL
	balanceErr    error
	workerAddr    fcaddr.Address
	workerAddrErr error
	nextNonce     uint64
	nextNonceErr  error
	walletAddr    fcaddr.Address
	walletAddrErr error
	sig           []byte
	sigErr        error
	msgSendCid    cid.Cid
	msgSendErr    error

	expectedBlock      *block.Block
	expectedMsgReceipt *types.MessageReceipt
	expectedSignedMsg  *types.SignedMessage
}

func (tapa *testAllPowerfulAPI) GetBalance(_ context.Context, _ fcaddr.Address) (types.AttoFIL, error) {
	return tapa.balance, tapa.balanceErr
}
func (tapa *testAllPowerfulAPI) GetWorkerAddress(_ context.Context, _ fcaddr.Address, _ block.TipSetKey) (fcaddr.Address, error) {
	return tapa.workerAddr, tapa.workerAddrErr
}
func (tapa *testAllPowerfulAPI) NextNonce(_ context.Context, _ address.Address) (uint64, error) {
	tapa.nextNonce++
	return tapa.nextNonce, tapa.nextNonceErr
}
func (tapa *testAllPowerfulAPI) GetDefaultWalletAddress() (fcaddr.Address, error) {
	return tapa.walletAddr, tapa.walletAddrErr
}
func (tapa *testAllPowerfulAPI) SignBytes(data []byte, addr fcaddr.Address) ([]byte, error) {
	return tapa.sig, tapa.sigErr
}

func (tapa *testAllPowerfulAPI) Wait(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	cb(tapa.expectedBlock, tapa.expectedSignedMsg, tapa.expectedMsgReceipt)
	return nil
}

func (tapa *testAllPowerfulAPI) Send(ctx context.Context, from, to fcaddr.Address, value types.AttoFIL,
	gasPrice types.AttoFIL, gasLimit types.GasUnits, bcast bool, method types.MethodID, params ...interface{}) (out cid.Cid, pubErrCh chan error, err error) {
	return tapa.msgSendCid, nil, tapa.msgSendErr
}

func requireMakeTestFcAddr(t *testing.T) fcaddr.Address {
	res, err := fcaddr.NewIDAddress(rand.Uint64())
	require.NoError(t, err)
	return res
}

func newChainStore(ctx context.Context, t *testing.T) *chain.Store {
	cst := hamt.NewCborStore()

	// Cribbed from chain/store_test
	// fakeCode := types.CidFromString(t, "somecid")
	// balance := types.NewAttoFILFromFIL(1000000)
	// testActor := actor.NewActor(fakeCode, balance)
	// addr := address.NewForTestGetter()()
	st1 := state.NewTree(cst)
	// require.NoError(t, st1.SetActor(ctx, addr, testActor))
	root, err := st1.Flush(ctx)
	require.NoError(t, err)

	// link testing state to test block
	builder := chain.NewBuilder(t, fcaddr.Undef)
	genTS := builder.NewGenesis()
	r := repo.NewInMemoryRepo()

	// setup chain store
	ds := r.Datastore()
	cs := chain.NewStore(ds, cst, state.NewTreeLoader(), chain.NewStatusReporter(), genTS.At(0).Cid())

	rootBlk := builder.AppendBlockOnBlocks()
	th.RequireNewTipSet(t, rootBlk)
	require.NoError(t, cs.SetHead(ctx, genTS))

	// add tipset and state to chainstore
	require.NoError(t, cs.PutTipSetMetadata(ctx, &chain.TipSetMetadata{
		TipSet:          genTS,
		TipSetStateRoot: root,
		TipSetReceipts:  types.EmptyReceiptsCID,
	}))
	return cs
}

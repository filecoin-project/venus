package retrievalmarketconnector_test

import (
	"context"
	"math/big"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/require"

	retrievalmarketconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/retrieval_market_connector"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	fcaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

func TestNewRetrievalClientNodeConnector(t *testing.T) {
	tapa := testAllPowerfulAPI{
		balance:    types.NewAttoFILFromFIL(1),
		workerAddr: requireMakeTestFcAddr(t),
		nextNonce:  rand.Uint64(),
		walletAddr: requireMakeTestFcAddr(t),
		sig:        big.NewInt(rand.Int63()).Bytes(),
	}

	bs := bstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))

	ctx := context.Background()
	builder := chain.NewBuilder(t, fcaddr.Undef)
	genTS := builder.NewGenesis()
	r := repo.NewInMemoryRepo()
	cs := newChainStore(r, genTS.At(0).Cid())
	rcnc := retrievalmarketconnector.NewRetrievalClientNodeConnector(tapa, bs, cs, tapa, tapa, , tapa, tapa)
}

func TestRetrievalClientNodeConnector_GetOrCreatePaymentChannel(t *testing.T) {
	t.Run("Errors if clientWallet addr is invalid", func(t *testing.T) {})

	t.Run("if the payment channel must be created", func(t *testing.T) {
		t.Run("creates a new payment channel registry entry", func(t *testing.T) {})
		t.Run("new payment channel message is posted on chain", func(t *testing.T) {})
		t.Run("returns address.Undef and nil", func(t *testing.T) {})
		t.Run("Errors if there aren't enough funds in wallet", func(t *testing.T) {})
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
	return nil
}

func (tapa *testAllPowerfulAPI) Send(ctx context.Context, from, to address.Address, value types.AttoFIL,
	gasPrice types.AttoFIL, gasLimit types.GasUnits, bcast bool, method types.MethodID, params ...interface{}) (out cid.Cid, pubErrCh chan error, err error) {
	return tapa.msgSendCid, nil, tapa.msgSendErr
}

func requireMakeTestFcAddr(t *testing.T) fcaddr.Address {
	rando := big.NewInt(rand.Int63())
	res, err := fcaddr.NewFromBytes(rando.Bytes())
	require.NoError(t, err)
	return res
}

func newChainStore(r repo.Repo, genCid cid.Cid) *chain.Store {
	return chain.NewStore(r.Datastore(), hamt.NewCborStore(), state.NewTreeLoader(), chain.NewStatusReporter(), genCid)
}

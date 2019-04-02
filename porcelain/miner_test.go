package porcelain_test

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/plumbing/cfg"
	. "github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type minerCreate struct {
	assert  *assert.Assertions
	require *require.Assertions
	address address.Address
	config  *cfg.Config
	wallet  *wallet.Wallet
	msgCid  cid.Cid
	msgFail bool
}

func newMinerCreate(assert *assert.Assertions, require *require.Assertions, msgFail bool, address address.Address) *minerCreate {
	repo := repo.NewInMemoryRepo()
	backend, err := wallet.NewDSBackend(repo.WalletDatastore())
	require.NoError(err)
	return &minerCreate{
		assert:  assert,
		require: require,
		address: address,
		config:  cfg.NewConfig(repo),
		wallet:  wallet.New(backend),
		msgFail: msgFail,
	}
}

func (mpc *minerCreate) ConfigGet(dottedPath string) (interface{}, error) {
	return mpc.config.Get(dottedPath)
}

func (mpc *minerCreate) ConfigSet(dottedPath string, paramJSON string) error {
	return mpc.config.Set(dottedPath, paramJSON)
}

func (mpc *minerCreate) MessageSendWithDefaultAddress(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
	if mpc.msgFail {
		return cid.Cid{}, errors.New("Test Error")
	}
	mpc.msgCid = types.SomeCid()
	return mpc.msgCid, nil
}

func (mpc *minerCreate) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	mpc.assert.Equal(mpc.msgCid, msgCid)
	receipt := &types.MessageReceipt{
		Return:   [][]byte{mpc.address.Bytes()},
		ExitCode: uint8(0),
	}
	return cb(nil, nil, receipt)
}

func (mpc *minerCreate) WalletDefaultAddress() (address.Address, error) {
	return wallet.NewAddress(mpc.wallet)
}

func (mpc *minerCreate) WalletGetPubKeyForAddress(addr address.Address) ([]byte, error) {
	return mpc.wallet.GetPubKeyForAddress(addr)
}

func TestMinerCreate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		ctx := context.Background()
		expectedAddress := address.NewForTestGetter()()
		plumbing := newMinerCreate(assert, require, false, expectedAddress)
		collateral := types.NewAttoFILFromFIL(1)

		addr, err := MinerCreate(
			ctx,
			plumbing,
			address.Address{},
			types.NewGasPrice(0),
			types.NewGasUnits(100),
			1,
			"",
			collateral,
		)
		require.NoError(err)
		assert.Equal(expectedAddress, *addr)
	})

	t.Run("failure to send", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		ctx := context.Background()
		plumbing := newMinerCreate(assert, require, true, address.Address{})
		collateral := types.NewAttoFILFromFIL(1)

		_, err := MinerCreate(
			ctx,
			plumbing,
			address.Address{},
			types.NewGasPrice(0),
			types.NewGasUnits(100),
			1,
			"",
			collateral,
		)
		assert.Error(err, "Test Error")
	})
}

type minerPreviewCreate struct {
	wallet *wallet.Wallet
}

func newMinerPreviewCreate(require *require.Assertions) *minerPreviewCreate {
	repo := repo.NewInMemoryRepo()
	backend, err := wallet.NewDSBackend(repo.WalletDatastore())
	wallet := wallet.New(backend)
	require.NoError(err)
	return &minerPreviewCreate{
		wallet: wallet,
	}
}

func (mpc *minerPreviewCreate) MessagePreview(_ context.Context, _, _ address.Address, _ string, _ ...interface{}) (types.GasUnits, error) {
	return types.NewGasUnits(5), nil
}

func (mpc *minerPreviewCreate) ConfigGet(dottedPath string) (interface{}, error) {
	return nil, nil
}

func (mpc *minerPreviewCreate) NetworkGetPeerID() peer.ID {
	return peer.ID("")
}

func (mpc *minerPreviewCreate) WalletDefaultAddress() (address.Address, error) {
	return wallet.NewAddress(mpc.wallet)
}

func (mpc *minerPreviewCreate) WalletFind(address address.Address) (wallet.Backend, error) {
	return mpc.wallet.Find(address)
}

func TestMinerPreviewCreate(t *testing.T) {
	t.Run("returns the price given by message preview", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		ctx := context.Background()
		plumbing := newMinerPreviewCreate(require)
		collateral := types.NewAttoFILFromFIL(1)

		usedGas, err := MinerPreviewCreate(ctx, plumbing, address.Undef, 1, "", collateral)
		require.NoError(err)
		assert.Equal(usedGas, types.NewGasUnits(5))
	})
}

type minerSetPricePlumbing struct {
	config  *cfg.Config
	assert  *assert.Assertions
	require *require.Assertions

	msgCid   cid.Cid
	blockCid cid.Cid

	failGet  bool
	failSet  bool
	failSend bool
	failWait bool

	messageSend func(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error)
}

func newMinerSetPricePlumbing(assert *assert.Assertions, require *require.Assertions) *minerSetPricePlumbing {
	return &minerSetPricePlumbing{
		config:  cfg.NewConfig(repo.NewInMemoryRepo()),
		assert:  assert,
		require: require,
	}
}

func (mtp *minerSetPricePlumbing) MessageSendWithDefaultAddress(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
	if mtp.failSend {
		return cid.Cid{}, errors.New("Test error in MessageSend")
	}

	if mtp.messageSend != nil {
		cid, err := mtp.messageSend(ctx, from, to, value, gasPrice, gasLimit, method, params...)
		mtp.msgCid = cid
		return cid, err
	}

	mtp.msgCid = types.NewCidForTestGetter()()
	return mtp.msgCid, nil
}

// calls back immediately
func (mtp *minerSetPricePlumbing) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	if mtp.failWait {
		return errors.New("Test error in MessageWait")
	}

	mtp.require.True(msgCid.Equals(mtp.msgCid))

	block := &types.Block{
		Nonce: 393,
	}
	mtp.blockCid = block.Cid()

	// call back
	return cb(block, &types.SignedMessage{}, &types.MessageReceipt{ExitCode: 0, Return: [][]byte{}})
}

func (mtp *minerSetPricePlumbing) ConfigSet(dottedKey string, jsonString string) error {
	if mtp.failSet {
		return errors.New("Test error in ConfigSet")
	}

	return mtp.config.Set(dottedKey, jsonString)
}

func (mtp *minerSetPricePlumbing) ConfigGet(dottedPath string) (interface{}, error) {
	if mtp.failGet {
		return nil, errors.New("Test error in ConfigGet")
	}

	return mtp.config.Get(dottedPath)
}

func TestMinerSetPrice(t *testing.T) {
	t.Run("reports error when get miner address fails", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newMinerSetPricePlumbing(assert, require)
		plumbing.failGet = true

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		_, err := MinerSetPrice(ctx, plumbing, address.Undef, address.Undef, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.Error(err)
		assert.Contains(err.Error(), "Test error in ConfigGet")
	})

	t.Run("reports error when setting into config", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newMinerSetPricePlumbing(assert, require)
		plumbing.failSet = true

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		_, err := MinerSetPrice(ctx, plumbing, address.Undef, address.Undef, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.Error(err)
		assert.Contains(err.Error(), "Test error in ConfigSet")
	})

	t.Run("sets price into config", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newMinerSetPricePlumbing(assert, require)

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		_, err := MinerSetPrice(ctx, plumbing, address.Undef, address.Undef, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.NoError(err)

		configPrice, err := plumbing.config.Get("mining.storagePrice")
		require.NoError(err)

		assert.Equal(price, configPrice)
	})

	t.Run("saves config and reports error when send fails", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newMinerSetPricePlumbing(assert, require)
		plumbing.failSend = true

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		_, err := MinerSetPrice(ctx, plumbing, address.Undef, address.Undef, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.Error(err)
		assert.Contains(err.Error(), "Test error in MessageSend")

		configPrice, err := plumbing.config.Get("mining.storagePrice")
		require.NoError(err)

		assert.Equal(price, configPrice)
	})

	t.Run("sends ask to specific miner when miner is given", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newMinerSetPricePlumbing(assert, require)

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		minerAddr := address.NewForTestGetter()()

		plumbing.messageSend = func(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
			assert.Equal(minerAddr, to)
			return types.NewCidForTestGetter()(), nil
		}

		_, err := MinerSetPrice(ctx, plumbing, address.Undef, minerAddr, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.NoError(err)
	})

	t.Run("sends ask to default miner when no miner is given", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newMinerSetPricePlumbing(assert, require)

		minerAddr := address.NewForTestGetter()()
		require.NoError(plumbing.config.Set("mining.minerAddress", minerAddr.String()))

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)

		plumbing.messageSend = func(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
			assert.Equal(minerAddr, to)
			return types.NewCidForTestGetter()(), nil
		}

		_, err := MinerSetPrice(ctx, plumbing, address.Undef, address.Undef, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.NoError(err)
	})

	t.Run("sends ask with correct parameters", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newMinerSetPricePlumbing(assert, require)

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		expiry := big.NewInt(24)

		plumbing.messageSend = func(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
			assert.Equal("addAsk", method)
			assert.Equal(price, params[0])
			assert.Equal(expiry, params[1])
			return types.NewCidForTestGetter()(), nil
		}

		_, err := MinerSetPrice(ctx, plumbing, address.Undef, address.Undef, types.NewGasPrice(0), types.NewGasUnits(0), price, expiry)
		require.NoError(err)
	})

	t.Run("reports error when wait fails", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newMinerSetPricePlumbing(assert, require)
		plumbing.failWait = true

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)

		_, err := MinerSetPrice(ctx, plumbing, address.Undef, address.Undef, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.Error(err)
		assert.Contains(err.Error(), "Test error in MessageWait")
	})

	t.Run("returns interesting information about adding the ask", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newMinerSetPricePlumbing(assert, require)

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		expiry := big.NewInt(24)
		minerAddr := address.NewForTestGetter()()

		messageCid := types.NewCidForTestGetter()()

		plumbing.messageSend = func(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
			return messageCid, nil
		}

		res, err := MinerSetPrice(ctx, plumbing, address.Undef, minerAddr, types.NewGasPrice(0), types.NewGasUnits(0), price, expiry)
		require.NoError(err)

		assert.Equal(price, res.Price)
		assert.Equal(minerAddr, res.MinerAddr)
		assert.Equal(messageCid, res.AddAskCid)
		assert.Equal(plumbing.blockCid, res.BlockCid)
	})
}

type minerPreviewSetPricePlumbing struct {
	config *cfg.Config
}

func newMinerPreviewSetPricePlumbing() *minerPreviewSetPricePlumbing {
	return &minerPreviewSetPricePlumbing{
		config: cfg.NewConfig(repo.NewInMemoryRepo()),
	}
}

func (mtp *minerPreviewSetPricePlumbing) MessagePreview(ctx context.Context, from, to address.Address, method string, params ...interface{}) (types.GasUnits, error) {
	return types.NewGasUnits(7), nil
}

func (mtp *minerPreviewSetPricePlumbing) ConfigSet(dottedKey string, jsonString string) error {
	return mtp.config.Set(dottedKey, jsonString)
}

func (mtp *minerPreviewSetPricePlumbing) ConfigGet(dottedPath string) (interface{}, error) {
	return mtp.config.Get(dottedPath)
}

func TestMinerPreviewSetPrice(t *testing.T) {
	t.Run("returns the gas cost given by preview query", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newMinerPreviewSetPricePlumbing()
		ctx := context.Background()
		price := types.NewAttoFILFromFIL(0)

		usedGas, err := MinerPreviewSetPrice(ctx, plumbing, address.Undef, address.Undef, price, big.NewInt(0))

		require.NoError(err)
		assert.Equal(types.NewGasUnits(7), usedGas)
	})
}

type minerGetOwnerPlumbing struct{}

func (mgop *minerGetOwnerPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
	return [][]byte{address.TestAddress.Bytes()}, nil
}

func TestMinerGetOwnerAddress(t *testing.T) {
	assert := assert.New(t)

	addr, err := MinerGetOwnerAddress(context.Background(), &minerGetOwnerPlumbing{}, address.TestAddress2)
	assert.NoError(err)
	assert.Equal(address.TestAddress, addr)
}

type minerGetPeerIDPlumbing struct{}

func (mgop *minerGetPeerIDPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {

	peerID := requirePeerID()
	return [][]byte{[]byte(peerID)}, nil
}

func TestMinerGetPeerID(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	id, err := MinerGetPeerID(context.Background(), &minerGetPeerIDPlumbing{}, address.TestAddress2)
	require.NoError(err)

	expected := requirePeerID()
	require.NoError(err)
	assert.Equal(expected, id)
}

type minerGetAskPlumbing struct{}

func (mgop *minerGetAskPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
	out, err := cbor.DumpObject(miner.Ask{
		Price:  types.NewAttoFILFromFIL(32),
		Expiry: types.NewBlockHeight(41),
		ID:     big.NewInt(4),
	})
	if err != nil {
		panic("Could not encode ask")
	}
	return [][]byte{out}, nil
}

func TestMinerGetAsk(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ask, err := MinerGetAsk(context.Background(), &minerGetAskPlumbing{}, address.TestAddress2, 4)
	require.NoError(err)

	assert.Equal(types.NewAttoFILFromFIL(32), ask.Price)
	assert.Equal(types.NewBlockHeight(41), ask.Expiry)
	assert.Equal(big.NewInt(4), ask.ID)
}

func requirePeerID() peer.ID {
	id, err := peer.IDB58Decode("QmWbMozPyW6Ecagtxq7SXBXXLY5BNdP1GwHB2WoZCKMvcb")
	if err != nil {
		panic("Could not create peer id")
	}
	return id
}

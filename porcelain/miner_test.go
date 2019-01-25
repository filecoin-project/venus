package porcelain

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/plumbing/cfg"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type minerTestPlumbing struct {
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

func newMinerTestPlumbing(assert *assert.Assertions, require *require.Assertions) *minerTestPlumbing {
	return &minerTestPlumbing{
		config:  cfg.NewConfig(repo.NewInMemoryRepo()),
		assert:  assert,
		require: require,
	}
}

func (mtp *minerTestPlumbing) MessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
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
func (mtp *minerTestPlumbing) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	if mtp.failWait {
		return errors.New("Test error in MessageWait")
	}

	mtp.require.True(msgCid.Equals(mtp.msgCid))

	block := &types.Block{
		Nonce: 393,
	}
	mtp.blockCid = block.Cid()

	// call back
	cb(block, &types.SignedMessage{}, &types.MessageReceipt{ExitCode: 0, Return: []types.Bytes{}})

	return nil
}

func (mtp *minerTestPlumbing) ConfigSet(dottedKey string, jsonString string) error {
	if mtp.failSet {
		return errors.New("Test error in ConfigSet")
	}

	return mtp.config.Set(dottedKey, jsonString)
}

func (mtp *minerTestPlumbing) ConfigGet(dottedPath string) (interface{}, error) {
	if mtp.failGet {
		return nil, errors.New("Test error in ConfigGet")
	}

	return mtp.config.Get(dottedPath)
}

func TestMinerSetPrice(t *testing.T) {
	t.Run("reports error when get miner address fails", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newMinerTestPlumbing(assert, require)
		plumbing.failGet = true

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		_, err := MinerSetPrice(ctx, plumbing, address.Address{}, address.Address{}, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.Error(err)
		assert.Contains(err.Error(), "Test error in ConfigGet")
	})

	t.Run("reports error when setting into config", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newMinerTestPlumbing(assert, require)
		plumbing.failSet = true

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		_, err := MinerSetPrice(ctx, plumbing, address.Address{}, address.Address{}, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.Error(err)
		assert.Contains(err.Error(), "Test error in ConfigSet")
	})

	t.Run("sets price into config", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newMinerTestPlumbing(assert, require)

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		_, err := MinerSetPrice(ctx, plumbing, address.Address{}, address.Address{}, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.NoError(err)

		configPrice, err := plumbing.config.Get("mining.storagePrice")
		require.NoError(err)

		assert.Equal(price, configPrice)
	})

	t.Run("saves config and reports error when send fails", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newMinerTestPlumbing(assert, require)
		plumbing.failSend = true

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		_, err := MinerSetPrice(ctx, plumbing, address.Address{}, address.Address{}, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.Error(err)
		assert.Contains(err.Error(), "Test error in MessageSend")

		configPrice, err := plumbing.config.Get("mining.storagePrice")
		require.NoError(err)

		assert.Equal(price, configPrice)
	})

	t.Run("sends ask to specific miner when miner is given", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newMinerTestPlumbing(assert, require)

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		minerAddr := address.NewForTestGetter()()

		plumbing.messageSend = func(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
			assert.Equal(minerAddr, to)
			return types.NewCidForTestGetter()(), nil
		}

		_, err := MinerSetPrice(ctx, plumbing, address.Address{}, minerAddr, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.NoError(err)
	})

	t.Run("sends ask to default miner when no miner is given", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newMinerTestPlumbing(assert, require)

		minerAddr := address.NewForTestGetter()()
		require.NoError(plumbing.config.Set("mining.minerAddress", minerAddr.String()))

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)

		plumbing.messageSend = func(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
			assert.Equal(minerAddr, to)
			return types.NewCidForTestGetter()(), nil
		}

		_, err := MinerSetPrice(ctx, plumbing, address.Address{}, address.Address{}, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.NoError(err)
	})

	t.Run("sends ask with correct parameters", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newMinerTestPlumbing(assert, require)

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		expiry := big.NewInt(24)

		plumbing.messageSend = func(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
			assert.Equal("addAsk", method)
			assert.Equal(price, params[0])
			assert.Equal(expiry, params[1])
			return types.NewCidForTestGetter()(), nil
		}

		_, err := MinerSetPrice(ctx, plumbing, address.Address{}, address.Address{}, types.NewGasPrice(0), types.NewGasUnits(0), price, expiry)
		require.NoError(err)
	})

	t.Run("reports error when wait fails", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newMinerTestPlumbing(assert, require)
		plumbing.failWait = true

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)

		_, err := MinerSetPrice(ctx, plumbing, address.Address{}, address.Address{}, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.Error(err)
		assert.Contains(err.Error(), "Test error in MessageWait")
	})

	t.Run("returns interesting information about adding the ask", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newMinerTestPlumbing(assert, require)

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		expiry := big.NewInt(24)
		minerAddr := address.NewForTestGetter()()

		messageCid := types.NewCidForTestGetter()()

		plumbing.messageSend = func(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
			return messageCid, nil
		}

		res, err := MinerSetPrice(ctx, plumbing, address.Address{}, minerAddr, types.NewGasPrice(0), types.NewGasUnits(0), price, expiry)
		require.NoError(err)

		assert.Equal(price, res.Price)
		assert.Equal(minerAddr, res.MinerAddr)
		assert.Equal(messageCid, res.AddAskCid)
		assert.Equal(plumbing.blockCid, res.BlockCid)
	})
}

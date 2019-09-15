package porcelain_test

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/filecoin-project/go-leb128"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/plumbing/cfg"
	. "github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/repo"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type minerCreate struct {
	testing *testing.T
	address address.Address
	config  *cfg.Config
	wallet  *wallet.Wallet
	msgCid  cid.Cid
	msgFail bool
}

func newMinerCreate(t *testing.T, msgFail bool, address address.Address) *minerCreate {
	testRepo := repo.NewInMemoryRepo()
	backend, err := wallet.NewDSBackend(testRepo.WalletDatastore())
	require.NoError(t, err)
	return &minerCreate{
		testing: t,
		address: address,
		config:  cfg.NewConfig(testRepo),
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

func (mpc *minerCreate) MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
	if mpc.msgFail {
		return cid.Cid{}, errors.New("test Error")
	}
	mpc.msgCid = types.CidFromString(mpc.testing, "somecid")

	return mpc.msgCid, nil
}

func (mpc *minerCreate) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	assert.Equal(mpc.testing, msgCid, msgCid)
	receipt := &types.MessageReceipt{
		Return:   [][]byte{mpc.address.Bytes()},
		ExitCode: uint8(0),
	}
	return cb(nil, nil, receipt)
}

func (mpc *minerCreate) WalletDefaultAddress() (address.Address, error) {
	return wallet.NewAddress(mpc.wallet)
}

func TestMinerCreate(t *testing.T) {
	tf.UnitTest(t)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		expectedAddress := address.NewForTestGetter()()
		plumbing := newMinerCreate(t, false, expectedAddress)
		collateral := types.NewAttoFILFromFIL(1)

		addr, err := MinerCreate(
			ctx,
			plumbing,
			address.Address{},
			types.NewGasPrice(0),
			types.NewGasUnits(100),
			types.OneKiBSectorSize,
			"",
			collateral,
		)
		require.NoError(t, err)
		assert.Equal(t, expectedAddress, *addr)
	})

	t.Run("failure to send", func(t *testing.T) {
		ctx := context.Background()
		plumbing := newMinerCreate(t, true, address.Address{})
		collateral := types.NewAttoFILFromFIL(1)

		_, err := MinerCreate(
			ctx,
			plumbing,
			address.Address{},
			types.NewGasPrice(0),
			types.NewGasUnits(100),
			types.OneKiBSectorSize,
			"",
			collateral,
		)
		assert.Error(t, err, "Test Error")
	})
}

type minerPreviewCreate struct {
	wallet *wallet.Wallet
}

func newMinerPreviewCreate(t *testing.T) *minerPreviewCreate {
	testRepo := repo.NewInMemoryRepo()
	backend, err := wallet.NewDSBackend(testRepo.WalletDatastore())
	wlt := wallet.New(backend)
	require.NoError(t, err)
	return &minerPreviewCreate{
		wallet: wlt,
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

func TestMinerPreviewCreate(t *testing.T) {
	tf.UnitTest(t)

	t.Run("returns the price given by message preview", func(t *testing.T) {
		ctx := context.Background()
		plumbing := newMinerPreviewCreate(t)

		usedGas, err := MinerPreviewCreate(ctx, plumbing, address.Undef, types.OneKiBSectorSize, "")
		require.NoError(t, err)
		assert.Equal(t, usedGas, types.NewGasUnits(5))
	})
}

type minerSetPricePlumbing struct {
	config  *cfg.Config
	testing *testing.T

	msgCid   cid.Cid
	blockCid cid.Cid

	failGet  bool
	failSet  bool
	failSend bool
	failWait bool

	messageSend func(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error)
}

func newMinerSetPricePlumbing(t *testing.T) *minerSetPricePlumbing {
	return &minerSetPricePlumbing{
		config:  cfg.NewConfig(repo.NewInMemoryRepo()),
		testing: t,
	}
}

func (mtp *minerSetPricePlumbing) MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
	if mtp.failSend {
		return cid.Cid{}, errors.New("test error in MessageSend")
	}

	if mtp.messageSend != nil {
		msgCid, err := mtp.messageSend(ctx, from, to, value, gasPrice, gasLimit, method, params...)
		mtp.msgCid = msgCid
		return msgCid, err
	}

	mtp.msgCid = types.NewCidForTestGetter()()
	return mtp.msgCid, nil
}

// calls back immediately
func (mtp *minerSetPricePlumbing) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	if mtp.failWait {
		return errors.New("test error in MessageWait")
	}

	require.True(mtp.testing, msgCid.Equals(mtp.msgCid))

	block := &types.Block{}
	mtp.blockCid = block.Cid()

	// call back
	return cb(block, &types.SignedMessage{}, &types.MessageReceipt{ExitCode: 0, Return: [][]byte{}})
}

func (mtp *minerSetPricePlumbing) ConfigSet(dottedKey string, jsonString string) error {
	if mtp.failSet {
		return errors.New("test error in ConfigSet")
	}

	return mtp.config.Set(dottedKey, jsonString)
}

func (mtp *minerSetPricePlumbing) ConfigGet(dottedPath string) (interface{}, error) {
	if mtp.failGet {
		return nil, errors.New("test error in ConfigGet")
	}

	return mtp.config.Get(dottedPath)
}

func TestMinerSetPrice(t *testing.T) {
	tf.UnitTest(t)

	t.Run("reports error when get miner address fails", func(t *testing.T) {
		plumbing := newMinerSetPricePlumbing(t)
		plumbing.failGet = true

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		_, err := MinerSetPrice(ctx, plumbing, address.Undef, address.Undef, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "test error in ConfigGet")
	})

	t.Run("reports error when setting into config", func(t *testing.T) {
		plumbing := newMinerSetPricePlumbing(t)
		plumbing.failSet = true

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		_, err := MinerSetPrice(ctx, plumbing, address.Undef, address.Undef, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "test error in ConfigSet")
	})

	t.Run("sets price into config", func(t *testing.T) {
		plumbing := newMinerSetPricePlumbing(t)

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		_, err := MinerSetPrice(ctx, plumbing, address.Undef, address.Undef, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.NoError(t, err)

		configPrice, err := plumbing.config.Get("mining.storagePrice")
		require.NoError(t, err)

		assert.Equal(t, price, configPrice)
	})

	t.Run("saves config and reports error when send fails", func(t *testing.T) {
		plumbing := newMinerSetPricePlumbing(t)
		plumbing.failSend = true

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		_, err := MinerSetPrice(ctx, plumbing, address.Undef, address.Undef, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "test error in MessageSend")

		configPrice, err := plumbing.config.Get("mining.storagePrice")
		require.NoError(t, err)

		assert.Equal(t, price, configPrice)
	})

	t.Run("sends ask to specific miner when miner is given", func(t *testing.T) {
		plumbing := newMinerSetPricePlumbing(t)

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		minerAddr := address.NewForTestGetter()()

		plumbing.messageSend = func(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
			assert.Equal(t, minerAddr, to)
			return types.NewCidForTestGetter()(), nil
		}

		_, err := MinerSetPrice(ctx, plumbing, address.Undef, minerAddr, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.NoError(t, err)
	})

	t.Run("sends ask to default miner when no miner is given", func(t *testing.T) {
		plumbing := newMinerSetPricePlumbing(t)

		minerAddr := address.NewForTestGetter()()
		require.NoError(t, plumbing.config.Set("mining.minerAddress", minerAddr.String()))

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)

		plumbing.messageSend = func(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
			assert.Equal(t, minerAddr, to)
			return types.NewCidForTestGetter()(), nil
		}

		_, err := MinerSetPrice(ctx, plumbing, address.Undef, address.Undef, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.NoError(t, err)
	})

	t.Run("sends ask with correct parameters", func(t *testing.T) {
		plumbing := newMinerSetPricePlumbing(t)

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		expiry := big.NewInt(24)

		plumbing.messageSend = func(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
			assert.Equal(t, "addAsk", method)
			assert.Equal(t, price, params[0])
			assert.Equal(t, expiry, params[1])
			return types.NewCidForTestGetter()(), nil
		}

		_, err := MinerSetPrice(ctx, plumbing, address.Undef, address.Undef, types.NewGasPrice(0), types.NewGasUnits(0), price, expiry)
		require.NoError(t, err)
	})

	t.Run("reports error when wait fails", func(t *testing.T) {
		plumbing := newMinerSetPricePlumbing(t)
		plumbing.failWait = true

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)

		_, err := MinerSetPrice(ctx, plumbing, address.Undef, address.Undef, types.NewGasPrice(0), types.NewGasUnits(0), price, big.NewInt(0))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "test error in MessageWait")
	})

	t.Run("returns interesting information about adding the ask", func(t *testing.T) {
		plumbing := newMinerSetPricePlumbing(t)

		ctx := context.Background()
		price := types.NewAttoFILFromFIL(50)
		expiry := big.NewInt(24)
		minerAddr := address.NewForTestGetter()()

		messageCid := types.NewCidForTestGetter()()

		plumbing.messageSend = func(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
			return messageCid, nil
		}

		res, err := MinerSetPrice(ctx, plumbing, address.Undef, minerAddr, types.NewGasPrice(0), types.NewGasUnits(0), price, expiry)
		require.NoError(t, err)

		assert.Equal(t, price, res.Price)
		assert.Equal(t, minerAddr, res.MinerAddr)
		assert.Equal(t, messageCid, res.AddAskCid)
		assert.Equal(t, plumbing.blockCid, res.BlockCid)
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
	tf.UnitTest(t)

	t.Run("returns the gas cost given by preview query", func(t *testing.T) {
		plumbing := newMinerPreviewSetPricePlumbing()
		ctx := context.Background()
		price := types.NewAttoFILFromFIL(0)

		usedGas, err := MinerPreviewSetPrice(ctx, plumbing, address.Undef, address.Undef, price, big.NewInt(0))

		require.NoError(t, err)
		assert.Equal(t, types.NewGasUnits(7), usedGas)
	})
}

type minerQueryAndDeserializePlumbing struct{}

func (mgop *minerQueryAndDeserializePlumbing) ChainHeadKey() types.TipSetKey {
	return types.NewTipSetKey()
}

func (mgop *minerQueryAndDeserializePlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, _ types.TipSetKey, params ...interface{}) ([][]byte, error) {
	switch method {
	case "getOwner":
		return [][]byte{address.TestAddress.Bytes()}, nil
	case "getWorker":
		return [][]byte{address.TestAddress2.Bytes()}, nil
	case "getPower":
		return [][]byte{types.NewBytesAmount(2).Bytes()}, nil
	case "getTotalStorage":
		return [][]byte{types.NewBytesAmount(4).Bytes()}, nil
	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
	}
}

func (mgop *minerQueryAndDeserializePlumbing) ActorGetSignature(ctx context.Context, actorAddr address.Address, method string) (*exec.FunctionSignature, error) {
	if method == "getSectorSize" {
		return &exec.FunctionSignature{
			Params: nil,
			Return: []abi.Type{abi.BytesAmount},
		}, nil
	}

	return nil, fmt.Errorf("unsupported method: %s", method)
}

func TestMinerGetOwnerAddress(t *testing.T) {
	tf.UnitTest(t)

	addr, err := MinerGetOwnerAddress(context.Background(), &minerQueryAndDeserializePlumbing{}, address.TestAddress2)
	assert.NoError(t, err)
	assert.Equal(t, address.TestAddress, addr)
}

func TestMinerGetWorkerAddress(t *testing.T) {
	tf.UnitTest(t)

	addr, err := MinerGetWorkerAddress(context.Background(), &minerQueryAndDeserializePlumbing{}, address.TestAddress2, types.NewTipSetKey())
	assert.NoError(t, err)
	assert.Equal(t, address.TestAddress2, addr)
}

func TestMinerGetPower(t *testing.T) {
	tf.UnitTest(t)

	power, err := MinerGetPower(context.Background(), &minerQueryAndDeserializePlumbing{}, address.TestAddress2)
	assert.NoError(t, err)
	assert.Equal(t, "4", power.Total.String())
	assert.Equal(t, "2", power.Power.String())
}

type minerGetProvingPeriodPlumbing struct{}

func (mpp *minerGetProvingPeriodPlumbing) ChainHeadKey() types.TipSetKey {
	return types.NewTipSetKey()
}

func (mpp *minerGetProvingPeriodPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, _ types.TipSetKey, params ...interface{}) ([][]byte, error) {
	if method == "getProvingWindow" {
		return [][]byte{types.NewBlockHeight(10).Bytes(), types.NewBlockHeight(20).Bytes()}, nil
	}
	if method == "getProvingSetCommitments" {
		commitments := make(map[string]types.Commitments)
		commitments["foo"] = types.Commitments{
			CommD:     [32]byte{1},
			CommR:     [32]byte{1},
			CommRStar: [32]byte{1},
		}
		thing, _ := cbor.DumpObject(commitments)
		return [][]byte{thing}, nil
	}
	return nil, fmt.Errorf("unsupported method: %s", method)
}

func (mpp *minerGetProvingPeriodPlumbing) ActorGetSignature(ctx context.Context, actorAddr address.Address, method string) (*exec.FunctionSignature, error) {
	if method == "getProvingSetCommitments" {
		return &exec.FunctionSignature{
			Params: nil,
			Return: []abi.Type{abi.CommitmentsMap},
		}, nil
	}

	return nil, fmt.Errorf("unsupported method: %s", method)
}

func TestMinerProvingPeriod(t *testing.T) {
	tf.UnitTest(t)

	pp, err := MinerGetProvingWindow(context.Background(), &minerGetProvingPeriodPlumbing{}, address.TestAddress2)
	assert.NoError(t, err)
	assert.Equal(t, "10", pp.Start.String())
	assert.Equal(t, "20", pp.End.String())
	assert.NotNil(t, pp.ProvingSet["foo"])
}

type minerGetPeerIDPlumbing struct{}

func (mgop *minerGetPeerIDPlumbing) ChainHeadKey() types.TipSetKey {
	return types.NewTipSetKey()
}

func (mgop *minerGetPeerIDPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, _ types.TipSetKey, params ...interface{}) ([][]byte, error) {

	peerID := requirePeerID()
	return [][]byte{[]byte(peerID)}, nil
}

func TestMinerGetPeerID(t *testing.T) {
	tf.UnitTest(t)

	id, err := MinerGetPeerID(context.Background(), &minerGetPeerIDPlumbing{}, address.TestAddress2)
	require.NoError(t, err)

	expected := requirePeerID()
	require.NoError(t, err)
	assert.Equal(t, expected, id)
}

type minerGetAskPlumbing struct{}

func (mgop *minerGetAskPlumbing) ChainHeadKey() types.TipSetKey {
	return types.NewTipSetKey()
}

func (mgop *minerGetAskPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, _ types.TipSetKey, params ...interface{}) ([][]byte, error) {
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
	tf.UnitTest(t)

	ask, err := MinerGetAsk(context.Background(), &minerGetAskPlumbing{}, address.TestAddress2, 4)
	require.NoError(t, err)

	assert.Equal(t, types.NewAttoFILFromFIL(32), ask.Price)
	assert.Equal(t, types.NewBlockHeight(41), ask.Expiry)
	assert.Equal(t, big.NewInt(4), ask.ID)
}

func requirePeerID() peer.ID {
	id, err := peer.IDB58Decode("QmWbMozPyW6Ecagtxq7SXBXXLY5BNdP1GwHB2WoZCKMvcb")
	if err != nil {
		panic("Could not create peer id")
	}
	return id
}

type minerGetSectorSizePlumbing struct{}

func (minerGetSectorSizePlumbing) ChainHeadKey() types.TipSetKey {
	return types.NewTipSetKey()
}

func (minerGetSectorSizePlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, _ types.TipSetKey, params ...interface{}) ([][]byte, error) {
	return [][]byte{types.NewBytesAmount(1234).Bytes()}, nil
}
func (minerGetSectorSizePlumbing) ActorGetSignature(ctx context.Context, actorAddr address.Address, method string) (*exec.FunctionSignature, error) {
	return &exec.FunctionSignature{
		Params: nil,
		Return: []abi.Type{abi.BytesAmount},
	}, nil
}

func TestMinerGetSectorSize(t *testing.T) {
	tf.UnitTest(t)

	sectorSize, err := MinerGetSectorSize(context.Background(), &minerGetSectorSizePlumbing{}, address.TestAddress2)
	require.NoError(t, err)

	assert.Equal(t, int(sectorSize.Uint64()), 1234)
}

type minerGetLastCommittedSectorIDPlumbing struct{}

func (minerGetLastCommittedSectorIDPlumbing) ChainHeadKey() types.TipSetKey {
	return types.NewTipSetKey()
}

func (minerGetLastCommittedSectorIDPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, _ types.TipSetKey, params ...interface{}) ([][]byte, error) {
	return [][]byte{leb128.FromUInt64(5432)}, nil
}
func (minerGetLastCommittedSectorIDPlumbing) ActorGetSignature(ctx context.Context, actorAddr address.Address, method string) (*exec.FunctionSignature, error) {
	return &exec.FunctionSignature{
		Params: nil,
		Return: []abi.Type{abi.SectorID},
	}, nil
}

func TestMinerGetLastCommittedSectorID(t *testing.T) {
	tf.UnitTest(t)

	lastCommittedSectorID, err := MinerGetLastCommittedSectorID(context.Background(), &minerGetLastCommittedSectorIDPlumbing{}, address.TestAddress2)
	require.NoError(t, err)

	assert.Equal(t, int(lastCommittedSectorID), 5432)
}

type minerSetWorkerAddressPlumbing struct {
	getOwnerFail, getWorkerFail, msgFail, msgWaitFail, cfgFail bool
	minerAddr, ownerAddr, workerAddr                           address.Address
}

func (mswap *minerSetWorkerAddressPlumbing) MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {

	if mswap.msgFail {
		return cid.Cid{}, errors.New("MsgFail")
	}
	return types.EmptyMessagesCID, nil
}

func (mswap *minerSetWorkerAddressPlumbing) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	if mswap.msgWaitFail {
		return errors.New("MsgWaitFail")
	}
	return nil
}

func (mswap *minerSetWorkerAddressPlumbing) ConfigGet(dottedKey string) (interface{}, error) {
	if mswap.cfgFail {
		return address.Undef, errors.New("ConfigGet failed")
	}
	if dottedKey == "mining.minerAddress" {
		return mswap.minerAddr, nil
	}
	return address.Undef, fmt.Errorf("unknown config %s", dottedKey)
}

func (mswap *minerSetWorkerAddressPlumbing) MinerGetOwnerAddress(ctx context.Context, minerAddr address.Address) (address.Address, error) {
	if mswap.getOwnerFail {
		return address.Undef, errors.New("MinerGetOwnerAddress failed")
	}
	return mswap.ownerAddr, nil
}

func TestMinerSetWorkerAddress(t *testing.T) {
	tf.UnitTest(t)

	minerOwner := address.TestAddress
	minerAddr := address.NewForTestGetter()()
	workerAddr := address.NewForTestGetter()()
	gprice := types.ZeroAttoFIL
	glimit := types.NewGasUnits(0)

	t.Run("Calling set worker address sets address", func(t *testing.T) {
		plumbing := &minerSetWorkerAddressPlumbing{
			workerAddr: workerAddr,
			ownerAddr:  minerOwner,
			minerAddr:  minerAddr,
		}

		_, err := MinerSetWorkerAddress(context.Background(), plumbing, workerAddr, gprice, glimit)
		assert.NoError(t, err)
		assert.Equal(t, workerAddr.String(), plumbing.workerAddr.String())
	})

	testCases := []struct {
		name     string
		plumbing *minerSetWorkerAddressPlumbing
		error    string
	}{
		{
			name:     "When MessageSend fails, returns the error and does not set worker address",
			plumbing: &minerSetWorkerAddressPlumbing{msgFail: true},
			error:    "MsgFail",
		},
		{
			name:     "When ConfigGet fails, returns the error and does not set worker address",
			plumbing: &minerSetWorkerAddressPlumbing{cfgFail: true},
			error:    "CfgFail",
		},
		{
			name:     "When MinerGetOwnerAddress fails, returns the error and does not set worker address",
			plumbing: &minerSetWorkerAddressPlumbing{getOwnerFail: true},
			error:    "CfgFail",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			_, err := MinerSetWorkerAddress(context.Background(), test.plumbing, workerAddr, gprice, glimit)
			assert.Error(t, err, test.error)
			assert.Empty(t, test.plumbing.workerAddr)
		})
	}
}

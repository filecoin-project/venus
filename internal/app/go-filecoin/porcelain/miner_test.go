package porcelain_test

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-leb128"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cfg"
	. "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/power"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/wallet"

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

func (mpc *minerCreate) MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method types.MethodID, params ...interface{}) (cid.Cid, chan error, error) {
	if mpc.msgFail {
		return cid.Cid{}, nil, errors.New("test Error")
	}
	mpc.msgCid = types.CidFromString(mpc.testing, "somecid")

	return mpc.msgCid, nil, nil
}

func (mpc *minerCreate) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	assert.Equal(mpc.testing, msgCid, msgCid)
	receipt := &types.MessageReceipt{
		Return:   [][]byte{mpc.address.Bytes()},
		ExitCode: uint8(0),
	}
	return cb(nil, nil, receipt)
}

func (mpc *minerCreate) WalletDefaultAddress() (address.Address, error) {
	return wallet.NewAddress(mpc.wallet, address.SECP256K1)
}

func TestMinerCreate(t *testing.T) {
	tf.UnitTest(t)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		expectedAddress := vmaddr.NewForTestGetter()()
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

type minerQueryAndDeserializePlumbing struct{}

func (mgop *minerQueryAndDeserializePlumbing) ChainHeadKey() block.TipSetKey {
	return block.NewTipSetKey()
}

func (mgop *minerQueryAndDeserializePlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, _ block.TipSetKey, params ...interface{}) ([][]byte, error) {
	// Note: this currently happens to work as is, but it's wrong
	// Note: a better mock is recommended to make sure the correct methods get dispatched
	switch method {
	case miner.GetOwner:
		return [][]byte{vmaddr.TestAddress.Bytes()}, nil
	case miner.GetWorker:
		return [][]byte{vmaddr.TestAddress2.Bytes()}, nil
	case power.GetPowerReport:
		powerReport := types.NewPowerReport(2, 0)
		val := abi.Value{
			Val:  powerReport,
			Type: abi.PowerReport,
		}
		raw, err := val.Serialize()
		return [][]byte{raw}, err
	case power.GetTotalPower:
		return [][]byte{types.NewBytesAmount(4).Bytes()}, nil
	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
	}
}

func (mgop *minerQueryAndDeserializePlumbing) ActorGetStableSignature(ctx context.Context, actorAddr address.Address, method types.MethodID) (vm.ActorMethodSignature, error) {
	panic("{Dragons} delete")
}

func TestMinerGetOwnerAddress(t *testing.T) {
	tf.UnitTest(t)

	addr, err := MinerGetOwnerAddress(context.Background(), &minerQueryAndDeserializePlumbing{}, vmaddr.TestAddress2)
	assert.NoError(t, err)
	assert.Equal(t, vmaddr.TestAddress, addr)
}

func TestMinerGetWorkerAddress(t *testing.T) {
	tf.UnitTest(t)

	addr, err := MinerGetWorkerAddress(context.Background(), &minerQueryAndDeserializePlumbing{}, vmaddr.TestAddress2, block.NewTipSetKey())
	assert.NoError(t, err)
	assert.Equal(t, vmaddr.TestAddress2, addr)
}

func TestMinerGetPower(t *testing.T) {
	tf.UnitTest(t)

	power, err := MinerGetPower(context.Background(), &minerQueryAndDeserializePlumbing{}, vmaddr.TestAddress2)
	assert.NoError(t, err)
	assert.Equal(t, "4", power.Total.String())
	assert.Equal(t, "2", power.Power.String())
}

type minerGetProvingPeriodPlumbing struct{}

func (mpp *minerGetProvingPeriodPlumbing) ChainHeadKey() block.TipSetKey {
	return block.NewTipSetKey()
}

func (mpp *minerGetProvingPeriodPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, _ block.TipSetKey, params ...interface{}) ([][]byte, error) {
	if method == miner.GetProvingWindow {
		ret, err := (&abi.Value{Type: abi.UintArray, Val: []types.Uint64{10, 20}}).Serialize()
		if err != nil {
			return nil, err
		}
		return [][]byte{ret}, nil
	}
	if method == miner.GetProvingSetCommitments {
		commitments := make(map[string]types.Commitments)
		commD := types.CommD([32]byte{1})
		commR := types.CommR([32]byte{1})
		commRStar := types.CommRStar([32]byte{1})

		commitments["foo"] = types.Commitments{
			CommD:     &commD,
			CommR:     &commR,
			CommRStar: &commRStar,
		}
		thing, err := encoding.Encode(commitments)
		if err != nil {
			return nil, err
		}
		return [][]byte{thing}, nil
	}
	return nil, fmt.Errorf("unsupported method: %s", method)
}

func (mpp *minerGetProvingPeriodPlumbing) ActorGetStableSignature(ctx context.Context, actorAddr address.Address, method types.MethodID) (vm.ActorMethodSignature, error) {
	panic("{Dragons} delete")
}

func TestMinerProvingPeriod(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("cbor limitation with possible workaround but not worth it for test that will be deleted")

	pp, err := MinerGetProvingWindow(context.Background(), &minerGetProvingPeriodPlumbing{}, vmaddr.TestAddress2)
	assert.NoError(t, err)
	assert.Equal(t, "10", pp.Start.String())
	assert.Equal(t, "20", pp.End.String())
	assert.NotNil(t, pp.ProvingSet["foo"])
}

type minerGetPeerIDPlumbing struct{}

func (mgop *minerGetPeerIDPlumbing) ChainHeadKey() block.TipSetKey {
	return block.NewTipSetKey()
}

func (mgop *minerGetPeerIDPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, _ block.TipSetKey, params ...interface{}) ([][]byte, error) {

	peerID := requirePeerID()
	return [][]byte{[]byte(peerID)}, nil
}

func TestMinerGetPeerID(t *testing.T) {
	tf.UnitTest(t)

	id, err := MinerGetPeerID(context.Background(), &minerGetPeerIDPlumbing{}, vmaddr.TestAddress2)
	require.NoError(t, err)

	expected := requirePeerID()
	require.NoError(t, err)
	assert.Equal(t, expected, id)
}

type minerGetAskPlumbing struct{}

func (mgop *minerGetAskPlumbing) ChainHeadKey() block.TipSetKey {
	return block.NewTipSetKey()
}

func (mgop *minerGetAskPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, _ block.TipSetKey, params ...interface{}) ([][]byte, error) {
	out, err := encoding.Encode(miner.Ask{
		Price:  types.NewAttoFILFromFIL(32),
		Expiry: types.NewBlockHeight(41),
		ID:     big.NewInt(4),
	})
	if err != nil {
		panic("Could not encode ask")
	}
	return [][]byte{out}, nil
}

func requirePeerID() peer.ID {
	id, err := peer.Decode("QmWbMozPyW6Ecagtxq7SXBXXLY5BNdP1GwHB2WoZCKMvcb")
	if err != nil {
		panic("Could not create peer id")
	}
	return id
}

type minerGetSectorSizePlumbing struct{}

func (minerGetSectorSizePlumbing) ChainHeadKey() block.TipSetKey {
	return block.NewTipSetKey()
}

func (minerGetSectorSizePlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, _ block.TipSetKey, params ...interface{}) ([][]byte, error) {
	return [][]byte{types.NewBytesAmount(1234).Bytes()}, nil
}
func (minerGetSectorSizePlumbing) ActorGetStableSignature(ctx context.Context, actorAddr address.Address, method types.MethodID) (vm.ActorMethodSignature, error) {
	panic("{Dragons} delete")
}

func TestMinerGetSectorSize(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("delte or rewrite")

	sectorSize, err := MinerGetSectorSize(context.Background(), &minerGetSectorSizePlumbing{}, vmaddr.TestAddress2)
	require.NoError(t, err)

	assert.Equal(t, int(sectorSize.Uint64()), 1234)
}

type minerGetLastCommittedSectorIDPlumbing struct{}

func (minerGetLastCommittedSectorIDPlumbing) ChainHeadKey() block.TipSetKey {
	return block.NewTipSetKey()
}

func (minerGetLastCommittedSectorIDPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, _ block.TipSetKey, params ...interface{}) ([][]byte, error) {
	return [][]byte{leb128.FromUInt64(5432)}, nil
}
func (minerGetLastCommittedSectorIDPlumbing) ActorGetStableSignature(ctx context.Context, actorAddr address.Address, method types.MethodID) (vm.ActorMethodSignature, error) {
	panic("{Dragons} delete")

}

func TestMinerGetLastCommittedSectorID(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("rewrite or delete")

	lastCommittedSectorID, err := MinerGetLastCommittedSectorID(context.Background(), &minerGetLastCommittedSectorIDPlumbing{}, vmaddr.TestAddress2)
	require.NoError(t, err)

	assert.Equal(t, int(lastCommittedSectorID), 5432)
}

type minerSetWorkerAddressPlumbing struct {
	getOwnerFail, getWorkerFail, msgFail, msgWaitFail, cfgFail bool
	minerAddr, ownerAddr, workerAddr                           address.Address
}

func (mswap *minerSetWorkerAddressPlumbing) MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method types.MethodID, params ...interface{}) (cid.Cid, chan error, error) {

	if mswap.msgFail {
		return cid.Cid{}, nil, errors.New("MsgFail")
	}
	return types.EmptyMessagesCID, nil, nil
}

func (mswap *minerSetWorkerAddressPlumbing) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
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

	minerOwner := vmaddr.TestAddress
	minerAddr := vmaddr.NewForTestGetter()()
	workerAddr := vmaddr.NewForTestGetter()()
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
		assert.Equal(t, workerAddr, plumbing.workerAddr)
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

package porcelain_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cfg"
	. "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/wallet"
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

func (mpc *minerCreate) MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method abi.MethodNum, params interface{}) (cid.Cid, chan error, error) {
	if mpc.msgFail {
		return cid.Cid{}, nil, errors.New("test Error")
	}
	mpc.msgCid = types.CidFromString(mpc.testing, "somecid")

	return mpc.msgCid, nil, nil
}

func (mpc *minerCreate) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *vm.MessageReceipt) error) error {
	assert.Equal(mpc.testing, msgCid, msgCid)
	receipt := vm.MessageReceipt{
		ReturnValue: mpc.address.Bytes(),
		ExitCode:    exitcode.Ok,
	}
	return cb(nil, nil, &receipt)
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
			types.GasUnits(100),
			constants.DevSectorSize,
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
			types.GasUnits(100),
			constants.DevSectorSize,
			"",
			collateral,
		)
		assert.Error(t, err, "Test Error")
	})
}

type mStatusPlumbing struct {
	head                 block.TipSetKey
	miner, owner, worker address.Address
}

func (p *mStatusPlumbing) ChainHeadKey() block.TipSetKey {
	return p.head
}

func (p *mStatusPlumbing) MinerStateView(baseKey block.TipSetKey) (MinerStateView, error) {
	return &state.FakeStateView{
		NetworkPower: abi.NewStoragePower(4),
		Miners: map[address.Address]*state.FakeMinerState{
			p.miner: {
				Owner:        p.owner,
				Worker:       p.worker,
				ClaimedPower: abi.NewStoragePower(2),
			},
		},
	}, nil
}

func TestMinerGetStatus(t *testing.T) {
	tf.UnitTest(t)
	key := block.NewTipSetKey(types.NewCidForTestGetter()())

	plumbing := mStatusPlumbing{
		key, vmaddr.RequireIDAddress(t, 1), vmaddr.RequireIDAddress(t, 2), vmaddr.RequireIDAddress(t, 3),
	}
	status, err := MinerGetStatus(context.Background(), &plumbing, plumbing.miner, key)
	assert.NoError(t, err)
	assert.Equal(t, plumbing.owner, status.OwnerAddress)
	assert.Equal(t, plumbing.worker, status.WorkerAddress)
	assert.Equal(t, "4", status.NetworkPower.String())
	assert.Equal(t, "2", status.Power.String())
}

type mSetWorkerPlumbing struct {
	head                                         block.TipSetKey
	getStatusFail, msgFail, msgWaitFail, cfgFail bool
	minerAddr, ownerAddr, workerAddr             address.Address
}

func (p *mSetWorkerPlumbing) ChainHeadKey() block.TipSetKey {
	return p.head
}

func (p *mSetWorkerPlumbing) MinerStateView(baseKey block.TipSetKey) (MinerStateView, error) {
	if p.getStatusFail {
		return &state.FakeStateView{}, errors.New("for testing")
	}

	return &state.FakeStateView{
		Miners: map[address.Address]*state.FakeMinerState{
			p.minerAddr: {
				Owner:  p.ownerAddr,
				Worker: p.workerAddr,
			},
		},
	}, nil
}

func (p *mSetWorkerPlumbing) MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method abi.MethodNum, params interface{}) (cid.Cid, chan error, error) {

	if p.msgFail {
		return cid.Cid{}, nil, errors.New("MsgFail")
	}
	return types.EmptyMessagesCID, nil, nil
}

func (p *mSetWorkerPlumbing) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *vm.MessageReceipt) error) error {
	if p.msgWaitFail {
		return errors.New("MsgWaitFail")
	}
	return nil
}

func (p *mSetWorkerPlumbing) ConfigGet(dottedKey string) (interface{}, error) {
	if p.cfgFail {
		return address.Undef, errors.New("ConfigGet failed")
	}
	if dottedKey == "mining.minerAddress" {
		return p.minerAddr, nil
	}
	return address.Undef, fmt.Errorf("unknown config %s", dottedKey)
}

func TestMinerSetWorkerAddress(t *testing.T) {
	tf.UnitTest(t)

	minerOwner := vmaddr.RequireIDAddress(t, 100)
	minerAddr := vmaddr.RequireIDAddress(t, 101)
	workerAddr := vmaddr.RequireIDAddress(t, 102)
	gprice := types.ZeroAttoFIL
	glimit := types.GasUnits(0)

	t.Run("Calling set worker address sets address", func(t *testing.T) {
		plumbing := &mSetWorkerPlumbing{
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
		plumbing *mSetWorkerPlumbing
		error    string
	}{
		{
			name:     "When MessageSend fails, returns the error and does not set worker address",
			plumbing: &mSetWorkerPlumbing{msgFail: true},
			error:    "MsgFail",
		},
		{
			name:     "When ConfigGet fails, returns the error and does not set worker address",
			plumbing: &mSetWorkerPlumbing{cfgFail: true},
			error:    "CfgFail",
		},
		{
			name:     "When MinerGetStatus fails, returns the error and does not set worker address",
			plumbing: &mSetWorkerPlumbing{getStatusFail: true},
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

package porcelain_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	power2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/power"

	"github.com/filecoin-project/venus/internal/app/go-filecoin/plumbing/cfg"
	. "github.com/filecoin-project/venus/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/encoding"
	"github.com/filecoin-project/venus/internal/pkg/state"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/internal/pkg/types"
	"github.com/filecoin-project/venus/internal/pkg/wallet"
)

type minerCreate struct {
	testing *testing.T
	address address.Address
	config  *cfg.Config
	wallet  *wallet.Wallet
	msgCid  cid.Cid
	msgFail bool
}

func (mpc *minerCreate) ConfigGet(dottedPath string) (interface{}, error) {
	return mpc.config.Get(dottedPath)
}

func (mpc *minerCreate) ConfigSet(dottedPath string, paramJSON string) error {
	return mpc.config.Set(dottedPath, paramJSON)
}

func (mpc *minerCreate) MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasBaseFee, gasPremium types.AttoFIL, gasLimit types.Unit, method abi.MethodNum, params interface{}) (cid.Cid, chan error, error) {
	if mpc.msgFail {
		return cid.Cid{}, nil, errors.New("test Error")
	}
	mpc.msgCid = types.CidFromString(mpc.testing, "somecid")

	return mpc.msgCid, nil, nil
}

func (mpc *minerCreate) MessageWait(ctx context.Context, msgCid cid.Cid, lookback uint64, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	assert.Equal(mpc.testing, msgCid, msgCid)
	midAddr, err := address.NewIDAddress(100)
	if err != nil {
		return err
	}

	value, err := encoding.Encode(&power2.CreateMinerReturn{
		IDAddress:     midAddr,
		RobustAddress: mpc.address,
	})
	if err != nil {
		return err
	}

	receipt := types.MessageReceipt{
		ReturnValue: value,
		ExitCode:    exitcode.Ok,
	}
	return cb(nil, nil, &receipt)
}

func (mpc *minerCreate) WalletDefaultAddress() (address.Address, error) {
	return wallet.NewAddress(mpc.wallet, address.SECP256K1)
}

type mStatusPlumbing struct {
	ts                   block.TipSet
	head                 block.TipSetKey
	miner, owner, worker address.Address
}

func (p *mStatusPlumbing) ChainHeadKey() block.TipSetKey {
	return p.head
}

func (p *mStatusPlumbing) ChainTipSet(_ block.TipSetKey) (*block.TipSet, error) {
	return &p.ts, nil
}

func (p *mStatusPlumbing) MinerStateView(baseKey block.TipSetKey) (MinerStateView, error) {
	return &state.FakeStateView{
		Power: &state.NetworkPower{
			RawBytePower:         big.NewInt(4),
			QualityAdjustedPower: big.NewInt(4),
			MinerCount:           0,
			MinPowerMinerCount:   0,
		},
		Miners: map[address.Address]*state.FakeMinerState{
			p.miner: {
				Owner:           p.owner,
				Worker:          p.worker,
				ClaimedRawPower: abi.NewStoragePower(2),
				ClaimedQAPower:  abi.NewStoragePower(2),
			},
		},
	}, nil
}

func TestMinerGetStatus(t *testing.T) {
	tf.UnitTest(t)
	key := block.NewTipSetKey(types.NewCidForTestGetter()())
	ts, err := block.NewTipSet(&block.Block{})
	require.NoError(t, err)

	plumbing := mStatusPlumbing{
		*ts, key, types.RequireIDAddress(t, 1), types.RequireIDAddress(t, 2), types.RequireIDAddress(t, 3),
	}
	status, err := MinerGetStatus(context.Background(), &plumbing, plumbing.miner, key)
	assert.NoError(t, err)
	assert.Equal(t, plumbing.owner, status.OwnerAddress)
	assert.Equal(t, plumbing.worker, status.WorkerAddress)
	assert.Equal(t, "4", status.NetworkQualityAdjustedPower.String())
	assert.Equal(t, "2", status.QualityAdjustedPower.String())
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

func (p *mSetWorkerPlumbing) MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasBaseFee, gasPremium types.AttoFIL, gasLimit types.Unit, method abi.MethodNum, params interface{}) (cid.Cid, chan error, error) {

	if p.msgFail {
		return cid.Cid{}, nil, errors.New("MsgFail")
	}
	return types.EmptyMessagesCID, nil, nil
}

func (p *mSetWorkerPlumbing) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
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

	minerOwner := types.RequireIDAddress(t, 100)
	minerAddr := types.RequireIDAddress(t, 101)
	workerAddr := types.RequireIDAddress(t, 102)
	baseFee := types.ZeroAttoFIL
	gasPremium := types.ZeroAttoFIL
	glimit := types.NewGas(0)

	t.Run("Calling set worker address sets address", func(t *testing.T) {
		plumbing := &mSetWorkerPlumbing{
			workerAddr: workerAddr,
			ownerAddr:  minerOwner,
			minerAddr:  minerAddr,
		}

		_, err := MinerSetWorkerAddress(context.Background(), plumbing, workerAddr, baseFee, gasPremium, glimit)
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
			_, err := MinerSetWorkerAddress(context.Background(), test.plumbing, workerAddr, baseFee, gasPremium, glimit)
			assert.Error(t, err, test.error)
			assert.Empty(t, test.plumbing.workerAddr)
		})
	}
}

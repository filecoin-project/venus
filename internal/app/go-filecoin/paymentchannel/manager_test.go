package paymentchannel_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	spect "github.com/filecoin-project/specs-actors/support/testing"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
	paychtest "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel/testing"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

func TestManager_GetPaymentChannelInfo(t *testing.T) {
	tf.UnitTest(t)
	t.Run("returns err if info does not exist", func(t *testing.T) {
		ds := dss.MutexWrap(datastore.NewMapDatastore())
		ctx := context.Background()
		testAPI := paychtest.NewFakePaymentChannelAPI(ctx, t)
		viewer := paychtest.NewFakeStateViewer(t)
		m := NewManager(context.Background(), ds, testAPI, testAPI, viewer)
		res, err := m.GetPaymentChannelInfo(spect.NewIDAddr(t, 1020))
		assert.EqualError(t, err, "No state for /t01020: datastore: key not found")
		assert.Nil(t, res)
	})
}

func TestManager_CreatePaymentChannel(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	viewer := paychtest.NewFakeStateViewer(t)
	balance := abi.NewTokenAmount(301)

	t.Run("happy path", func(t *testing.T) {
		ds := dss.MutexWrap(datastore.NewMapDatastore())
		testAPI := paychtest.NewFakePaymentChannelAPI(ctx, t)
		m := NewManager(context.Background(), ds, testAPI, testAPI, viewer)
		clientAddr, minerAddr, paychUniqueAddr, _ := requireSetupPaymentChannel(t, testAPI, m, balance)
		exists, err := m.ChannelExists(paychUniqueAddr)
		require.NoError(t, err)
		assert.True(t, exists)

		chinfo, err := m.GetPaymentChannelInfo(paychUniqueAddr)
		require.NoError(t, err)
		require.NotNil(t, chinfo)
		expectedChinfo := ChannelInfo{
			NextLane:   0,
			From:       clientAddr,
			To:         minerAddr,
			UniqueAddr: paychUniqueAddr,
		}
		assert.Equal(t, expectedChinfo, *chinfo)
	})
	testCases := []struct {
		name             string
		waitErr, sendErr error
		expErr           string
	}{
		{name: "returns err and does not create channel if Send fails", sendErr: errors.New("sendboom"), expErr: "sendboom"},
		{name: "returns err and does not create channel if Wait fails", waitErr: errors.New("waitboom"), expErr: "waitboom"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ds := dss.MutexWrap(datastore.NewMapDatastore())
			testAPI := paychtest.NewFakePaymentChannelAPI(ctx, t)
			testAPI.MsgSendErr = tc.sendErr
			testAPI.MsgWaitErr = tc.waitErr
			clientAddr := spect.NewIDAddr(t, rand.Uint64())
			minerAddr := spect.NewIDAddr(t, rand.Uint64())
			paych := spect.NewActorAddr(t, "paych")
			blockHeight := uint64(1234)
			m := NewManager(context.Background(), ds, testAPI, testAPI, viewer)

			testAPI.ExpectedMsgCid, testAPI.ExpectedResult = paychtest.GenCreatePaychActorMessage(t, clientAddr, minerAddr, paych, balance, exitcode.Ok, blockHeight)

			_, err := m.CreatePaymentChannel(clientAddr, minerAddr, balance)
			assert.EqualError(t, err, tc.expErr)
		})
	}

	t.Run("errors if payment channel exists", func(t *testing.T) {
		ds := dss.MutexWrap(datastore.NewMapDatastore())
		testAPI := paychtest.NewFakePaymentChannelAPI(ctx, t)
		m := NewManager(context.Background(), ds, testAPI, testAPI, viewer)
		clientAddr, minerAddr, _, _ := requireSetupPaymentChannel(t, testAPI, m, types.ZeroAttoFIL)
		_, err := m.CreatePaymentChannel(clientAddr, minerAddr, balance)
		assert.EqualError(t, err, "payment channel exists for client t0901, miner t0902")
	})
}

func TestManager_AllocateLane(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	ds := dss.MutexWrap(datastore.NewMapDatastore())
	testAPI := paychtest.NewFakePaymentChannelAPI(ctx, t)
	balance := big.NewInt(301)

	viewer := paychtest.NewFakeStateViewer(t)
	m := NewManager(context.Background(), ds, testAPI, testAPI, viewer)
	clientAddr, minerAddr, paychUniqueAddr, _ := requireSetupPaymentChannel(t, testAPI, m, balance)

	t.Run("saves a new lane", func(t *testing.T) {
		lane, err := m.AllocateLane(paychUniqueAddr)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), lane)

		chinfo, err := m.GetPaymentChannelInfo(paychUniqueAddr)
		require.NoError(t, err)
		require.NotNil(t, chinfo)
		expectedChinfo := ChannelInfo{
			NextLane:   1,
			NextNonce:  1,
			From:       clientAddr,
			To:         minerAddr,
			UniqueAddr: paychUniqueAddr,
		}

		assert.Equal(t, expectedChinfo, *chinfo)
	})

	t.Run("errors if update lane doesn't exist", func(t *testing.T) {
		badAddr := spect.NewActorAddr(t, "nonexistent")
		lane, err := m.AllocateLane(badAddr)
		expErr := fmt.Sprintf("No state for /%s", badAddr.String())
		assert.EqualError(t, err, expErr)
		assert.Zero(t, lane)
	})
}

// AddVoucherToChannel is called by a retrieval client
func TestManager_AddVoucherToChannel(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	amt := big.NewInt(300)
	balance := big.NewInt(301)
	sig := crypto.Signature{Type: crypto.SigTypeSecp256k1, Data: []byte("doesntmatter")}
	root := shared_testutil.GenerateCids(1)[0]

	v := paych.SignedVoucher{
		Nonce:          2,
		TimeLockMax:    abi.ChainEpoch(12345),
		TimeLockMin:    abi.ChainEpoch(12346),
		Amount:         amt,
		Signature:      &sig,
		SecretPreimage: []uint8{},
	}
	newV := v
	newV.Amount = abi.NewTokenAmount(500)

	testAPI := paychtest.NewFakePaymentChannelAPI(ctx, t)

	t.Run("happy path", func(t *testing.T) {
		ds := dss.MutexWrap(datastore.NewMapDatastore())
		viewer := paychtest.NewFakeStateViewer(t)
		manager := NewManager(context.Background(), ds, testAPI, testAPI, viewer)
		clientAddr, minerAddr, paychUniqueAddr, _ := requireSetupPaymentChannel(t, testAPI, manager, balance)
		lane, err := manager.AllocateLane(paychUniqueAddr)
		require.NoError(t, err)
		v.Lane = lane
		testAPI.ExpectedMsgCid, testAPI.ExpectedResult = paychtest.GenCreatePaychActorMessage(t, clientAddr, minerAddr, paychUniqueAddr, balance, exitcode.Ok, 42)

		assert.NoError(t, manager.AddVoucherToChannel(paychUniqueAddr, &v))
	})

	t.Run("errors if channel doesn't exist", func(t *testing.T) {
		_, manager := setupViewerManager(ctx, t, root)
		assert.EqualError(t, manager.AddVoucherToChannel(spect.NewActorAddr(t, "not-there"), &v), "No state for /t2bfuuk4wniuwo2tfso3bfar55hf4d6zq4fbcagui: datastore: key not found")
	})

	t.Run("returns error if lane does not exist", func(t *testing.T) {
		ds := dss.MutexWrap(datastore.NewMapDatastore())
		viewer := paychtest.NewFakeStateViewer(t)
		manager := NewManager(context.Background(), ds, testAPI, testAPI, viewer)
		_, _, paychUniqueAddr, _ := requireSetupPaymentChannel(t, testAPI, manager, balance)
		assert.EqualError(t, manager.AddVoucherToChannel(paychUniqueAddr, &v), "lane does not exist 0")
	})
}

// AddVoucher is called by a retrieval provider
func TestManager_AddVoucher(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	paychAddr := spect.NewActorAddr(t, "abcd123")
	paychIDAddr := spect.NewIDAddr(t, 103)
	clientAddr := spect.NewIDAddr(t, 99)
	minerAddr := spect.NewIDAddr(t, 100)

	root := shared_testutil.GenerateCids(1)[0]
	proof := []byte("proof")
	amt := big.NewInt(300)
	sig := crypto.Signature{Type: crypto.SigTypeSecp256k1, Data: []byte("doesntmatter")}
	v := paych.SignedVoucher{
		Nonce:          2,
		TimeLockMax:    abi.ChainEpoch(12345),
		TimeLockMin:    abi.ChainEpoch(12346),
		Lane:           0,
		Amount:         amt,
		Signature:      &sig,
		SecretPreimage: []uint8{},
	}

	tsk := block.NewTipSetKey(root)
	tok, err := encoding.Encode(tsk)
	require.NoError(t, err)

	t.Run("Adding a valid voucher creates a payment channel info and saves the voucher", func(t *testing.T) {
		testAPI := paychtest.NewFakePaymentChannelAPI(ctx, t)
		ds := dss.MutexWrap(datastore.NewMapDatastore())
		viewer := paychtest.NewFakeStateViewer(t)
		manager := NewManager(context.Background(), ds, testAPI, testAPI, viewer)
		viewer.AddActorWithState(paychAddr, clientAddr, minerAddr, address.Undef)

		increment := int64(10)
		// increment voucher amount by 10, expect 10
		numVouchers := 3
		for i := 0; i < numVouchers; i++ {
			newV := v
			newV.Amount = abi.NewTokenAmount(increment * int64(i+1))
			resAmt, err := manager.AddVoucher(paychAddr, &newV, proof, abi.NewTokenAmount(increment), tok)
			require.NoError(t, err)
			assert.True(t, resAmt.Equals(abi.NewTokenAmount(increment)))
		}
		has, err := manager.ChannelExists(paychAddr)
		require.NoError(t, err)
		assert.True(t, has)
		chinfo, err := manager.GetPaymentChannelInfo(paychAddr)
		require.NoError(t, err)
		require.NotNil(t, chinfo)
		assert.Len(t, chinfo.Vouchers, numVouchers)
		assert.Equal(t, abi.NewTokenAmount(30), chinfo.Vouchers[2].Voucher.Amount)
	})

	t.Run("returns error if we try to save the same voucher", func(t *testing.T) {
		viewer, manager := setupViewerManager(ctx, t, root)
		viewer.AddActorWithState(paychAddr, clientAddr, minerAddr, paychIDAddr)
		resAmt, err := manager.AddVoucher(paychAddr, &v, []byte("porkchops"), abi.NewTokenAmount(1), tok)
		require.NoError(t, err)
		assert.Equal(t, amt, resAmt)

		resAmt, err = manager.AddVoucher(paychAddr, &v, []byte("porkchops"), abi.NewTokenAmount(1), tok)
		assert.EqualError(t, err, "voucher already saved")
		assert.Equal(t, abi.NewTokenAmount(0), resAmt)
	})

	t.Run("returns error if marshaling fails", func(t *testing.T) {
		viewer, manager := setupViewerManager(ctx, t, root)
		viewer.AddActorWithState(paychAddr, clientAddr, address.Undef, address.Undef)
		resAmt, err := manager.AddVoucher(paychAddr, &v, []byte("applesauce"), abi.NewTokenAmount(1), tok)
		assert.EqualError(t, err, "cannot marshal undefined address")
		assert.Equal(t, abi.NewTokenAmount(0), resAmt)
	})

	t.Run("returns error if cannot get actor state/parties", func(t *testing.T) {
		viewer, manager := setupViewerManager(ctx, t, root)
		viewer.AddActorWithState(paychAddr, clientAddr, minerAddr, paychIDAddr)
		viewer.PaychActorPartiesErr = errors.New("boom")
		resAmt, err := manager.AddVoucher(paychAddr, &v, []byte("porkchops"), abi.NewTokenAmount(1), tok)
		assert.EqualError(t, err, "boom")
		assert.Equal(t, abi.NewTokenAmount(0), resAmt)
	})

	t.Run("returns error if voucher amount is insufficient", func(t *testing.T) {
		viewer, manager := setupViewerManager(ctx, t, root)

		viewer.AddActorWithState(paychAddr, clientAddr, minerAddr, paychIDAddr)
		resAmt, err := manager.AddVoucher(paychAddr, &v, []byte("porkchops"), abi.NewTokenAmount(1), tok)
		require.NoError(t, err)
		_, err = manager.AllocateLane(paychAddr)
		require.NoError(t, err)

		// newV.Amount - v.Amount (300) must be > expected (100), or returns error
		amounts := []uint64{10, 300, 399}
		for _, amt := range amounts {
			newV := v
			newV.Amount = types.NewAttoFILFromFIL(amt)
			resAmt, err = manager.AddVoucher(paychAddr, &newV, []byte("porkchops"), abi.NewTokenAmount(1), tok)
			assert.EqualError(t, err, "voucher amount insufficient")
			assert.Equal(t, abi.NewTokenAmount(0), resAmt)
		}
	})

}

func TestManager_GetMinerWorker(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	minerAddr := spect.NewIDAddr(t, 100)
	minerWorkerAddr := spect.NewIDAddr(t, 101)
	root := shared_testutil.GenerateCids(1)[0]
	viewer, manager := setupViewerManager(ctx, t, root)

	tsk := block.NewTipSetKey(root)
	tok, err := encoding.Encode(tsk)
	require.NoError(t, err)

	t.Run("happy path", func(t *testing.T) {
		viewer.AddMinerWithState(minerAddr, minerWorkerAddr)
		res, err := manager.GetMinerWorkerAddress(ctx, minerAddr, tok)
		assert.NoError(t, err)
		assert.Equal(t, minerWorkerAddr, res)
	})

	t.Run("returns error if getting control addr fails", func(t *testing.T) {
		viewer.AddMinerWithState(minerAddr, minerWorkerAddr)
		viewer.MinerControlErr = errors.New("boom")
		_, err := manager.GetMinerWorkerAddress(ctx, minerAddr, tok)
		assert.EqualError(t, err, "boom")
	})
}

func setupViewerManager(ctx context.Context, t *testing.T, root cid.Cid) (*paychtest.FakeStateViewer, *Manager) {
	testAPI := paychtest.NewFakePaymentChannelAPI(ctx, t)
	ds := dss.MutexWrap(datastore.NewMapDatastore())
	viewer := paychtest.NewFakeStateViewer(t)
	return viewer, NewManager(context.Background(), ds, testAPI, testAPI, viewer)
}

func requireSetupPaymentChannel(t *testing.T, testAPI *paychtest.FakePaymentChannelAPI, m *Manager, balance abi.TokenAmount) (address.Address, address.Address, address.Address, uint64) {

	clientAddr := spect.NewIDAddr(t, 901)
	minerAddr := spect.NewIDAddr(t, 902)
	paychUniqueAddr := spect.NewActorAddr(t, "abcd123")
	blockHeight := uint64(1234)

	testAPI.ExpectedMsgCid, testAPI.ExpectedResult = paychtest.GenCreatePaychActorMessage(t, clientAddr, minerAddr, paychUniqueAddr, balance, exitcode.Ok, blockHeight)

	addr, err := m.CreatePaymentChannel(clientAddr, minerAddr, balance)
	require.NoError(t, err)
	require.Equal(t, addr, paychUniqueAddr)
	assert.True(t, testAPI.ExpectedMsgCid.Equals(testAPI.ActualWaitCid))
	return clientAddr, minerAddr, paychUniqueAddr, blockHeight
}

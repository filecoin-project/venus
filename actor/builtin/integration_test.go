package builtin_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

func TestVerifyPieceInclusionInRedeem(t *testing.T) {
	tf.UnitTest(t)

	var mockSigner, _ = types.NewMockSignersAndKeyInfo(10)

	ctx := context.Background()
	payer := mockSigner.Addresses[0]
	addrGetter := address.NewForTestGetter()
	target := addrGetter()
	defaultValidAt := types.NewBlockHeight(uint64(0))
	_, st, vms := requireGenesis(ctx, t, target)

	minerAddr := addrGetter()
	sectorID := uint64(123)
	commP := []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	commD := []byte{0, 1, 2, 3, 4, 5, 6, 7, 0, 1, 2, 3, 4, 5, 6, 7, 0, 1, 2, 3, 4, 5, 6, 7, 0, 1, 2, 3, 4, 5, 6, 7}
	lastPoSt := types.NewBlockHeight(10)

	require.NoError(t, createMinerWithCommitment(ctx, st, vms, minerAddr, sectorID, commD, lastPoSt))

	payerActor := th.RequireNewAccountActor(require.New(t), types.NewAttoFILFromFIL(50000))
	state.MustSetActor(st, payer, payerActor)

	channelID := establishChannel(st, vms, payer, target, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(20000))

	t.Run("Voucher with piece inclusion condition and correct proof succeeds", func(t *testing.T) {
		// create voucher with piece inclusion condition
		condition := &types.Predicate{To: minerAddr, Method: "verifyPieceInclusion", Params: []interface{}{commP}}

		amt := types.NewAttoFILFromFIL(100)
		sig, err := paymentbroker.SignVoucher(channelID, amt, defaultValidAt, payer, condition, mockSigner)
		require.NoError(t, err)
		signature := ([]byte)(sig)

		// make redeem request with correct sector id and PIP
		// TODO: this pip is very fake
		pip := []byte{}
		pip = append(pip, commP[:]...)
		pip = append(pip, commD[:]...)
		suppliedParams := []interface{}{sectorID, pip}
		pdata := core.MustConvertParams(payer, channelID, amt, types.NewBlockHeight(0), condition, signature, suppliedParams)
		msg := types.NewMessage(target, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "redeem", pdata)

		appResult, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))

		require.NoError(t, err)
		require.NoError(t, appResult.ExecutionError)
	})

	t.Run("Voucher with piece inclusion condition and wrong miner fails", func(t *testing.T) {
		differentMinerAddr := addrGetter()
		differentMinerActor := miner.NewActor()
		storage := vms.NewStorage(differentMinerAddr, differentMinerActor)
		require.NoError(t, (&miner.Actor{}).InitializeState(storage, &miner.State{}))
		require.NoError(t, st.SetActor(ctx, differentMinerAddr, differentMinerActor))

		// create voucher with piece inclusion condition
		condition := &types.Predicate{To: differentMinerAddr, Method: "verifyPieceInclusion", Params: []interface{}{commP}}

		amt := types.NewAttoFILFromFIL(100)
		sig, err := paymentbroker.SignVoucher(channelID, amt, defaultValidAt, payer, condition, mockSigner)
		require.NoError(t, err)
		signature := ([]byte)(sig)

		// make redeem request with correct sector id and PIP
		// TODO: this pip is very fake
		pip := []byte{}
		pip = append(pip, commP[:]...)
		pip = append(pip, commD[:]...)
		suppliedParams := []interface{}{sectorID, pip}
		pdata := core.MustConvertParams(payer, channelID, amt, types.NewBlockHeight(0), condition, signature, suppliedParams)
		msg := types.NewMessage(target, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "redeem", pdata)

		appResult, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))

		require.NoError(t, err)
		require.Error(t, appResult.ExecutionError)
		require.Contains(t, appResult.ExecutionError.Error(), "failed to validate voucher condition: sector not committed")
	})

	t.Run("Voucher with piece inclusion condition and incorrect PIP fails", func(t *testing.T) {
		// create voucher with piece inclusion condition
		condition := &types.Predicate{To: minerAddr, Method: "verifyPieceInclusion", Params: []interface{}{commP}}

		amt := types.NewAttoFILFromFIL(100)
		sig, err := paymentbroker.SignVoucher(channelID, amt, defaultValidAt, payer, condition, mockSigner)
		require.NoError(t, err)
		signature := ([]byte)(sig)

		// make redeem request with correct sector id and PIP
		// TODO: this pip is very fake
		pip := []byte{}
		pip = append(pip, commP[:]...)
		pip = append(pip, commD[:]...)
		pip[12]++ // Make PIP wrong
		suppliedParams := []interface{}{sectorID, pip}
		pdata := core.MustConvertParams(payer, channelID, amt, types.NewBlockHeight(0), condition, signature, suppliedParams)
		msg := types.NewMessage(target, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "redeem", pdata)

		appResult, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))

		require.NoError(t, err)
		require.Error(t, appResult.ExecutionError)
		require.Contains(t, appResult.ExecutionError.Error(), "failed to validate voucher condition: invalid inclusion proof")
	})
}

func createMinerWithCommitment(ctx context.Context, st state.Tree, vms vm.StorageMap, minerAddr address.Address, sectorID uint64, commD []byte, lastPoSt *types.BlockHeight) error {
	minerActor := miner.NewActor()
	storage := vms.NewStorage(minerAddr, minerActor)

	commitments := map[string]types.Commitments{}
	commD32 := [32]byte{}
	copy(commD32[:], commD)
	sectorIDstr := strconv.FormatUint(sectorID, 10)
	commitments[sectorIDstr] = types.Commitments{CommD: commD32, CommR: [32]byte{}, CommRStar: [32]byte{}}
	minerState := &miner.State{
		SectorCommitments: commitments,
		LastPoSt:          lastPoSt,
	}
	executableActor := miner.Actor{}
	if err := executableActor.InitializeState(storage, minerState); err != nil {
		return err
	}
	return st.SetActor(ctx, minerAddr, minerActor)
}

func requireGenesis(ctx context.Context, t *testing.T, targetAddresses ...address.Address) (*hamt.CborIpldStore, state.Tree, vm.StorageMap) {
	require := require.New(t)

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := vm.NewStorageMap(bs)

	cst := hamt.NewCborStore()
	blk, err := consensus.DefaultGenesis(cst, bs)
	require.NoError(err)

	st, err := state.LoadStateTree(ctx, cst, blk.StateRoot, builtin.Actors)
	require.NoError(err)

	for _, addr := range targetAddresses {
		targetActor := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(0))
		st.SetActor(ctx, addr, targetActor)
	}

	return cst, st, vms
}

func establishChannel(st state.Tree, vms vm.StorageMap, from address.Address, target address.Address, nonce uint64, amt *types.AttoFIL, eol *types.BlockHeight) *types.ChannelID {
	pdata := core.MustConvertParams(target, eol)
	msg := types.NewMessage(from, address.PaymentBrokerAddress, nonce, amt, "createChannel", pdata)
	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
	if err != nil {
		panic(err)
	}

	if result.ExecutionError != nil {
		panic(result.ExecutionError)
	}

	channelID := types.NewChannelIDFromBytes(result.Receipt.Return[0])
	return channelID
}

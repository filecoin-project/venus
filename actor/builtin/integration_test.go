package builtin_test

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor"
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

	minerActor := actor.NewActor(types.MinerActorCodeCid, types.ZeroAttoFIL)
	storage := vms.NewStorage(address.TestAddress, minerActor)
	minerState := miner.State{}
	require.NoError(t, miner.Actor{}.InitializeState(storage, minerState))
	require.NoError(t, st.SetActor(ctx, address.TestAddress, minerActor))

	payerActor := th.RequireNewAccountActor(require.New(t), types.NewAttoFILFromFIL(50000))
	state.MustSetActor(st, payer, payerActor)

	channelID := establishChannel(ctx, st, vms, payer, target, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(20000))

	toAddress := addrGetter()

	t.Run("Redeem should succeed if condition is met", func(t *testing.T) {
		condition := &types.Predicate{To: toAddress, Method: "checkParams", Params: []interface{}{addrGetter(), uint64(6)}}

		amt := types.NewAttoFILFromFIL(100)
		sig, err := paymentbroker.SignVoucher(channelID, amt, defaultValidAt, payer, condition, mockSigner)
		require.NoError(t, err)
		signature := ([]byte)(sig)

		pdata := core.MustConvertParams(payer, channelID, amt, types.NewBlockHeight(0), condition, signature, []interface{}{types.NewBlockHeight(43)})
		msg := types.NewMessage(target, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "redeem", pdata)

		appResult, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))

		require.NoError(t, err)
		require.NoError(t, appResult.ExecutionError)
	})
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

func establishChannel(ctx context.Context, st state.Tree, vms vm.StorageMap, from address.Address, target address.Address, nonce uint64, amt *types.AttoFIL, eol *types.BlockHeight) *types.ChannelID {
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

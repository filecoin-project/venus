package builtin_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-core/peer"

	sbtesting "github.com/filecoin-project/go-filecoin/internal/pkg/sectorbuilder/testing"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storagemap"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
	"github.com/filecoin-project/go-sectorbuilder"

	"github.com/stretchr/testify/require"
)

func TestVerifyPieceInclusionInRedeem(t *testing.T) {
	tf.FunctionalTest(t)

	ctx := context.Background()
	addrGetter := address.NewForTestGetter()
	amt := types.NewAttoFILFromFIL(100)
	blockHeight := types.NewBlockHeight(0)

	// Make a payment target who will receive the funds
	target := addrGetter()
	defaultValidAt := types.NewBlockHeight(uint64(0))

	// Establish our state
	_, st, vms := requireGenesis(ctx, t, target)

	// if piece consumes all available space in the staged sector, sealing will be triggered
	pieceSize := go_sectorbuilder.GetMaxUserBytesPerStagedSector(types.OneKiBSectorSize.Uint64())
	pieceData := make([]byte, pieceSize)

	sbh := sbtesting.NewBuilder(t).Build()
	defer sbh.Close()

	_, _, err := sbh.AddPiece(ctx, pieceData)
	require.NoError(t, err)

	sealResults := sbh.SectorBuilder.SectorSealResults()

	// wait for sector to be sealed
	sealedSectorResult := <-sealResults
	require.NoError(t, sealedSectorResult.SealingErr)

	sectorMetadata := sealedSectorResult.SealingResult
	require.Len(t, sectorMetadata.Pieces, 1)

	// Create a miner actor with fake commitments
	minerAddr := addrGetter()
	sectorID := sectorMetadata.SectorID
	piece := sectorMetadata.Pieces[0]
	pip := piece.InclusionProof
	commD := sectorMetadata.CommD[:]
	commP := piece.CommP[:]
	provingPeriodEnd := types.NewBlockHeight(10)
	require.NoError(t, createStorageMinerWithCommitment(ctx, st, vms, minerAddr, sectorID, commD, provingPeriodEnd))

	// Create the payer actor
	var mockSigner, _ = types.NewMockSignersAndKeyInfo(10)
	payer := mockSigner.Addresses[0]
	payerActor := th.RequireNewAccountActor(t, types.NewAttoFILFromFIL(50000))
	state.MustSetActor(st, payer, payerActor)

	// Create a payment channel from payer -> target
	channelID := establishChannel(st, vms, payer, target, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(20000))

	makeCondition := func() *types.Predicate {
		return &types.Predicate{To: minerAddr, Method: miner.VerifyPieceInclusion, Params: []interface{}{commP, pieceSize}}
	}

	makeAndSignVoucher := func(condition *types.Predicate) []byte {
		sig, err := paymentbroker.SignVoucher(channelID, amt, defaultValidAt, payer, condition, mockSigner)
		require.NoError(t, err)
		signature := ([]byte)(sig)

		return signature
	}

	makeRedeemMsg := func(condition *types.Predicate, sectorID uint64, pip []byte, signature []byte) *types.UnsignedMessage {
		suppliedParams := []interface{}{sectorID, pip}
		pdata := abi.MustConvertParams(payer, channelID, amt, types.NewBlockHeight(0), condition, signature, suppliedParams)
		return types.NewUnsignedMessage(target, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), paymentbroker.Redeem, pdata)
	}

	t.Run("Voucher with piece inclusion condition and correct proof succeeds", func(t *testing.T) {
		condition := makeCondition()
		signature := makeAndSignVoucher(condition)
		msg := makeRedeemMsg(condition, sectorID, pip, signature)
		appResult, err := th.ApplyTestMessage(st, vms, msg, blockHeight)

		require.NoError(t, err)
		require.NoError(t, appResult.ExecutionError)
	})

	t.Run("Voucher with piece inclusion condition and wrong address fails", func(t *testing.T) {
		condition := makeCondition()

		condition.To = addrGetter()

		signature := makeAndSignVoucher(condition)
		msg := makeRedeemMsg(condition, sectorID, pip, signature)
		appResult, err := th.ApplyTestMessage(st, vms, msg, blockHeight)

		require.NoError(t, err)
		require.Error(t, appResult.ExecutionError)
		require.Contains(t, appResult.ExecutionError.Error(), "failed to validate voucher condition: actor code not found")
	})

	t.Run("Voucher with piece inclusion condition and incorrect PIP fails", func(t *testing.T) {
		condition := makeCondition()
		signature := makeAndSignVoucher(condition)

		badPip := []byte{8, 3, 3, 5, 6, 7, 8}

		msg := makeRedeemMsg(condition, sectorID, badPip, signature)
		appResult, err := th.ApplyTestMessage(st, vms, msg, blockHeight)

		require.NoError(t, err)
		require.Error(t, appResult.ExecutionError)
		require.Contains(t, appResult.ExecutionError.Error(), "failed to verify piece inclusion proof")
	})

	t.Run("Voucher with piece inclusion condition and wrong sectorID fails", func(t *testing.T) {
		condition := makeCondition()
		signature := makeAndSignVoucher(condition)

		badSectorID := sectorID + 1

		msg := makeRedeemMsg(condition, badSectorID, pip, signature)
		appResult, err := th.ApplyTestMessage(st, vms, msg, blockHeight)

		require.NoError(t, err)
		require.Error(t, appResult.ExecutionError)
		require.Contains(t, appResult.ExecutionError.Error(), "failed to validate voucher condition: sector not committed")
	})

	t.Run("Voucher with piece inclusion condition and slashable miner", func(t *testing.T) {
		condition := makeCondition()
		signature := makeAndSignVoucher(condition)
		msg := makeRedeemMsg(condition, sectorID, pip, signature)

		blockHeightThatExpiresLastPoSt := types.NewBlockHeight(100000)

		appResult, err := th.ApplyTestMessage(st, vms, msg, blockHeightThatExpiresLastPoSt)

		require.NoError(t, err)
		require.Error(t, appResult.ExecutionError)
		require.Contains(t, appResult.ExecutionError.Error(), "miner is tardy")
	})

	t.Run("Signed voucher cannot be altered", func(t *testing.T) {
		condition := makeCondition()
		signature := makeAndSignVoucher(condition)

		condition.Method = types.MethodID(8272782)

		msg := makeRedeemMsg(condition, sectorID, pip, signature)
		appResult, err := th.ApplyTestMessage(st, vms, msg, blockHeight)

		require.NoError(t, err)
		require.Error(t, appResult.ExecutionError)
		require.Contains(t, appResult.ExecutionError.Error(), "signature failed to validate")
	})
}

func createStorageMinerWithCommitment(ctx context.Context, st state.Tree, vms storagemap.StorageMap, minerAddr address.Address, sectorID uint64, commD []byte, provingPeriodEnd *types.BlockHeight) error {
	minerActor := miner.NewActor()
	storage := vms.NewStorage(minerAddr, minerActor)

	commitments := map[string]types.Commitments{}
	commD32 := [32]byte{}
	copy(commD32[:], commD)
	sectorIDstr := strconv.FormatUint(sectorID, 10)
	commitments[sectorIDstr] = types.Commitments{CommD: commD32, CommR: [32]byte{}, CommRStar: [32]byte{}}

	minerState := miner.NewState(address.TestAddress, address.TestAddress, peer.ID(""), types.OneKiBSectorSize)
	minerState.SectorCommitments = commitments
	minerState.ProvingPeriodEnd = provingPeriodEnd

	executableActor := miner.Actor{}
	if err := executableActor.InitializeState(storage, minerState); err != nil {
		return err
	}
	return st.SetActor(ctx, minerAddr, minerActor)
}

func requireGenesis(ctx context.Context, t *testing.T, targetAddresses ...address.Address) (*hamt.CborIpldStore, state.Tree, storagemap.StorageMap) {
	require := require.New(t)

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := storagemap.NewStorageMap(bs)

	cst := hamt.NewCborStore()
	blk, err := th.DefaultGenesis(cst, bs)
	require.NoError(err)

	st, err := state.NewTreeLoader().LoadStateTree(ctx, cst, blk.StateRoot)
	require.NoError(err)

	for _, addr := range targetAddresses {
		targetActor := th.RequireNewAccountActor(t, types.NewAttoFILFromFIL(0))
		st.SetActor(ctx, addr, targetActor)
	}

	return cst, st, vms
}

func establishChannel(st state.Tree, vms storagemap.StorageMap, from address.Address, target address.Address, nonce uint64, amt types.AttoFIL, eol *types.BlockHeight) *types.ChannelID {
	pdata := abi.MustConvertParams(target, eol)
	msg := types.NewUnsignedMessage(from, address.PaymentBrokerAddress, nonce, amt, paymentbroker.CreateChannel, pdata)
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

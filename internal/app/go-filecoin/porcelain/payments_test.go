package porcelain_test

import (
	"context"
	"errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-ipld-cbor"
	"math/big"
	"testing"

	. "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	startingBlock   = 77
	channelID       = 4
	paymentInterval = uint64(5)
)

type paymentsTestPlumbing struct {
	height uint64
	msgCid cid.Cid

	messageSend  func(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method types.MethodID, params ...interface{}) (cid.Cid, chan error, error)
	messageWait  func(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error
	messageQuery func(ctx context.Context, optFrom, to address.Address, method types.MethodID, params ...interface{}) ([][]byte, error)
}

func newTestCreatePaymentsPlumbing() *paymentsTestPlumbing {
	var payer address.Address
	var target address.Address
	channelID := types.NewChannelID(channelID)
	cidGetter := types.NewCidForTestGetter()
	msgCid := cidGetter()
	return &paymentsTestPlumbing{
		msgCid: msgCid,
		height: startingBlock,
		messageSend: func(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method types.MethodID, params ...interface{}) (cid.Cid, chan error, error) {
			payer = from
			target = params[0].(address.Address)
			return msgCid, nil, nil
		},
		messageWait: func(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
			return cb(nil, nil, &types.MessageReceipt{
				ExitCode:   uint8(0),
				Return:     [][]byte{channelID.Bytes()},
				GasAttoFIL: types.NewAttoFILFromFIL(9),
			})
		},
		messageQuery: func(ctx context.Context, optFrom, to address.Address, method types.MethodID, params ...interface{}) ([][]byte, error) {
			voucher := &types.PaymentVoucher{
				Channel:   *channelID,
				Payer:     payer,
				Target:    target,
				Amount:    params[1].(types.AttoFIL),
				ValidAt:   *params[2].(*types.BlockHeight),
				Condition: params[3].(*types.Predicate),
			}
			voucherBytes, err := actor.MarshalStorage(voucher)
			if err != nil {
				panic(err)
			}
			return [][]byte{voucherBytes}, nil
		},
	}
}

func (ptp *paymentsTestPlumbing) MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method types.MethodID, params ...interface{}) (cid.Cid, chan error, error) {
	return ptp.messageSend(ctx, from, to, value, gasPrice, gasLimit, method, params...)
}

func (ptp *paymentsTestPlumbing) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	return ptp.messageWait(ctx, msgCid, cb)
}

func (ptp *paymentsTestPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, _ block.TipSetKey, params ...interface{}) ([][]byte, error) {
	return ptp.messageQuery(ctx, optFrom, to, method, params...)
}

func (ptp *paymentsTestPlumbing) ChainTipSet(_ block.TipSetKey) (block.TipSet, error) {
	return block.NewTipSet(&block.Block{Height: types.Uint64(ptp.height)})
}

func (ptp *paymentsTestPlumbing) ChainHeadKey() block.TipSetKey {
	return block.NewTipSetKey()
}

func (ptp *paymentsTestPlumbing) SignBytes(data []byte, addr address.Address) (types.Signature, error) {
	return []byte("signature"), nil
}

func validPaymentsConfig() CreatePaymentsParams {
	addresses := address.NewForTestGetter()
	from := addresses()
	to := addresses()
	minerAddress := addresses()
	var commP types.CommP
	copy(commP[:], []byte("commitment"))

	return CreatePaymentsParams{
		From:            from,
		To:              to,
		Value:           types.NewAttoFILFromFIL(93),
		Duration:        50,
		MinerAddress:    minerAddress,
		CommP:           commP,
		PieceSize:       types.NewBytesAmount(127),
		PaymentInterval: paymentInterval,
		ChannelExpiry:   *types.NewBlockHeight(500),
		GasPrice:        types.NewAttoFILFromFIL(3),
		GasLimit:        types.NewGasUnits(300),
	}
}

func TestCreatePayments(t *testing.T) {
	tf.UnitTest(t)

	successPlumbing := newTestCreatePaymentsPlumbing()

	t.Run("Creates channel and creates payments", func(t *testing.T) {
		config := validPaymentsConfig()
		paymentResponse, err := CreatePayments(context.Background(), successPlumbing, config)
		require.NoError(t, err)

		// assert we get stats in the responses
		assert.Equal(t, config.From, paymentResponse.From)
		assert.Equal(t, config.To, paymentResponse.To)
		assert.Equal(t, successPlumbing.msgCid, paymentResponse.ChannelMsgCid)
		assert.Equal(t, types.NewChannelID(4), paymentResponse.Channel)
		assert.Equal(t, types.NewAttoFILFromFIL(9), paymentResponse.GasAttoFIL)

		// Assert the vouchers have been properly constructed.
		// ValidAts should start at the current block + the interval, and go up by the interval each time.
		// Amounts should evenly divide the total value.
		require.Len(t, paymentResponse.Vouchers, 10)

		expectedValuePerPayment, ok := types.NewAttoFILFromFILString("9.3")
		require.True(t, ok)

		for i := 0; i < 9; i++ {
			voucher := paymentResponse.Vouchers[i]
			assert.Equal(t, *types.NewChannelID(channelID), voucher.Channel)
			assert.Equal(t, config.From, voucher.Payer)
			assert.Equal(t, config.To, voucher.Target)
			assert.Equal(t, *types.NewBlockHeight(startingBlock).Add(types.NewBlockHeight(config.PaymentInterval * uint64(i+1))), voucher.ValidAt)
			assert.Equal(t, expectedValuePerPayment.MulBigInt(big.NewInt(int64(i+1))), voucher.Amount)

			// assert all vouchers other than the last have a valid condition
			assert.NotNil(t, voucher.Condition)
			assert.Equal(t, config.MinerAddress, voucher.Condition.To)
			assert.Equal(t, miner.VerifyPieceInclusion, voucher.Condition.Method)
			assert.Equal(t, config.CommP[:], voucher.Condition.Params[0])

			// voucher signature should be what is returned by SignBytes
			sig := types.Signature([]byte("signature"))
			assert.Equal(t, sig, voucher.Signature)
		}

		// last payment should be for the full amount and have no condition
		assert.Equal(t, *types.NewChannelID(channelID), paymentResponse.Vouchers[9].Channel)
		assert.Equal(t, config.From, paymentResponse.Vouchers[9].Payer)
		assert.Equal(t, config.To, paymentResponse.Vouchers[9].Target)
		assert.Equal(t, config.Value, paymentResponse.Vouchers[9].Amount)
		assert.Nil(t, paymentResponse.Vouchers[9].Condition)
	})

	t.Run("Payments constructed correctly when paymentInterval does not divide duration", func(t *testing.T) {
		config := validPaymentsConfig()

		// lower duration by two blocks
		config.Duration = 48

		paymentResponse, err := CreatePayments(context.Background(), successPlumbing, config)
		require.NoError(t, err)

		// Value*PaymentInterval/Duration = 93 * 5 / 48 = 9.6875
		expectedValuePerPayment, ok := types.NewAttoFILFromFILString("9.6875")
		require.True(t, ok)

		for i := 0; i < 9; i++ {
			voucher := paymentResponse.Vouchers[i]
			assert.Equal(t, *types.NewBlockHeight(startingBlock).Add(types.NewBlockHeight(config.PaymentInterval * uint64(i+1))), voucher.ValidAt)
			assert.Equal(t, expectedValuePerPayment.MulBigInt(big.NewInt(int64(i+1))), voucher.Amount)

			// assert all vouchers other than the last have a valid condition
			assert.NotNil(t, voucher.Condition)
		}

		// last payment should be for the full amount and have no condition
		assert.Equal(t, config.Value, paymentResponse.Vouchers[9].Amount)
		assert.Nil(t, paymentResponse.Vouchers[9].Condition)
	})

	t.Run("Payments constructed correctly when duration < paymentInterval", func(t *testing.T) {
		config := validPaymentsConfig()

		// lower duration below payment interval (5)
		config.Duration = 3

		paymentResponse, err := CreatePayments(context.Background(), successPlumbing, config)
		require.NoError(t, err)

		require.Len(t, paymentResponse.Vouchers, 1)

		// last payment should be for the full amount and have no condition
		assert.Equal(t, config.Value, paymentResponse.Vouchers[0].Amount)
		assert.Nil(t, paymentResponse.Vouchers[0].Condition)
	})

	t.Run("Validates from", func(t *testing.T) {
		config := validPaymentsConfig()
		config.From = address.Undef
		_, err := CreatePayments(context.Background(), successPlumbing, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "From")
	})

	t.Run("Validates to", func(t *testing.T) {
		config := validPaymentsConfig()
		config.To = address.Undef
		_, err := CreatePayments(context.Background(), successPlumbing, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "To")
	})

	t.Run("Validates payment interval", func(t *testing.T) {
		config := validPaymentsConfig()
		config.PaymentInterval = 0
		_, err := CreatePayments(context.Background(), successPlumbing, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "PaymentInterval")
	})

	t.Run("Validates channel expiry", func(t *testing.T) {
		config := validPaymentsConfig()
		config.ChannelExpiry = *types.NewBlockHeight(startingBlock + config.PaymentInterval*10 - 1)
		_, err := CreatePayments(context.Background(), successPlumbing, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "channel would expire")
	})

	t.Run("Errors creating channel are surfaced", func(t *testing.T) {
		plumbing := newTestCreatePaymentsPlumbing()
		plumbing.messageSend = func(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method types.MethodID, params ...interface{}) (cid.Cid, chan error, error) {
			return cid.Cid{}, nil, errors.New("Error in MessageSend")
		}

		config := validPaymentsConfig()
		_, err := CreatePayments(context.Background(), plumbing, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "MessageSend")
	})

	t.Run("Errors waiting for message are surfaced", func(t *testing.T) {
		plumbing := newTestCreatePaymentsPlumbing()
		plumbing.messageWait = func(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
			return errors.New("Error in MessageWait")
		}

		config := validPaymentsConfig()
		_, err := CreatePayments(context.Background(), plumbing, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "MessageWait")
	})

	t.Run("Errors in create channel response are surfaced", func(t *testing.T) {
		plumbing := newTestCreatePaymentsPlumbing()
		plumbing.messageWait = func(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
			receipt := &types.MessageReceipt{
				ExitCode: 1,
			}
			return cb(nil, nil, receipt)
		}

		config := validPaymentsConfig()
		_, err := CreatePayments(context.Background(), plumbing, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "createChannel failed")
	})

	t.Run("Errors in creating vouchers are surfaced", func(t *testing.T) {
		plumbing := newTestCreatePaymentsPlumbing()
		plumbing.messageQuery = func(ctx context.Context, optFrom, to address.Address, method types.MethodID, params ...interface{}) ([][]byte, error) {
			return nil, errors.New("Errors in MessageQuery")
		}

		config := validPaymentsConfig()
		_, err := CreatePayments(context.Background(), plumbing, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "MessageQuery")
	})
}

func TestValidateStoragePaymentCondition(t *testing.T) {
	pieceSize := types.NewBytesAmount(2828383)
	var commP types.CommP
	copy(commP[:], testhelpers.MakeRandomBytes(len(types.CommP{})))

	makeValidCondition := func() *types.Predicate {
		condition := &types.Predicate{
			To:     address.TestAddress,
			Method: miner.VerifyPieceInclusion,
			Params: []interface{}{
				commP,
				pieceSize,
			},
		}

		// simulate parameter encoding from encoding and decoding to send across the network
		conditionBytes, err := cbornode.DumpObject(condition)
		require.NoError(t, err)
		require.NoError(t, cbornode.DecodeInto(conditionBytes, condition))

		return condition
	}

	t.Run("Accepts the nil condition", func(t *testing.T) {
		err := ValidatePaymentVoucherCondition(context.Background(), nil, address.TestAddress, commP, pieceSize)
		require.NoError(t, err)
	})

	t.Run("Accepts valid condition", func(t *testing.T) {
		condition := makeValidCondition()

		err := ValidatePaymentVoucherCondition(context.Background(), condition, address.TestAddress, commP, pieceSize)
		require.NoError(t, err)
	})

	t.Run("Condition invalid when given wrong method", func(t *testing.T) {
		condition := makeValidCondition()
		condition.Method = types.MethodID(217292)

		err := ValidatePaymentVoucherCondition(context.Background(), condition, address.TestAddress, commP, pieceSize)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "payment condition method")

	})

	t.Run("Condition invalid when given address", func(t *testing.T) {
		condition := makeValidCondition()
		condition.To = address.TestAddress2

		err := ValidatePaymentVoucherCondition(context.Background(), condition, address.TestAddress, commP, pieceSize)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "voucher condition addressed")

	})

	t.Run("Condition invalid when it contains wrong number of parameters", func(t *testing.T) {
		invalidParameters := [][]interface{}{
			nil,
			{},
			{commP},
			{commP, pieceSize, "foo"},
		}

		for _, invalidParam := range invalidParameters {
			condition := makeValidCondition()
			condition.Params = invalidParam

			err := ValidatePaymentVoucherCondition(context.Background(), condition, address.TestAddress, commP, pieceSize)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "2 parameters")
		}
	})

	t.Run("Condition invalid when given wrong commD", func(t *testing.T) {
		condition := makeValidCondition()
		condition.Params[0] = types.CommP{}

		err := ValidatePaymentVoucherCondition(context.Background(), condition, address.TestAddress, commP, pieceSize)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "piece commitment")

		condition.Params[0] = "not event a byte slice"

		err = ValidatePaymentVoucherCondition(context.Background(), condition, address.TestAddress, commP, pieceSize)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "piece commitment")
	})

	t.Run("Condition invalid when given wrong pieceSize", func(t *testing.T) {
		condition := makeValidCondition()
		condition.Params[1] = uint64(3223893)

		err := ValidatePaymentVoucherCondition(context.Background(), condition, address.TestAddress, commP, pieceSize)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "piece size")

		condition.Params[1] = "not even a uint64"

		err = ValidatePaymentVoucherCondition(context.Background(), condition, address.TestAddress, commP, pieceSize)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "piece size")
	})
}

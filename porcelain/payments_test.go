package porcelain_test

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	. "github.com/filecoin-project/go-filecoin/porcelain"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	startingBlock   = 77
	channelID       = 4
	paymentInterval = uint64(5)
)

type paymentsTestPlumbing struct {
	height *types.BlockHeight
	msgCid cid.Cid

	messageSend  func(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error)
	messageWait  func(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error
	messageQuery func(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)
}

func newTestCreatePaymentsPlumbing() *paymentsTestPlumbing {
	var payer address.Address
	var target address.Address
	channelID := types.NewChannelID(channelID)
	cidGetter := types.NewCidForTestGetter()
	msgCid := cidGetter()
	return &paymentsTestPlumbing{
		msgCid: msgCid,
		height: types.NewBlockHeight(startingBlock),
		messageSend: func(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
			payer = from
			target = params[0].(address.Address)
			return msgCid, nil
		},
		messageWait: func(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
			return cb(nil, nil, &types.MessageReceipt{
				ExitCode:   uint8(0),
				Return:     [][]byte{channelID.Bytes()},
				GasAttoFIL: types.NewAttoFILFromFIL(9),
			})
		},
		messageQuery: func(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
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

func (ptp *paymentsTestPlumbing) MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
	return ptp.messageSend(ctx, from, to, value, gasPrice, gasLimit, method, params...)
}

func (ptp *paymentsTestPlumbing) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	return ptp.messageWait(ctx, msgCid, cb)
}

func (ptp *paymentsTestPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
	return ptp.messageQuery(ctx, optFrom, to, method, params...)
}

func (ptp *paymentsTestPlumbing) ChainBlockHeight() (*types.BlockHeight, error) {
	return ptp.height, nil
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
			assert.Equal(t, "verifyPieceInclusion", voucher.Condition.Method)
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
		plumbing.messageSend = func(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
			return cid.Cid{}, errors.New("Error in MessageSend")
		}

		config := validPaymentsConfig()
		_, err := CreatePayments(context.Background(), plumbing, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "MessageSend")
	})

	t.Run("Errors waiting for message are surfaced", func(t *testing.T) {
		plumbing := newTestCreatePaymentsPlumbing()
		plumbing.messageWait = func(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
			return errors.New("Error in MessageWait")
		}

		config := validPaymentsConfig()
		_, err := CreatePayments(context.Background(), plumbing, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "MessageWait")
	})

	t.Run("Errors in create channel response are surfaced", func(t *testing.T) {
		plumbing := newTestCreatePaymentsPlumbing()
		plumbing.messageWait = func(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
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
		plumbing.messageQuery = func(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
			return nil, errors.New("Errors in MessageQuery")
		}

		config := validPaymentsConfig()
		_, err := CreatePayments(context.Background(), plumbing, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "MessageQuery")
	})
}

package porcelain_test

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/exec"
	. "github.com/filecoin-project/go-filecoin/porcelain"
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
	tipSets []*types.TipSet
	msgCid  cid.Cid

	messageSend  func(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error)
	messageWait  func(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error
	messageQuery func(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error)
}

func newTestCreatePaymentsPlumbing() *paymentsTestPlumbing {
	var payer address.Address
	var target address.Address
	channelID := types.NewChannelID(channelID)
	cidGetter := types.NewCidForTestGetter()
	msgCid := cidGetter()
	tipSet, err := types.NewTipSet(&types.Block{
		Nonce:  43,
		Height: startingBlock,
	})
	if err != nil {
		panic("could not create tipset")
	}
	return &paymentsTestPlumbing{
		msgCid:  msgCid,
		tipSets: []*types.TipSet{&tipSet},
		messageSend: func(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
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
		messageQuery: func(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error) {
			voucher := &paymentbroker.PaymentVoucher{
				Channel: *channelID,
				Payer:   payer,
				Target:  target,
				Amount:  *params[1].(*types.AttoFIL),
				ValidAt: *params[2].(*types.BlockHeight),
			}
			voucherBytes, err := actor.MarshalStorage(voucher)
			if err != nil {
				panic(err)
			}
			return [][]byte{voucherBytes}, nil, nil
		},
	}
}

func (ptp *paymentsTestPlumbing) MessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
	return ptp.messageSend(ctx, from, to, value, gasPrice, gasLimit, method, params...)
}

func (ptp *paymentsTestPlumbing) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	return ptp.messageWait(ctx, msgCid, cb)
}

func (ptp *paymentsTestPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error) {
	return ptp.messageQuery(ctx, optFrom, to, method, params...)
}

func (ptp *paymentsTestPlumbing) ChainLs(ctx context.Context) <-chan *chain.BlockHistoryResult {
	out := make(chan *chain.BlockHistoryResult, len(ptp.tipSets))

	go func() {
		defer close(out)

		for _, result := range ptp.tipSets {
			out <- &chain.BlockHistoryResult{
				TipSet: *result,
			}
		}
	}()

	return out
}

func (ptp *paymentsTestPlumbing) SignBytes(data []byte, addr address.Address) (types.Signature, error) {
	return []byte("signature"), nil
}

func validPaymentsConfig() CreatePaymentsParams {
	addresses := address.NewForTestGetter()
	from := addresses()
	to := addresses()

	return CreatePaymentsParams{
		From:            from,
		To:              to,
		Value:           *types.NewAttoFILFromFIL(93),
		Duration:        50,
		PaymentInterval: paymentInterval,
		ChannelExpiry:   *types.NewBlockHeight(500),
		GasPrice:        *types.NewAttoFILFromFIL(3),
		GasLimit:        types.NewGasUnits(300),
	}
}

func TestCreatePayments(t *testing.T) {
	successPlumbing := newTestCreatePaymentsPlumbing()

	t.Run("Creates channel and creates payments", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		config := validPaymentsConfig()
		paymentResponse, err := CreatePayments(context.Background(), successPlumbing, config)
		require.NoError(err)

		// assert we get stats in the responses
		assert.Equal(config.From, paymentResponse.From)
		assert.Equal(config.To, paymentResponse.To)
		assert.Equal(successPlumbing.msgCid, paymentResponse.ChannelMsgCid)
		assert.Equal(types.NewChannelID(4), paymentResponse.Channel)
		assert.Equal(types.NewAttoFILFromFIL(9), paymentResponse.GasAttoFIL)

		// Assert the vouchers have been properly constructed.
		// ValidAts should start at the current block + the interval, and go up by the interval each time.
		// Amounts should evenly divide the total value.
		require.Len(paymentResponse.Vouchers, 10)

		expectedValuePerPayment, ok := types.NewAttoFILFromFILString("9.3")
		require.True(ok)

		for i := 0; i < 9; i++ {
			voucher := paymentResponse.Vouchers[i]
			assert.Equal(*types.NewChannelID(channelID), voucher.Channel)
			assert.Equal(config.From, voucher.Payer)
			assert.Equal(config.To, voucher.Target)
			assert.Equal(*types.NewBlockHeight(startingBlock).Add(types.NewBlockHeight(config.PaymentInterval * uint64(i+1))), voucher.ValidAt)
			assert.Equal(*expectedValuePerPayment.MulBigInt(big.NewInt(int64(i + 1))), voucher.Amount)

			// voucher signature should be what is returned by SignBytes

			sig := types.Signature([]byte("signature"))
			assert.Equal(sig, voucher.Signature)
		}

		// last payment should be for the full amount
		assert.Equal(*types.NewChannelID(channelID), paymentResponse.Vouchers[9].Channel)
		assert.Equal(config.From, paymentResponse.Vouchers[9].Payer)
		assert.Equal(config.To, paymentResponse.Vouchers[9].Target)
		assert.Equal(config.Value, paymentResponse.Vouchers[9].Amount)
	})

	t.Run("Validates from", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		config := validPaymentsConfig()
		config.From = address.Undef
		_, err := CreatePayments(context.Background(), successPlumbing, config)
		require.Error(err)
		assert.Contains(err.Error(), "From")
	})

	t.Run("Validates to", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		config := validPaymentsConfig()
		config.To = address.Undef
		_, err := CreatePayments(context.Background(), successPlumbing, config)
		require.Error(err)
		assert.Contains(err.Error(), "To")
	})

	t.Run("Validates payment interval", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		config := validPaymentsConfig()
		config.PaymentInterval = 0
		_, err := CreatePayments(context.Background(), successPlumbing, config)
		require.Error(err)
		assert.Contains(err.Error(), "PaymentInterval")
	})

	t.Run("Validates channel expiry", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		config := validPaymentsConfig()
		config.ChannelExpiry = *types.NewBlockHeight(startingBlock + config.PaymentInterval*10 - 1)
		_, err := CreatePayments(context.Background(), successPlumbing, config)
		require.Error(err)
		assert.Contains(err.Error(), "channel would expire")
	})

	t.Run("Errors retrieving block height are surfaced", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newTestCreatePaymentsPlumbing()
		plumbing.tipSets = []*types.TipSet{}

		config := validPaymentsConfig()
		_, err := CreatePayments(context.Background(), plumbing, config)
		require.Error(err)
		assert.Contains(err.Error(), "block height")

		plumbing = newTestCreatePaymentsPlumbing()
		ts := types.TipSet{}
		plumbing.tipSets = []*types.TipSet{&ts}

		config = validPaymentsConfig()
		_, err = CreatePayments(context.Background(), plumbing, config)
		require.Error(err)
		assert.Contains(err.Error(), "block height")
	})

	t.Run("Errors creating channel are surfaced", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newTestCreatePaymentsPlumbing()
		plumbing.messageSend = func(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
			return cid.Cid{}, errors.New("Error in MessageSend")
		}

		config := validPaymentsConfig()
		_, err := CreatePayments(context.Background(), plumbing, config)
		require.Error(err)
		assert.Contains(err.Error(), "MessageSend")
	})

	t.Run("Errors waiting for message are surfaced", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newTestCreatePaymentsPlumbing()
		plumbing.messageWait = func(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
			return errors.New("Error in MessageWait")
		}

		config := validPaymentsConfig()
		_, err := CreatePayments(context.Background(), plumbing, config)
		require.Error(err)
		assert.Contains(err.Error(), "MessageWait")
	})

	t.Run("Errors in create channel response are surfaced", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newTestCreatePaymentsPlumbing()
		plumbing.messageWait = func(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
			receipt := &types.MessageReceipt{
				ExitCode: 1,
			}
			return cb(nil, nil, receipt)
		}

		config := validPaymentsConfig()
		_, err := CreatePayments(context.Background(), plumbing, config)
		require.Error(err)
		assert.Contains(err.Error(), "createChannel failed")
	})

	t.Run("Errors in creating vouchers are surfaced", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		plumbing := newTestCreatePaymentsPlumbing()
		plumbing.messageQuery = func(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error) {
			return nil, nil, errors.New("Errors in MessageQuery")
		}

		config := validPaymentsConfig()
		_, err := CreatePayments(context.Background(), plumbing, config)
		require.Error(err)
		assert.Contains(err.Error(), "MessageQuery")
	})
}

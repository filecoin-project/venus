package porcelain

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/plumbing/chn"
	"github.com/filecoin-project/go-filecoin/types"
)

// cpPlumbing is the subset of the plumbing.API that CreatePayments uses.
type cpPlumbing interface {
	MessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error)
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)
	MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error
	ChainLs(ctx context.Context) <-chan *chn.ChainLsResult
	SignBytes(data []byte, addr address.Address) (types.Signature, error)
}

// CreatePaymentsParams structures all the parameters for the CreatePayments command. All values are required.
// The first payment will be valid at PaymentStart+PaymentInterval. Payment voucher will be created for every
// PaymentInterval after that until PaymentStart+Duration is reached.
// ChannelExpiry is when the channel closes and must be after the final payment is valid.
type CreatePaymentsParams struct {
	// From is the address of the payer.
	From address.Address

	// To is the address of the target of the payments.
	To address.Address

	// Value is the amount of the payment channel that will be opened and the sum of all the payments.
	Value types.AttoFIL

	// Duration is the amount of time (in block height) the payments will cover.
	Duration uint64

	// PaymentInterval is the time between payments (in block height)
	PaymentInterval uint64

	// ChannelExpiry is the time (block height) at which the payment channel will close. It must
	// be greater than the current block height plus Duration.
	ChannelExpiry types.BlockHeight

	// GasPrice is the price of gas to be paid to create the payment channel
	GasPrice types.AttoFIL

	// GasLimit is the maximum amount of gas to be paid creating the payment channel.
	GasLimit types.GasUnits
}

// CreatePaymentsReturn collects relevant stats from the create payments process
type CreatePaymentsReturn struct {
	// CreatePaymentsParams are the parameters given to create the payment
	CreatePaymentsParams

	// Channel is the id of the payment channel
	Channel *types.ChannelID

	// ChannelMsgCid is the id of the message sent to create the payment channel
	ChannelMsgCid cid.Cid

	// GasAttoFIL is the amount spent on gas creating the channel
	GasAttoFIL *types.AttoFIL

	// Vouchers are the payment vouchers created to pay the target at regular intervals.
	Vouchers []*paymentbroker.PaymentVoucher
}

// CreatePayments establishes a payment channel and create multiple payments against it
func CreatePayments(ctx context.Context, plumbing cpPlumbing, config CreatePaymentsParams) (*CreatePaymentsReturn, error) {
	// validate
	if config.From.Empty() {
		return nil, errors.New("From cannot be empty")
	}
	if config.To.Empty() {
		return nil, errors.New("To cannot be empty")
	}
	if config.PaymentInterval < 1 {
		return nil, errors.New("PaymentInterval must be at least 1")
	}

	// get current block height
	currentHeight, err := ChainBlockHeight(ctx, plumbing)
	if err != nil {
		return nil, errors.Wrap(err, "Could not retrieve block height for making payments")
	}

	// validate that channel expiry gives us enough time
	lastPayment := currentHeight.Add(types.NewBlockHeight(config.Duration))
	if config.ChannelExpiry.LessThan(lastPayment) {
		return nil, fmt.Errorf("channel would expire (%s) before last payment is made (%s)", config.ChannelExpiry.String(), lastPayment)
	}

	response := &CreatePaymentsReturn{
		CreatePaymentsParams: config,
	}

	// Create channel
	response.ChannelMsgCid, err = plumbing.MessageSend(ctx,
		config.From,
		address.PaymentBrokerAddress,
		&config.Value,
		config.GasPrice,
		config.GasLimit,
		"createChannel",
		config.To,
		&config.ChannelExpiry)
	if err != nil {
		return response, err
	}

	// wait for response
	err = plumbing.MessageWait(ctx, response.ChannelMsgCid, func(block *types.Block, message *types.SignedMessage, receipt *types.MessageReceipt) error {
		if receipt.ExitCode != 0 {
			return fmt.Errorf("createChannel failed %d", receipt.ExitCode)
		}

		response.Channel = types.NewChannelIDFromBytes(receipt.Return[0])
		response.GasAttoFIL = receipt.GasAttoFIL
		return nil
	})
	if err != nil {
		return response, err
	}

	// compute value per payment. Roughly value/num payments. Exactly ceil(value*interval/duration).
	intervalAsBigInt := big.NewInt(int64(config.PaymentInterval))
	// Convert to AttoFIL, because values have to be the same type.
	durationAsAttoFIL := types.NewAttoFIL(big.NewInt(int64(config.Duration)))
	valuePerPayment := *config.Value.MulBigInt(intervalAsBigInt).DivCeil(durationAsAttoFIL)

	// generate payments
	response.Vouchers = []*paymentbroker.PaymentVoucher{}
	voucherAmount := types.ZeroAttoFIL
	for i := 0; uint64(i+1)*config.PaymentInterval < config.Duration; i++ {
		voucherAmount = voucherAmount.Add(&valuePerPayment)
		if voucherAmount.GreaterThan(&config.Value) {
			voucherAmount = &config.Value
		}

		validAt := currentHeight.Add(types.NewBlockHeight(uint64(i+1) * config.PaymentInterval))
		err = createPayment(ctx, plumbing, response, voucherAmount, validAt)
		if err != nil {
			return response, err
		}
	}

	if voucherAmount.LessThan(&config.Value) {
		validAt := currentHeight.Add(types.NewBlockHeight(config.Duration))
		err = createPayment(ctx, plumbing, response, &config.Value, validAt)
		if err != nil {
			return response, err
		}
	}

	return response, nil
}

func createPayment(ctx context.Context, plumbing cpPlumbing, response *CreatePaymentsReturn, amount *types.AttoFIL, validAt *types.BlockHeight) error {
	ret, err := plumbing.MessageQuery(ctx,
		response.From,
		address.PaymentBrokerAddress,
		"voucher",
		response.Channel,
		amount,
		validAt)
	if err != nil {
		return err
	}

	var voucher paymentbroker.PaymentVoucher
	if err := cbor.DecodeInto(ret[0], &voucher); err != nil {
		return err
	}

	sig, err := paymentbroker.SignVoucher(&voucher.Channel, amount, validAt, voucher.Payer, plumbing)
	if err != nil {
		return err
	}
	voucher.Signature = sig

	response.Vouchers = append(response.Vouchers, &voucher)
	return nil
}

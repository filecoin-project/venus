package porcelain

import (
	"context"
	"fmt"
	"math/big"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

const verifyPieceInclusionMethod = miner.VerifyPieceInclusion

// cpPlumbing is the subset of the plumbing.API that CreatePayments uses.
type cpPlumbing interface {
	ChainHeadKey() block.TipSetKey
	ChainTipSet(key block.TipSetKey) (block.TipSet, error)
	MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, baseKey block.TipSetKey, params ...interface{}) ([][]byte, error)
	MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method types.MethodID, params ...interface{}) (cid.Cid, chan error, error)
	MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error
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

	// MinerAddress is the address of the miner actor representing the payment recipient.
	// Conditions confirming that the miner is storing the client's piece will be directed towards this actor.
	MinerAddress address.Address

	// CommP is the client's data commitment. It will be the basis of piece inclusion conditions added to the payments.
	CommP types.CommP

	// PieceSize represents the size of the user-provided piece, in bytes.
	PieceSize *types.BytesAmount

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
	GasAttoFIL types.AttoFIL

	// Vouchers are the payment vouchers created to pay the target at regular intervals.
	Vouchers []*types.PaymentVoucher
}

// CreatePayments establishes a payment channel and creates multiple payments against it.
//
// Each payment except the last will get a condition that calls verifyPieceInclusion on the recipient's miner
// actor to ensure the storage miner is still storing the file at the time of redemption.
// The last payment does not contain a condition so that the miner may collect payment without posting a
// piece inclusion proof after the storage deal is complete.
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

	// get current head info
	head, err := plumbing.ChainTipSet(plumbing.ChainHeadKey())
	if err != nil {
		return nil, err
	}

	h, err := head.Height()
	if err != nil {
		return nil, errors.Wrap(err, "Could not retrieve block height for making payments")
	}
	currentHeight := types.NewBlockHeight(h)

	// validate that channel expiry gives us enough time
	lastPayment := currentHeight.Add(types.NewBlockHeight(config.Duration))
	if config.ChannelExpiry.LessThan(lastPayment) {
		return nil, fmt.Errorf("channel would expire (%s) before last payment is made (%s)", config.ChannelExpiry.String(), lastPayment)
	}

	response := &CreatePaymentsReturn{
		CreatePaymentsParams: config,
	}

	// Create channel
	response.ChannelMsgCid, _, err = plumbing.MessageSend(ctx,
		config.From,
		address.PaymentBrokerAddress,
		config.Value,
		config.GasPrice,
		config.GasLimit,
		paymentbroker.CreateChannel,
		config.To,
		&config.ChannelExpiry)
	if err != nil {
		return response, err
	}

	// wait for response
	err = plumbing.MessageWait(ctx, response.ChannelMsgCid, func(block *block.Block, message *types.SignedMessage, receipt *types.MessageReceipt) error {
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
	valuePerPayment := config.Value.MulBigInt(intervalAsBigInt).DivCeil(durationAsAttoFIL)

	// condition is a condition that requires that the miner has the client's piece and is currently proving on it
	condition := &types.Predicate{
		To:     config.MinerAddress,
		Method: verifyPieceInclusionMethod,
		Params: []interface{}{config.CommP[:], config.PieceSize},
	}

	// generate payments
	headKey := plumbing.ChainHeadKey()
	response.Vouchers = []*types.PaymentVoucher{}
	voucherAmount := types.ZeroAttoFIL
	for i := 0; uint64(i+1)*config.PaymentInterval < config.Duration; i++ {
		voucherAmount = voucherAmount.Add(valuePerPayment)
		if voucherAmount.GreaterThan(config.Value) {
			voucherAmount = config.Value
		}

		validAt := currentHeight.Add(types.NewBlockHeight(uint64(i+1) * config.PaymentInterval))
		err = createPayment(ctx, plumbing, headKey, response, voucherAmount, validAt, condition)
		if err != nil {
			return response, err
		}
	}

	// create last payment
	validAt := currentHeight.Add(types.NewBlockHeight(config.Duration))
	err = createPayment(ctx, plumbing, headKey, response, config.Value, validAt, nil)
	if err != nil {
		return response, err
	}

	return response, nil
}

// ValidatePaymentVoucherCondition validates that condition of a voucher created for a storage payment meets expectations
func ValidatePaymentVoucherCondition(ctx context.Context, condition *types.Predicate, minerAddr address.Address, commP types.CommP, pieceSize *types.BytesAmount) error {
	// a nil condition is always valid
	if condition == nil {
		return nil
	}

	if condition.To != minerAddr {
		return errors.Errorf("voucher condition addressed to %s, should be %s", condition.To, minerAddr)
	}

	if condition.Method != verifyPieceInclusionMethod {
		return errors.Errorf("payment condition method, %s, should be %s", condition.Method, verifyPieceInclusionMethod)
	}

	if condition.Params == nil || len(condition.Params) != 2 {
		return errors.New("payment condition does not contain exactly 2 parameters")
	}

	var clientCommP types.CommP
	clientCommPBytes, ok := condition.Params[0].([]byte)
	if ok {
		copy(clientCommP[:], clientCommPBytes)
	} else {
		return errors.New("piece commitment is not a CommP")
	}

	if clientCommP != commP {
		return errors.Errorf("piece commitment, [% x] does not match payment condition commitment: [% x]", clientCommP[:], commP[:])
	}

	var clientPieceSize *types.BytesAmount
	clientPieceSizeBytes, ok := condition.Params[1].([]byte)
	if ok {
		clientPieceSize = types.NewBytesAmountFromBytes(clientPieceSizeBytes)
	} else {
		return errors.New("piece size is not a bytes amount")
	}

	if !pieceSize.Equal(clientPieceSize) {
		return errors.Errorf("piece size, %v,  does not match piece size supplied in payment condition: %v", pieceSize, clientPieceSize)
	}

	return nil
}

func createPayment(ctx context.Context, plumbing cpPlumbing, baseKey block.TipSetKey, response *CreatePaymentsReturn, amount types.AttoFIL, validAt *types.BlockHeight, condition *types.Predicate) error {

	ret, err := plumbing.MessageQuery(ctx,
		response.From,
		address.PaymentBrokerAddress,
		paymentbroker.Voucher,
		baseKey,
		response.Channel,
		amount,
		validAt,
		condition,
	)
	if err != nil {
		return err
	}

	var voucher types.PaymentVoucher
	if err := encoding.Decode(ret[0], &voucher); err != nil {
		return err
	}

	sig, err := paymentbroker.SignVoucher(&voucher.Channel, amount, validAt, voucher.Payer, condition, plumbing)
	if err != nil {
		return err
	}
	voucher.Signature = sig

	response.Vouchers = append(response.Vouchers, &voucher)
	return nil
}

package settler

import (
	"context"
	"sync"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/events"

	"github.com/filecoin-project/venus/pkg/paychmgr"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/paych"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("payment-channel-settler")

type API struct {
	events.IEvent
	Settler
}
type PaymentChannelSettler interface {
	check(ts *types.TipSet) (done bool, more bool, err error)
	messageHandler(msg *types.UnsignedMessage, rec *types.MessageReceipt, ts *types.TipSet, curH abi.ChainEpoch) (more bool, err error)
	revertHandler(ctx context.Context, ts *types.TipSet) error
	matcher(msg *types.UnsignedMessage) (matched bool, err error)
}
type paymentChannelSettler struct {
	ctx context.Context
	api Settler
}

func NewPaymentChannelSettler(ctx context.Context, api Settler) PaymentChannelSettler {
	return &paymentChannelSettler{
		ctx: ctx,
		api: api,
	}
}

func (pcs *paymentChannelSettler) check(ts *types.TipSet) (done bool, more bool, err error) {
	return false, true, nil
}

func (pcs *paymentChannelSettler) messageHandler(msg *types.UnsignedMessage, rec *types.MessageReceipt, ts *types.TipSet, curH abi.ChainEpoch) (more bool, err error) {
	// Ignore unsuccessful settle messages
	if rec.ExitCode != 0 {
		return true, nil
	}

	bestByLane, err := paychmgr.BestSpendableByLane(pcs.ctx, pcs.api, msg.To)
	if err != nil {
		return true, err
	}
	var wg sync.WaitGroup
	wg.Add(len(bestByLane))
	for _, voucher := range bestByLane {
		submitMessageCID, err := pcs.api.PaychVoucherSubmit(pcs.ctx, msg.To, voucher, nil, nil)
		if err != nil {
			return true, err
		}
		go func(voucher *paych.SignedVoucher, submitMessageCID cid.Cid) {
			defer wg.Done()
			msgLookup, err := pcs.api.StateWaitMsg(pcs.ctx, submitMessageCID, constants.MessageConfidence, constants.LookbackNoLimit, true)
			if err != nil {
				log.Errorf("submitting voucher: %s", err.Error())
			}
			if msgLookup.Receipt.ExitCode != 0 {
				log.Errorf("failed submitting voucher: %+v", voucher)
			}
		}(voucher, submitMessageCID)
	}
	wg.Wait()
	return true, nil
}

func (pcs *paymentChannelSettler) revertHandler(ctx context.Context, ts *types.TipSet) error {
	return nil
}

func (pcs *paymentChannelSettler) matcher(msg *types.UnsignedMessage) (matched bool, err error) {
	// Check if this is a settle payment channel message
	if msg.Method != paych.Methods.Settle {
		return false, nil
	}
	// Check if this payment channel is of concern to this node (i.e. tracked in payment channel store),
	// and its inbound (i.e. we're getting vouchers that we may need to redeem)
	trackedAddresses, err := pcs.api.PaychList(pcs.ctx)
	if err != nil {
		return false, err
	}
	for _, addr := range trackedAddresses {
		if msg.To == addr {
			status, err := pcs.api.PaychStatus(pcs.ctx, addr)
			if err != nil {
				return false, err
			}
			if status.Direction == types.PCHInbound {
				return true, nil
			}
		}
	}
	return false, nil
}

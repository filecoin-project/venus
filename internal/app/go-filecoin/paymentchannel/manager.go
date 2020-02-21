package paymentchannel

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	paychActor "github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
)

// Manager manages payment channel actor and the data store operations.
type Manager struct {
	ctx    context.Context
	store  *Store
	waiter MsgWaiter
	sender MsgSender
}

// MsgWaiter is an interface for waiting for a message to appear on chain
type MsgWaiter interface {
	Wait(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *vm.MessageReceipt) error) error
}

// MsgSender is an interface for something that can post messages on chain
type MsgSender interface {
	// Send sends a message to the chain
	Send(ctx context.Context,
		from, to address.Address,
		value types.AttoFIL,
		gasPrice types.AttoFIL, gasLimit types.GasUnits,
		bcast bool,
		method types.MethodID,
		params interface{}) (out cid.Cid, pubErrCh chan error, err error)
}

func NewManager(ctx context.Context, store *Store, waiter MsgWaiter, sender MsgSender) *Manager {
	return &Manager{ctx, store, waiter, sender}
}

// AllocateLane adds a new lane to a payment channel entry
func (pm *Manager) AllocateLane(paychAddr address.Address) (uint64, error) {
	return 0, nil
}

// GetPaymentChannelInfo retrieves channel info from the store
func (pm *Manager) GetPaymentChannelInfo(paychAddr address.Address) (ChannelInfo, error) {
	return ChannelInfo{}, nil
}

// CreatePaymentChannel will send the message to the InitActor to create a paych.Actor,
// If successful,  a new payment channel entry will be persisted to the store
func (pm *Manager) CreatePaymentChannel(payer, payee address.Address) (ChannelInfo, error) {
	// TODO: finish him
	//execParams, err := paychActorCtorExecParamsFor(clientAddress, minerAddress)
	//if err != nil {
	//	return address.Undef, err
	//}
	//msgCid, _, err := r.sndr.Send(
	//	context.Background(),
	//	clientAddress,
	//	builtin.InitActorAddr,
	//	types.ZeroAttoFIL,
	//	types.NewAttoFILFromFIL(1),
	//	types.NewGasUnits(300),
	//	true,
	//	types.MethodID(builtin.MethodsInit.Exec),
	//	execParams,
	//)
	//if err != nil {
	//	return address.Undef, err
	//}
	//err = r.wtr.Wait(ctx, msgCid, r.handleMessageReceived)
	//if err != nil {
	//	return address.Undef, err
	//}
	return ChannelInfo{}, nil
}


// UpdatePaymentChannel sends a signed voucher to the payment actor and persists the result
func (pm *Manager) UpdatePaymentChannel(paychAddr address.Address, voucher *paychActor.SignedVoucher) error {
	//execParams, err := paychActorCtorExecParamsFor(clientAddress, minerAddress)
	//if err != nil {
	//	return address.Undef, err
	//}
	// 1. get the channelinfo
	// use channel info to create and send msg\
	msgCid, _, err := r.sndr.Send(
		context.Background(),
		clientAddress,
		builtin.InitActorAddr,
		types.ZeroAttoFIL,
		types.NewAttoFILFromFIL(1),
		types.NewGasUnits(300),
		true,
		types.MethodID(builtin.MethodsInit.Exec),
		execParams,
	)
	if err != nil {
		return err
	}
	//err = r.wtr.Wait(ctx, msgCid, pm.handleUpdatePaymentChannelResult)
	//if err != nil {
	//	return address.Undef, err
	//}	return nil
}

func

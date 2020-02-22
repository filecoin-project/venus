package paymentchannel

import (
	"bytes"
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	initActor "github.com/filecoin-project/specs-actors/actors/builtin/init"
	paychActor "github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
)

var defaultMessageValue = types.NewAttoFILFromFIL(0)
var defaultGasPrice = types.NewAttoFILFromFIL(1)
var defaultGasLimit = types.NewGasUnits(300)

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
		gasPrice types.AttoFIL,
		gasLimit types.GasUnits,
		bcast bool,
		method types.MethodID,
		params interface{}) (out cid.Cid, pubErrCh chan error, err error)
}

func NewManager(ctx context.Context, store *Store, waiter MsgWaiter, sender MsgSender) *Manager {
	return &Manager{ctx, store, waiter, sender}
}

// AllocateLane adds a new lane to a payment channel entry
func (pm *Manager) AllocateLane(paychAddr address.Address) (uint64, error) {
	panic("implement AllocateLane")
	return 0, nil
}

// GetPaymentChannelInfo retrieves channel info from the store
func (pm *Manager) GetPaymentChannelInfo(paychAddr address.Address) (ChannelInfo, error) {
	return ChannelInfo{}, nil
}

// CreatePaymentChannel will send the message to the InitActor to create a paych.Actor.
// If successful, a new payment channel entry will be persisted to the store via a message wait handler
func (pm *Manager) CreatePaymentChannel(clientAddress, minerAddress address.Address) error {
	// TODO: finish him
	execParams, err := paychActorCtorExecParamsFor(clientAddress, minerAddress)
	if err != nil {
		return err
	}
	msgCid, _, err := pm.sender.Send(
		context.Background(),
		clientAddress,
		builtin.InitActorAddr,
		defaultMessageValue,
		defaultGasPrice,
		defaultGasLimit,
		true,
		types.MethodID(builtin.MethodsInit.Exec),
		execParams,
	)
	if err != nil {
		return err
	}
	err = pm.waiter.Wait(pm.ctx, msgCid, pm.handleCreatePaymentChannelResult)
	if err != nil {
		return err
	}
	return nil
}


// UpdatePaymentChannel sends a signed voucher to the payment actor and persists the result
func (pm *Manager) SendNewSignedVoucher(paychAddr address.Address, voucher *paychActor.SignedVoucher) error {
	execParams, err := updatePaymentChannelStateParamsFor(voucher)
	if err != nil {
		return err
	}

	chinfo, err := pm.store.getChannelInfo(paychAddr)
	if err != nil {
		return err
	}
	// use channel info to create and send msg
	msgCid, _, err := pm.sender.Send(
		context.Background(),
		chinfo.Owner,
		builtin.InitActorAddr,
		defaultMessageValue,
		defaultGasPrice,
		defaultGasLimit,
		true,
		types.MethodID(builtin.MethodsInit.Exec),
		execParams,
	)
	if err != nil {
		return err
	}
	// save voucher, secret, proof, msgCid in store
	err = pm.waiter.Wait(pm.ctx, msgCid, pm.handleUpdatePaymentChannelResult)
	if err != nil {
		return err
	}
	return nil
}

func (pm *Manager) SaveVoucher(paychAddr address.Address, voucher *paychActor.SignedVoucher, proof []byte, amt abi.TokenAmount) error {
	// save results in store
	panic("implement SaveVoucher")
	return nil
}

func (pm *Manager)handleUpdatePaymentChannelResult(b *block.Block, sm *types.SignedMessage, mr *vm.MessageReceipt) error {
	// save results in store
	panic("implement handleUpdatePaymentChannelResult")
	return nil
}

func (pm *Manager)handleCreatePaymentChannelResult(b *block.Block, sm *types.SignedMessage, mr *vm.MessageReceipt) error {
	// save results in store
	panic("implement handleCreatePaymentChannelResult")
	return nil
}

func updatePaymentChannelStateParamsFor(voucher *paychActor.SignedVoucher) (initActor.ExecParams, error) {
	ucp := paychActor.UpdateChannelStateParams{
		Sv:     paychActor.SignedVoucher{},
		// TODO secret, proof for UpdatePaymentChanneStateParams
		//Secret: nil,
		//Proof:  nil,
	}
	var marshaled bytes.Buffer
	err := ucp.MarshalCBOR(&marshaled)
	if err != nil {
		return initActor.ExecParams{}, err
	}

	p := initActor.ExecParams{
		CodeCID:           builtin.PaymentChannelActorCodeID,
		ConstructorParams: marshaled.Bytes(),
	}
	return p, nil
}

func paychActorCtorExecParamsFor(client, miner address.Address) (initActor.ExecParams, error) {
	ctorParams := paychActor.ConstructorParams{
		From: client,
		To:   miner,
	}
	var marshaled bytes.Buffer
	err := ctorParams.MarshalCBOR(&marshaled)
	if err != nil {
		return initActor.ExecParams{}, err
	}

	p := initActor.ExecParams{
		CodeCID:           builtin.PaymentChannelActorCodeID,
		ConstructorParams: marshaled.Bytes(),
	}
	return p, nil
}


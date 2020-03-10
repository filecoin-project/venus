package paymentchannel

import (
	"context"
	"reflect"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	initActor "github.com/filecoin-project/specs-actors/actors/builtin/init"
	paychActor "github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
)

var defaultGasPrice = types.NewAttoFILFromFIL(actor.DefaultGasCost)     // default gas price for messages sent from this manager
var defaultGasLimit = types.GasUnits(300)            // default gas limit for messages sent from this manager
var zeroAmt = abi.NewTokenAmount(0)

// Manager manages payment channel actor and the data paymentChannels operations.
type Manager struct {
	ctx             context.Context
	paymentChannels *statestore.StateStore
	sender          MsgSender
	waiter          MsgWaiter
	stateViewer     MgrStateViewer
	cr              ChainReader
}

// PaymentChannelStorePrefix is the prefix used in the datastore
var PaymentChannelStorePrefix = "/retrievaldeals/paymentchannel"

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
		method abi.MethodNum,
		params interface{}) (out cid.Cid, pubErrCh chan error, err error)
}

// MgrStateViewer is the subset of a StateViewer API that the Manager uses
type MgrStateViewer interface {
	StateView(root cid.Cid) PaychActorStateView
}

// PaychActorStateView is the subset of a StateView that the Manager uses
type PaychActorStateView interface {
	PaychActorParties(ctx context.Context, paychAddr address.Address) (from, to address.Address, err error)
}

// ChainReader is the subset of the ChainReadWriter API that the Manager uses
type ChainReader interface {
	GetTipSetStateRoot(context.Context, block.TipSetKey) (cid.Cid, error)
	Head() block.TipSetKey
}

// NewManager creates and returns a new paymentchannel.Manager
func NewManager(ctx context.Context, ds datastore.Batching, waiter MsgWaiter, sender MsgSender, viewer MgrStateViewer, cr ChainReader) *Manager {
	store := statestore.New(namespace.Wrap(ds, datastore.NewKey(PaymentChannelStorePrefix)))
	return &Manager{ctx, store, sender, waiter, viewer, cr}
}

// AllocateLane adds a new lane to a payment channel entry
func (pm *Manager) AllocateLane(paychAddr address.Address) (laneID uint64, err error) {
	err = pm.paymentChannels.
		Get(paychAddr).
		Mutate(func(chinfo *ChannelInfo) error {
			laneID = chinfo.LastLane
			chinfo.LastLane++
			return nil
		})
	return laneID, err
}

// GetPaymentChannelByAccounts looks up a payment channel via payer/payee
// returns ChannelInfoUndef if not found.
func (pm *Manager) GetPaymentChannelByAccounts(payer, payee address.Address) (*ChannelInfo, error) {
	var chinfos []ChannelInfo
	var found ChannelInfo

	if err := pm.paymentChannels.List(&chinfos); err != nil {
		return nil, err
	}
	for _, chinfo := range chinfos {
		if chinfo.From == payer && chinfo.To == payee {
			found = chinfo
			break
		}
	}
	return &found, nil
}

// GetPaymentChannelInfo retrieves channel info from the paymentChannels
func (pm *Manager) GetPaymentChannelInfo(paychAddr address.Address) (*ChannelInfo, error) {
	ss := pm.paymentChannels.Get(paychAddr)
	if ss == nil {
		return nil, xerrors.New("no stored state")
	}
	var chinfo ChannelInfo
	if err := ss.Get(&chinfo); err != nil {
		return nil, err
	}
	return &chinfo, nil
}

// CreatePaymentChannel will send the message to the InitActor to create a paych.Actor.
// If successful, a new payment channel entry will be persisted to the
// paymentChannels via a message wait handler.  Returns the created payment channel address
func (pm *Manager) CreatePaymentChannel(clientAddress, minerAddress address.Address) (address.Address, error) {
	chinfo, err := pm.GetPaymentChannelByAccounts(clientAddress, minerAddress)
	if err != nil {
		return address.Undef, err
	}
	if !chinfo.Blank() {
		return address.Undef, xerrors.Errorf("payment channel exists for client %s, miner %s", clientAddress, minerAddress)
	}

	execParams, err := PaychActorCtorExecParamsFor(clientAddress, minerAddress)
	if err != nil {
		return address.Undef, err
	}
	msgCid, _, err := pm.sender.Send(
		pm.ctx,
		clientAddress,
		builtin.InitActorAddr,
		types.ZeroAttoFIL,
		defaultGasPrice,
		defaultGasLimit,
		true,
		builtin.MethodsInit.Exec,
		execParams,
	)
	if err != nil {
		return address.Undef, err
	}

	var newPaychAddr address.Address

	handleResult := func(b *block.Block, sm *types.SignedMessage, mr *vm.MessageReceipt) error {
		var res initActor.ExecReturn
		if err := encoding.Decode(mr.ReturnValue, &res); err != nil {
			return err
		}

		var msgParams initActor.ExecParams
		if err := encoding.Decode(sm.Message.Params, &msgParams); err != nil {
			return err
		}

		var ctorParams *paychActor.ConstructorParams
		if err = encoding.Decode(msgParams.ConstructorParams, &ctorParams); err != nil {
			return err
		}

		chinfo := ChannelInfo{UniqueAddr: res.RobustAddress, From: ctorParams.From, To: ctorParams.To}
		newPaychAddr = res.RobustAddress
		return pm.paymentChannels.Begin(res.RobustAddress, &chinfo)
	}

	err = pm.waiter.Wait(pm.ctx, msgCid, handleResult)
	if err != nil {
		return address.Undef, err
	}

	return newPaychAddr, nil
}

// AddVoucherToChannel saves a new signed voucher entry to the payment store
// Assumes paychAddr channel has already been created.
//  Called by the retrieval market client.
func (pm *Manager) AddVoucherToChannel(paychAddr address.Address, voucher *paychActor.SignedVoucher) error {
	_, err := pm.saveNewVoucher(paychAddr, voucher, nil)
	return err
}

// AddVoucher saves voucher to the store
// if payment channel record does not exist in store, it will be created.
// called by the retrieval market provider, when it has received a new voucher from the client
func (pm *Manager) AddVoucher(paychAddr address.Address, voucher *paychActor.SignedVoucher, proof []byte) (abi.TokenAmount, error) {
	has, err := pm.paymentChannels.Has(paychAddr)
	if err != nil {
		return zeroAmt, err
	}
	if !has {
		return pm.createPaymentChannelWithVoucher(paychAddr, voucher, proof)
	}
	return pm.saveNewVoucher(paychAddr, voucher, proof)
}

// ChannelExists returns whether paychAddr has a store entry, + error
// Exported for retrieval provider
func (pm *Manager) ChannelExists(paychAddr address.Address) (bool, error) {
	return pm.paymentChannels.Has(paychAddr)
}

// PaychActorCtorExecParamsFor constructs parameters to send a message to InitActor
// To construct a paychActor
func PaychActorCtorExecParamsFor(client, miner address.Address) (initActor.ExecParams, error) {

	ctorParams := paychActor.ConstructorParams{From: client, To: miner}
	marshaled, err := encoding.Encode(ctorParams)
	if err != nil {
		return initActor.ExecParams{}, err
	}

	p := initActor.ExecParams{
		CodeCID:           builtin.PaymentChannelActorCodeID,
		ConstructorParams: marshaled,
	}
	return p, nil
}

// hasVoucher returns true if the voucher is already stored in the channelinfo
func (pm *Manager) hasVoucher(info *ChannelInfo, voucher *paychActor.SignedVoucher) bool {
	for _, v := range info.Vouchers {
		if reflect.DeepEqual(*v.Voucher, *voucher) {
			return true
		}
	}
	return false
}

func (pm *Manager) getStateView() (PaychActorStateView, error) {
	head := pm.cr.Head()
	root, err := pm.cr.GetTipSetStateRoot(pm.ctx, head)
	if err != nil {
		return nil, err
	}
	sv := pm.stateViewer.StateView(root)
	return sv, nil
}

func (pm *Manager) createPaymentChannelWithVoucher(paychAddr address.Address, voucher *paychActor.SignedVoucher, proof []byte) (abi.TokenAmount, error) {
	sv, err := pm.getStateView()
	if err != nil {
		return zeroAmt, err
	}
	from, to, err := sv.PaychActorParties(pm.ctx, paychAddr)
	if err != nil {
		return zeroAmt, err
	}

	chinfo := ChannelInfo{
		From:       from,
		To:         to,
		UniqueAddr: paychAddr,
		Vouchers:   []*VoucherInfo{{Voucher: voucher, Proof: proof}},
	}
	if err = pm.paymentChannels.Begin(paychAddr, &chinfo); err != nil {
		return zeroAmt, err
	}
	return voucher.Amount, nil

}

// saveNewVoucher saves a voucher to an existing payment channel
func (pm *Manager) saveNewVoucher(paychAddr address.Address, voucher *paychActor.SignedVoucher, proof []byte) (abi.TokenAmount, error) {
	var chinfo ChannelInfo
	st := pm.paymentChannels.Get(paychAddr)
	err := st.Get(&chinfo)
	if err != nil {
		return zeroAmt, err
	}
	if pm.hasVoucher(&chinfo, voucher) {
		return zeroAmt, xerrors.Errorf("voucher already saved: %s", string(voucher.Signature.Data))
	}
	if err = pm.paymentChannels.Get(paychAddr).Mutate(func(info *ChannelInfo) error {
		info.Vouchers = append(info.Vouchers, &VoucherInfo{
			Voucher: voucher,
			Proof:   proof,
		})
		return nil
	}); err != nil {
		return zeroAmt, err
	}
	return voucher.Amount, nil
}

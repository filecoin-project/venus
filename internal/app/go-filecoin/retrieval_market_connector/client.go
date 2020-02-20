package retrievalmarketconnector

import (
	"bytes"
	"context"
	"errors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	initActor "github.com/filecoin-project/specs-actors/actors/builtin/init"
	paychActor "github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/ipfs/go-cid"
	cid "github.com/ipfs/go-cid/_rsrch/cidiface"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paych"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
)

// RetrievalClientConnector is the glue between go-filecoin and go-fil-markets'
// retrieval market interface
type RetrievalClientConnector struct {
	bs blockstore.Blockstore
	cs *chain.Store

	// APIs/interfaces
	mw       MsgWaiter
	outbox   MsgSender
	paychMgr MgrAPI
	ps       piecestore.PieceStore
	signer   RetrievalSigner
	wal      WalletAPI
}

type laneEntries []types.AttoFIL

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

// WalletAPI is the subset of the Wallet interface needed by the retrieval client node
type WalletAPI interface {
	// GetBalance gets the balance in AttoFIL for a given address
	GetBalance(ctx context.Context, address address.Address) (types.AttoFIL, error)
}

// RetrievalSigner is an interface with the ability to sign data
type RetrievalSigner interface {
	SignBytes(data []byte, addr address.Address) (types.Signature, error)
}

type MgrAPI interface {
	AllocateLane(paychAddr address.Address) (int64, error)
	GetPaymentChannelInfo(paychAddr address.Address) (paych.ChannelInfo, error)
	CreatePaymentChannel(payer, payee address.Address) (paych.ChannelInfo, error)
	UpdatePaymentChannel(paychAddr address.Address) error
}

// NewRetrievalClientConnector creates a new RetrievalClientConnector
func NewRetrievalClientConnector(
	bs blockstore.Blockstore,
	cs *chain.Store,
	mw MsgWaiter,
	ob MsgSender,
	ps piecestore.PieceStore,
	signer RetrievalSigner,
	wal WalletAPI,
	paychMgr MgrAPI,
) *RetrievalClientConnector {
	return &RetrievalClientConnector{
		bs:       bs,
		cs:       cs,
		mw:       mw,
		outbox:   ob,
		paychMgr: paychMgr,
		ps:       ps,
		signer:   signer,
		wal:      wal,
	}
}

// GetOrCreatePaymentChannel gets or creates a payment channel and posts to chain
// Assumes GetOrCreatePaymentChannel is called before AllocateLane
// Blocks until message is mined?
func (r *RetrievalClientConnector) GetOrCreatePaymentChannel(ctx context.Context, clientAddress address.Address, minerAddress address.Address, clientFundsAvailable abi.TokenAmount) (address.Address, error) {

	if clientAddress == address.Undef || minerAddress == address.Undef {
		return address.Undef, errors.New("empty address")
	}

	var chid address.Address
	//chid, err := address.Undef
	//if err != nil {
	//	return address.Undef, err
	//}

	if chid == address.Undef {
		// create the payment channel
		bal, err := r.wal.GetBalance(ctx, clientAddress)
		if err != nil {
			return address.Undef, err
		}

		filAmt := types.NewAttoFIL(clientFundsAvailable.Int)
		if bal.LessThan(filAmt) {
			return address.Undef, errors.New("not enough funds in wallet")
		}

		execParams, err := paychActorCtorExecParamsFor(clientAddress, minerAddress)
		if err != nil {
			return address.Undef, err
		}
		msgCid, _, err := r.outbox.Send(
			ctx,
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
			return address.Undef, err
		}
		err = r.mw.Wait(ctx, msgCid, r.handleMessageReceived)
		if err != nil {
			return address.Undef, err
		}
	}

	// Not a real actor, just plays one on PaymentChannels.
	return chid, nil
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

// AllocateLane creates a new lane for this paymentChannel with 0 FIL in the lane
// Assumes AllocateLane is called after GetOrCreatePaymentChannel
//func (r *RetrievalClientConnector) AllocateLane(paymentChannel address.Address) (int64, error) {
func (r *RetrievalClientConnector) AllocateLane(paymentChannel address.Address) (lane uint64, err error) {
	//payer, err := r.getPayerForChannel(paymentChannel)
	//if err != nil {
	//	return 0, err
	//}
	//
	//if r.laneStore[payer] == nil {
	//	r.laneStore[payer] = []types.AttoFIL{}
	//}
	//r.laneStore[payer] = append(r.laneStore[payer], types.NewAttoFILFromFIL(0))
	//lane := uint64(len(r.laneStore[payer]) - 1)

	return lane, nil
}

// CreatePaymentVoucher creates a payment voucher for the retrieval client.
// If there is not enough value stored in the payment channel registry, an error is returned.
// If a lane has not been allocated for this payment channel, an error is returned.
func (r *RetrievalClientConnector) CreatePaymentVoucher(ctx context.Context, channelAddr address.Address, amount abi.TokenAmount, lane uint64) (*paychActor.SignedVoucher, error) {
	v, err := r.createSignedVoucher(ctx, channelAddr, lane, amount)
	if err != nil {
		return nil, err
	}
	return v, nil
}
func (r *RetrievalClientConnector) createSignedVoucher(ctx context.Context, payer address.Address, lane uint64, amount abi.TokenAmount) (*paychActor.SignedVoucher, error) {
	height, err := r.getBlockHeight()
	if err != nil {
		return nil, err
	}

	//nonce, err := r.NextNonce(ctx, payer)
	//if err != nil {
	//	return nil, err
	//}
	var nonce uint64

	v := paychActor.SignedVoucher{
		TimeLock:        0,   // TODO
		SecretPreimage:  nil, // TODO
		Extra:           nil, // TODO
		Lane:            lane,
		Nonce:           nonce,
		Amount:          amount,
		MinSettleHeight: abi.ChainEpoch(height + 1),
	}

	var buf bytes.Buffer
	if err := v.MarshalCBOR(&buf); err != nil {
		return nil, err
	}

	sig, err := r.signer.SignBytes(buf.Bytes(), payer)
	if err != nil {
		return nil, err
	}
	// TODO: set type correctly. See storagemarket
	signature := crypto.Signature{
		Type: crypto.SigTypeBLS,
		Data: sig,
	}
	v.Signature = &signature
	return &v, nil
}

// handleMessageReceived would be where any subscribers would be notified that the payment channel
// has been created.
func (r *RetrievalClientConnector) handleMessageReceived(_ *block.Block, sm *types.SignedMessage, mr *vm.MessageReceipt) error {
	return nil
}

func (r *RetrievalClientConnector) getBlockHeight() (uint64, error) {
	head := r.cs.GetHead()
	ts, err := r.cs.GetTipSet(head)
	if err != nil {
		return 0, err
	}
	return ts.Height()
}

func (r *RetrievalClientConnector) getPayerForChannel(paymentChannel address.Address) (payer address.Address, err error) {
	return payer, nil
}

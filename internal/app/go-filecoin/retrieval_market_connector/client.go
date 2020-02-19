package retrievalmarketconnector

import (
	"bytes"
	"context"
	"errors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/shared/tokenamount"
	gfm_types "github.com/filecoin-project/go-fil-markets/shared/types"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// CreatePaymentChannelMethod is the method to call to create a new payment channel (actor)
// TODO: make this the right method
var CreatePaymentChannelMethod = storagemarket.CreateStorageMiner

// RetrievalClientNodeConnector is the glue between go-filecoin and go-fil-markets'
// retrieval market interface
type RetrievalClientNodeConnector struct {
	bs blockstore.Blockstore
	cs *chain.Store

	// APIs/interfaces
	signer RetrievalSigner
	mw     MsgWaiter
	outbox MsgSender
	ps     piecestore.PieceStore
	wal    WalletAPI
	pchMgr PchMgrAPI
}

type laneEntries []types.AttoFIL

// ChannelInfo is a temporary struct for storing
type ChannelInfo struct {
	Payee    address.Address
	Amount   types.AttoFIL
	Redeemed types.AttoFIL
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

// WalletAPI is the subset of the Wallet interface needed by the retrieval client node
type WalletAPI interface {
	// GetBalance gets the balance in AttoFIL for a given address
	GetBalance(ctx context.Context, address address.Address) (types.AttoFIL, error)
}

// RetrievalSigner is an interface with the ability to sign data
type RetrievalSigner interface {
	SignBytes(data []byte, addr address.Address) (types.Signature, error)
}

type PchMgrAPI interface {
	AllocateLane(paychAddr address.Address) (uint64, error)
	GetPaymentChannelInfo(paychAddr address.Address)
	CreatePaymentChannel(payer, payee address.Address) (ChannelInfo, error)
	UpdatePaymentChannel(paychAddr address.Address) error
}

// NewRetrievalClientNodeConnector creates a new RetrievalClientNodeConnector
func NewRetrievalClientNodeConnector(
	bs blockstore.Blockstore,
	cs *chain.Store,
	mw MsgWaiter,
	ob MsgSender,
	ps piecestore.PieceStore,
	signer RetrievalSigner,
	wal WalletAPI,
) *RetrievalClientNodeConnector {
	return &RetrievalClientNodeConnector{
		bs:     bs,
		cs:     cs,
		mw:     mw,
		outbox: ob,
		ps:     ps,
		signer: signer,
		wal:    wal,
	}
}

// GetOrCreatePaymentChannel gets or creates a payment channel and posts to chain
// Assumes GetOrCreatePaymentChannel is called before AllocateLane
// Blocks until message is mined?
func (r *RetrievalClientNodeConnector) GetOrCreatePaymentChannel(ctx context.Context, clientAddress address.Address, minerAddress address.Address, clientFundsAvailable abi.TokenAmount) (address.Address, error) {

	if clientWallet == address.Undef || minerWallet == address.Undef {
		return address.Undef, errors.New("empty address")
	}

	var chid address.Address
	//chid, err := address.Undef
	//if err != nil {
	//	return address.Undef, err
	//}

	if chid == address.Undef {
		// create the payment channel
		bal, err := r.wal.GetBalance(ctx, clientWallet)
		if err != nil {
			return address.Undef, err
		}

		filAmt := types.NewAttoFIL(clientFundsAvailable.Int)
		if bal.LessThan(filAmt) {
			return address.Undef, errors.New("not enough funds in wallet")
		}

		height, err := r.getBlockHeight()
		if err != nil {
			return address.Undef, err
		}
		validAt := height + 1 // valid almost immediately since a retrieval could theoretically happen in 1 block

		params := struct {
			pa address.Address
			bh uint64
		}{minerWallet, validAt} // params: payment address, valid block height}
		msgCid, _, err := r.outbox.Send(
			ctx,
			clientWallet,               // from
			minerWallet,                // to
			types.ZeroAttoFIL,          // value
			types.NewAttoFILFromFIL(1), // gasPrice
			types.NewGasUnits(300),     // gasLimit
			true,                       // broadcast to network
			CreatePaymentChannelMethod, // command
			params,
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

// AllocateLane creates a new lane for this paymentChannel with 0 FIL in the lane
// Assumes AllocateLane is called after GetOrCreatePaymentChannel
//func (r *RetrievalClientNodeConnector) AllocateLane(paymentChannel address.Address) (int64, error) {
func (r *RetrievalClientNodeConnector) AllocateLane(paymentChannel address.Address) (lane uint64, err error) {
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
func (r *RetrievalClientNodeConnector) CreatePaymentVoucher(ctx context.Context, channelAddr address.Address, amount tokenamount.TokenAmount, lane uint64) (*gfm_types.SignedVoucher, error) {
	v, err := r.createSignedVoucher(ctx, channelAddr, lane, amount)
	if err != nil {
		return nil, err
	}
	return v, nil
}
func (r *RetrievalClientNodeConnector) createSignedVoucher(ctx context.Context, payer address.Address, lane uint64, amount tokenamount.TokenAmount) (*gfm_types.SignedVoucher, error) {
	height, err := r.getBlockHeight()
	if err != nil {
		return nil, err
	}

	//nonce, err := r.NextNonce(ctx, payer)
	//if err != nil {
	//	return nil, err
	//}
	var nonce uint64

	v := gfm_types.SignedVoucher{
		TimeLock:       0,   // TODO
		SecretPreimage: nil, // TODO
		Extra:          nil, // TODO
		Lane:           lane,
		Nonce:          nonce,
		Amount:         amount,
		MinCloseHeight: height + 1,
	}

	var buf bytes.Buffer
	if err := v.MarshalCBOR(&buf); err != nil {
		return nil, err
	}

	sig, err := r.signer.SignBytes(buf.Bytes(), payer)
	if err != nil {
		return nil, err
	}
	signature := gfm_types.Signature{
		Type: gfm_types.KTBLS,
		Data: sig,
	}
	v.Signature = &signature
	return &v, nil
}

// handleMessageReceived would be where any subscribers would be notified that the payment channel
// has been created.
func (r *RetrievalClientNodeConnector) handleMessageReceived(_ *block.Block, sm *types.SignedMessage, mr *types.MessageReceipt) error {
	return nil
}

func (r *RetrievalClientNodeConnector) getBlockHeight() (uint64, error) {
	head := r.cs.GetHead()
	ts, err := r.cs.GetTipSet(head)
	if err != nil {
		return 0, err
	}
	return ts.Height()
}

func (r *RetrievalClientNodeConnector) getPayerForChannel(paymentChannel address.Address) (payer address.Address, err error) {
	return payer, nil
}

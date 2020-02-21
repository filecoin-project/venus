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
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
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
	paychMgr MgrAPI
	ps       piecestore.PieceStore
	signer   RetrievalSigner
	wal      WalletAPI
}

type laneEntries []types.AttoFIL

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
	AllocateLane(paychAddr address.Address) (uint64, error)
	GetPaymentChannelInfo(paychAddr address.Address) (*paymentchannel.ChannelInfo, error)
	GetPaymentChannelByAccounts(payer, payee address.Address) (address.Address, *paymentchannel.ChannelInfo)
	CreatePaymentChannel(payer, payee address.Address) (address.Address, error)
	UpdatePaymentChannel(paychAddr address.Address, voucher *paychActor.SignedVoucher) error
}

// NewRetrievalClientConnector creates a new RetrievalClientConnector
func NewRetrievalClientConnector(
	bs blockstore.Blockstore,
	cs *chain.Store,
	ps piecestore.PieceStore,
	signer RetrievalSigner,
	wal WalletAPI,
	paychMgr MgrAPI,
) *RetrievalClientConnector {
	return &RetrievalClientConnector{
		bs:       bs,
		cs:       cs,
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
	paychAddr, ci := r.paychMgr.GetPaymentChannelByAccounts(clientAddress, minerAddress)
	if ci == nil {
		// create the payment channel
		bal, err := r.wal.GetBalance(ctx, clientAddress)
		if err != nil {
			return address.Undef, err
		}

		filAmt := types.NewAttoFIL(clientFundsAvailable.Int)
		if bal.LessThan(filAmt) {
			return address.Undef, errors.New("not enough funds in wallet")
		}
		return r.paychMgr.CreatePaymentChannel(clientAddress, minerAddress)
	}

	// Not a real actor, just plays one on PaymentChannels.
	return paychAddr, nil
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
	return r.paychMgr.AllocateLane(paymentChannel)
}

// CreatePaymentVoucher creates a payment voucher for the retrieval client.
// If there is not enough value stored in the payment channel registry, an error is returned.
// If a lane has not been allocated for this payment channel, an error is returned.
func (r *RetrievalClientConnector) CreatePaymentVoucher(ctx context.Context, paychAddr address.Address, amount abi.TokenAmount, lane uint64) (*paychActor.SignedVoucher, error) {
	height, err := r.getBlockHeight()
	if err != nil {
		return nil, err
	}

	chinfo, err := r.paychMgr.GetPaymentChannelInfo(paychAddr)
	if err != nil {
		return nil, err
	}
	if lane >= uint64(len(chinfo.State.LaneStates)) {
		return nil, xerrors.New("lane not allocated")
	}
	ls := chinfo.State.LaneStates[lane]
	v := paychActor.SignedVoucher{
		TimeLock:        0,   // TODO
		SecretPreimage:  nil, // TODO
		Extra:           nil, // TODO
		Lane:            lane,
		Nonce:           ls.Nonce+1, // TODO
		Amount:          amount,
		MinSettleHeight: abi.ChainEpoch(height + 1),
	}

	var buf bytes.Buffer
	if err := v.MarshalCBOR(&buf); err != nil {
		return nil, err
	}

	sig, err := r.signer.SignBytes(buf.Bytes(), chinfo.Owner)
	if err != nil {
		return nil, err
	}
	// TODO: set type correctly. See storagemarket
	signature := crypto.Signature{
		Type: crypto.SigTypeBLS,
		Data: sig,
	}
	v.Signature = &signature

	// TODO tell Mgr to send UpdateChannelState message with the new voucher and signature
	// if successful:
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

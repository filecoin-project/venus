package retrievalmarketconnector

import (
	"bytes"
	"context"
	"errors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	paychActor "github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
)

// RetrievalClientConnector is the glue between go-filecoin and go-fil-markets'
// retrieval market interface
type RetrievalClientConnector struct {
	bs blockstore.Blockstore

	// APIs/interfaces
	paychMgr PaychMgrAPI
	signer   RetrievalSigner
	cs ChainReaderAPI
}

// WalletAPI is the subset of the Wallet interface needed by the retrieval client node
type ChainReaderAPI interface {
	// GetBalance gets the balance in AttoFIL for a given address
	Head() block.TipSetKey
	GetTipSet(key block.TipSetKey) (block.TipSet, error)
	GetActorAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address) (*actor.Actor, error)
}

// RetrievalSigner is an interface with the ability to sign data
type RetrievalSigner interface {
	SignBytes(data []byte, addr address.Address) (crypto.Signature, error)
}

// PaychMgrAPI is an API used for communicating with payment channel actor and store.
type PaychMgrAPI interface {
	AllocateLane(paychAddr address.Address) (uint64, error)
	GetPaymentChannelInfo(paychAddr address.Address) (*paymentchannel.ChannelInfo, error)
	GetPaymentChannelByAccounts(payer, payee address.Address) (address.Address, *paymentchannel.ChannelInfo)
	CreatePaymentChannel(payer, payee address.Address) error
	CreateVoucher(paychAddr address.Address, voucher *paychActor.SignedVoucher) error
	SaveVoucher(paychAddr address.Address, voucher *paychActor.SignedVoucher, proof []byte, expected abi.TokenAmount) (actual abi.TokenAmount, err error)
}

// NewRetrievalClientConnector creates a new RetrievalClientConnector
func NewRetrievalClientConnector(
	bs blockstore.Blockstore,
	cs ChainReaderAPI,
	signer RetrievalSigner,
	paychMgr PaychMgrAPI,
) *RetrievalClientConnector {
	return &RetrievalClientConnector{
		bs:       bs,
		cs:       cs,
		paychMgr: paychMgr,
		signer:   signer,
	}
}

// GetOrCreatePaymentChannel gets or creates a payment channel and posts to chain
func (r *RetrievalClientConnector) GetOrCreatePaymentChannel(ctx context.Context, clientAddress address.Address, minerAddress address.Address, clientFundsAvailable abi.TokenAmount) (address.Address, error) {

	if clientAddress == address.Undef || minerAddress == address.Undef {
		return address.Undef, errors.New("empty address")
	}
	paychAddr, ci := r.paychMgr.GetPaymentChannelByAccounts(clientAddress, minerAddress)
	if ci == nil {
		// create the payment channel
		bal, err := r.getBalance(ctx, clientAddress)
		if err != nil {
			return address.Undef, err
		}

		filAmt := types.NewAttoFIL(clientFundsAvailable.Int)
		if bal.LessThan(filAmt) {
			return address.Undef, errors.New("not enough funds in wallet")
		}
		return address.Undef, r.paychMgr.CreatePaymentChannel(clientAddress, minerAddress)
	}
	return paychAddr, nil
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
	ls := chinfo.State.LaneStates[lane]
	v := paychActor.SignedVoucher{
		TimeLock:        0,   // TODO
		SecretPreimage:  nil, // TODO
		Extra:           nil, // TODO
		Lane:            lane,
		Nonce:           ls.Nonce+1, // TODO
		Amount:          amount,
		MinSettleHeight: height+1,
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
		Data: sig.Data,
	}
	v.Signature = &signature

	if err := r.paychMgr.CreateVoucher(paychAddr, &v); err != nil {
		return nil, err
	}
	// if successful:
	return &v, nil
}

func (r *RetrievalClientConnector) getBlockHeight() (abi.ChainEpoch, error) {
	ts, err := r.getHeadTipSet()
	if err != nil {
		return 0, err
	}
	return ts.Height()
}

func (r *RetrievalClientConnector)getBalance(ctx context.Context, account address.Address) (types.AttoFIL, error){
	ts, err := r.getHeadTipSet()
	if err != nil {
		return types.ZeroAttoFIL, err
	}

	actor, err := r.cs.GetActorAt(ctx, ts.Key(), account)
	if err != nil {
		return types.ZeroAttoFIL, err
	}

	return actor.Balance, nil
}

func (r *RetrievalClientConnector) getHeadTipSet() (block.TipSet, error) {
	head := r.cs.Head()
	ts, err := r.cs.GetTipSet(head)
	if err != nil {
		return block.TipSet{}, err
	}
	return ts, nil
}


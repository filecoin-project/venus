package retrievalmarketconnector

import (
	"bytes"
	"context"
	"errors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/specs-actors/actors/abi"
	paychActor "github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// RetrievalClientConnector is the glue between go-filecoin and go-fil-markets'
// retrieval market interface
type RetrievalClientConnector struct {
	bs blockstore.Blockstore

	// APIs/interfaces
	paychMgr PaychMgrAPI
	signer   RetrievalSigner
	cs       ChainReaderAPI
}

var _ retrievalmarket.RetrievalClientNode = new(RetrievalClientConnector)

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
func (r *RetrievalClientConnector) GetOrCreatePaymentChannel(ctx context.Context, clientAddress address.Address, minerAddress address.Address, clientFundsAvailable abi.TokenAmount, tok shared.TipSetToken) (address.Address, cid.Cid, error) {
	if clientAddress == address.Undef || minerAddress == address.Undef {
		return address.Undef, cid.Undef, errors.New("empty address")
	}
	chinfo, err := r.paychMgr.GetPaymentChannelByAccounts(clientAddress, minerAddress)
	if err != nil {
		return address.Undef, cid.Undef, err
	}
	if chinfo.IsZero() {
		// create the payment channel
		bal, err := r.getBalance(ctx, clientAddress, tok)
		if err != nil {
			return address.Undef, cid.Undef, err
		}

		filAmt := types.NewAttoFIL(clientFundsAvailable.Int)
		if bal.LessThan(filAmt) {
			return address.Undef, cid.Undef, errors.New("not enough funds in wallet")
		}
		return r.paychMgr.CreatePaymentChannel(clientAddress, minerAddress, clientFundsAvailable)
	}
	// TODO: I think this is supposed to return the message CID from the creation of the payment channel.
	// If there was one already, what CID should this return?
	// What happens if that message is subsequently re-orged out of the chain and never replayed?
	// https://github.com/filecoin-project/go-filecoin/issues/4034
	return chinfo.UniqueAddr, cid.Undef, nil
}

// AllocateLane creates a new lane for this paymentChannel with 0 FIL in the lane
// Assumes AllocateLane is called after GetOrCreatePaymentChannel
func (r *RetrievalClientConnector) AllocateLane(paymentChannel address.Address) (lane uint64, err error) {
	return r.paychMgr.AllocateLane(paymentChannel)
}

// CreatePaymentVoucher creates a payment voucher for the retrieval client.
func (r *RetrievalClientConnector) CreatePaymentVoucher(ctx context.Context, paychAddr address.Address, amount abi.TokenAmount, lane uint64, tok shared.TipSetToken) (*paychActor.SignedVoucher, error) {
	height, err := r.getBlockHeight(tok)
	if err != nil {
		return nil, err
	}

	bal, err := r.getBalance(ctx, paychAddr, tok)
	if err != nil {
		return nil, err
	}
	if amount.GreaterThan(bal) {
		return nil, xerrors.New("insufficient funds for voucher amount")
	}

	chinfo, err := r.paychMgr.GetPaymentChannelInfo(paychAddr)
	if err != nil {
		return nil, err
	}
	v := paychActor.SignedVoucher{
		TimeLockMin:     height + 1,
		SecretPreimage:  nil, // optional
		Extra:           nil, // optional
		Lane:            lane,
		Nonce:           chinfo.NextNonce,
		Amount:          amount,
		MinSettleHeight: height + 1,
		Merges:          nil,
		Signature:       nil,
	}

	var buf bytes.Buffer
	if err := v.MarshalCBOR(&buf); err != nil {
		return nil, err
	}

	sig, err := r.signer.SignBytes(ctx, buf.Bytes(), chinfo.From)
	if err != nil {
		return nil, err
	}
	v.Signature = &sig

	if err := r.paychMgr.AddVoucherToChannel(paychAddr, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *RetrievalClientConnector) WaitForPaymentChannelCreation(messageCID cid.Cid) (address.Address, error) {
	// TODO https://github.com/filecoin-project/go-filecoin/issues/4034
	panic("implement me")
}

func (r *RetrievalClientConnector) WaitForPaymentChannelAddFunds(messageCID cid.Cid) error {
	// TODO https://github.com/filecoin-project/go-filecoin/issues/4034
	panic("implement me")
}

func (r *RetrievalClientConnector) getBlockHeight(tok shared.TipSetToken) (abi.ChainEpoch, error) {
	ts, err := r.getTipSet(tok)
	if err != nil {
		return 0, err
	}
	return ts.Height()
}

func (r *RetrievalClientConnector) getBalance(ctx context.Context, account address.Address, tok shared.TipSetToken) (types.AttoFIL, error) {
	var tsk block.TipSetKey
	if err := encoding.Decode(tok, &tsk); err != nil {
		return types.ZeroAttoFIL, xerrors.Wrapf(err, "failed to marshal TipSetToken into a TipSetKey")
	}

	actor, err := r.cs.GetActorAt(ctx, tsk, account)
	if err != nil {
		return types.ZeroAttoFIL, err
	}

	return actor.Balance, nil
}

func (r *RetrievalClientConnector) GetChainHead(ctx context.Context) (shared.TipSetToken, abi.ChainEpoch, error) {
	return connectors.GetChainHead(r.cs)
}

func (r *RetrievalClientConnector) getTipSet(tok shared.TipSetToken) (block.TipSet, error) {
	var tsk block.TipSetKey
	if err := encoding.Decode(tok, &tsk); err != nil {
		return block.TipSet{}, xerrors.Wrapf(err, "failed to marshal TipSetToken into a TipSetKey")
	}

	return r.cs.GetTipSet(tsk)
}

package v0api

import (
	"context"
	"fmt"

	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
)

type WrapperV1IMultiSig struct {
	v1api.IMultiSig
	v1api.IMessagePool
}

var _ v0api.IMultiSig = (*WrapperV1IMultiSig)(nil)

func (a *WrapperV1IMultiSig) executePrototype(ctx context.Context, p *types.MessagePrototype) (cid.Cid, error) {
	sm, err := a.IMessagePool.MpoolPushMessage(ctx, &p.Message, nil)
	if err != nil {
		return cid.Undef, fmt.Errorf("pushing message: %w", err)
	}

	return sm.Cid(), nil
}
func (a *WrapperV1IMultiSig) MsigCreate(ctx context.Context, req uint64, addrs []address.Address, duration abi.ChainEpoch, val types.BigInt, src address.Address, gp types.BigInt) (cid.Cid, error) {

	p, err := a.IMultiSig.MsigCreate(ctx, req, addrs, duration, val, src, gp)
	if err != nil {
		return cid.Undef, fmt.Errorf("creating prototype: %w", err)
	}

	return a.executePrototype(ctx, p)
}

func (a *WrapperV1IMultiSig) MsigPropose(ctx context.Context, msig address.Address, to address.Address, amt types.BigInt, src address.Address, method uint64, params []byte) (cid.Cid, error) {

	p, err := a.IMultiSig.MsigPropose(ctx, msig, to, amt, src, method, params)
	if err != nil {
		return cid.Undef, fmt.Errorf("creating prototype: %w", err)
	}

	return a.executePrototype(ctx, p)
}
func (a *WrapperV1IMultiSig) MsigApprove(ctx context.Context, msig address.Address, txID uint64, src address.Address) (cid.Cid, error) {

	p, err := a.IMultiSig.MsigApprove(ctx, msig, txID, src)
	if err != nil {
		return cid.Undef, fmt.Errorf("creating prototype: %w", err)
	}

	return a.executePrototype(ctx, p)
}

func (a *WrapperV1IMultiSig) MsigApproveTxnHash(ctx context.Context, msig address.Address, txID uint64, proposer address.Address, to address.Address, amt types.BigInt, src address.Address, method uint64, params []byte) (cid.Cid, error) {
	p, err := a.IMultiSig.MsigApproveTxnHash(ctx, msig, txID, proposer, to, amt, src, method, params)
	if err != nil {
		return cid.Undef, fmt.Errorf("creating prototype: %w", err)
	}

	return a.executePrototype(ctx, p)
}

func (a *WrapperV1IMultiSig) MsigCancel(ctx context.Context, msig address.Address, txID uint64, src address.Address) (cid.Cid, error) {
	p, err := a.IMultiSig.MsigCancel(ctx, msig, txID, src)
	if err != nil {
		return cid.Undef, fmt.Errorf("creating prototype: %w", err)
	}

	return a.executePrototype(ctx, p)
}

func (a *WrapperV1IMultiSig) MsigCancelTxnHash(ctx context.Context, msig address.Address, txID uint64, to address.Address, amt types.BigInt, src address.Address, method uint64, params []byte) (cid.Cid, error) {
	p, err := a.IMultiSig.MsigCancelTxnHash(ctx, msig, txID, to, amt, src, method, params)
	if err != nil {
		return cid.Undef, fmt.Errorf("creating prototype: %w", err)
	}

	return a.executePrototype(ctx, p)
}

func (a *WrapperV1IMultiSig) MsigAddPropose(ctx context.Context, msig address.Address, src address.Address, newAdd address.Address, inc bool) (cid.Cid, error) {

	p, err := a.IMultiSig.MsigAddPropose(ctx, msig, src, newAdd, inc)
	if err != nil {
		return cid.Undef, fmt.Errorf("creating prototype: %w", err)
	}

	return a.executePrototype(ctx, p)
}

func (a *WrapperV1IMultiSig) MsigAddApprove(ctx context.Context, msig address.Address, src address.Address, txID uint64, proposer address.Address, newAdd address.Address, inc bool) (cid.Cid, error) {

	p, err := a.IMultiSig.MsigAddApprove(ctx, msig, src, txID, proposer, newAdd, inc)
	if err != nil {
		return cid.Undef, fmt.Errorf("creating prototype: %w", err)
	}

	return a.executePrototype(ctx, p)
}

func (a *WrapperV1IMultiSig) MsigAddCancel(ctx context.Context, msig address.Address, src address.Address, txID uint64, newAdd address.Address, inc bool) (cid.Cid, error) {

	p, err := a.IMultiSig.MsigAddCancel(ctx, msig, src, txID, newAdd, inc)
	if err != nil {
		return cid.Undef, fmt.Errorf("creating prototype: %w", err)
	}

	return a.executePrototype(ctx, p)
}

func (a *WrapperV1IMultiSig) MsigSwapPropose(ctx context.Context, msig address.Address, src address.Address, oldAdd address.Address, newAdd address.Address) (cid.Cid, error) {

	p, err := a.IMultiSig.MsigSwapPropose(ctx, msig, src, oldAdd, newAdd)
	if err != nil {
		return cid.Undef, fmt.Errorf("creating prototype: %w", err)
	}

	return a.executePrototype(ctx, p)
}

func (a *WrapperV1IMultiSig) MsigSwapApprove(ctx context.Context, msig address.Address, src address.Address, txID uint64, proposer address.Address, oldAdd address.Address, newAdd address.Address) (cid.Cid, error) {

	p, err := a.IMultiSig.MsigSwapApprove(ctx, msig, src, txID, proposer, oldAdd, newAdd)
	if err != nil {
		return cid.Undef, fmt.Errorf("creating prototype: %w", err)
	}

	return a.executePrototype(ctx, p)
}

func (a *WrapperV1IMultiSig) MsigSwapCancel(ctx context.Context, msig address.Address, src address.Address, txID uint64, oldAdd address.Address, newAdd address.Address) (cid.Cid, error) {

	p, err := a.IMultiSig.MsigSwapCancel(ctx, msig, src, txID, oldAdd, newAdd)
	if err != nil {
		return cid.Undef, fmt.Errorf("creating prototype: %w", err)
	}

	return a.executePrototype(ctx, p)
}

func (a *WrapperV1IMultiSig) MsigRemoveSigner(ctx context.Context, msig address.Address, proposer address.Address, toRemove address.Address, decrease bool) (cid.Cid, error) {

	p, err := a.IMultiSig.MsigRemoveSigner(ctx, msig, proposer, toRemove, decrease)
	if err != nil {
		return cid.Undef, fmt.Errorf("creating prototype: %w", err)
	}

	return a.executePrototype(ctx, p)
}

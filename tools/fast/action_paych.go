package fast

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// PaychCreate runs the `paych create` command against the filecoin process.
func (f *Filecoin) PaychCreate(ctx context.Context,
	target address.Address, amount types.AttoFIL, eol *types.BlockHeight,
	options ...ActionOption) (cid.Cid, error) {

	var out commands.CreateChannelResult
	args := []string{"go-filecoin", "paych", "create", target.String(), amount.String(), eol.String()}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return cid.Undef, err
	}

	return out.Cid, nil
}

// PaychClose runs the `paych close` command against the filecoin process.
func (f *Filecoin) PaychClose(ctx context.Context, voucher string, options ...ActionOption) (cid.Cid, error) {
	var out commands.CloseResult
	args := []string{"go-filecoin", "paych", "close", voucher}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return cid.Undef, err
	}

	return out.Cid, nil

}

// PaychExtend runs the `paych extend` command against the filecoin process.
func (f *Filecoin) PaychExtend(ctx context.Context,
	channel *types.ChannelID, amount types.AttoFIL, eol *types.BlockHeight,
	options ...ActionOption) (cid.Cid, error) {

	var out commands.ExtendResult
	args := []string{"go-filecoin", "paych", "extend", channel.String(), amount.String(), eol.String()}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return cid.Undef, err
	}

	return out.Cid, nil

}

// PaychCancel runs the `paych cancel` command against the filecoin process.
func (f *Filecoin) PaychCancel(ctx context.Context, channel *types.ChannelID, options ...ActionOption) (cid.Cid, error) {

	var out commands.CancelResult
	args := []string{"go-filecoin", "paych", "cancel", channel.String()}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return cid.Undef, err
	}

	return out.Cid, nil
}

// PaychLs runs the `paych ls` command against the filecoin process.
func (f *Filecoin) PaychLs(ctx context.Context, options ...ActionOption) (map[string]*paymentbroker.PaymentChannel, error) {
	var out map[string]*paymentbroker.PaymentChannel
	args := []string{"go-filecoin", "paych", "ls"}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return out, nil
}

// PaychReclaim runs the `paych reclaim` command against the filecoin process.
func (f *Filecoin) PaychReclaim(ctx context.Context, channel *types.ChannelID, options ...ActionOption) (cid.Cid, error) {
	var out commands.ReclaimResult
	args := []string{"go-filecoin", "paych", "reclaim", channel.String()}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return cid.Undef, err
	}

	return out.Cid, nil
}

// PaychRedeem runs the `paych redeem` command against the filecoin process.
func (f *Filecoin) PaychRedeem(ctx context.Context, voucher string, options ...ActionOption) (cid.Cid, error) {
	var out commands.RedeemResult
	args := []string{"go-filecoin", "paych", "redeem", voucher}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return cid.Undef, err
	}

	return out.Cid, nil
}

// PaychVoucher runs the `paych voucher` command against the filecoin process.
func (f *Filecoin) PaychVoucher(ctx context.Context, channel *types.ChannelID, amount types.AttoFIL, options ...ActionOption) (string, error) {
	var out string

	args := []string{"go-filecoin", "paych", "voucher", channel.String(), amount.String()}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return "", err
	}

	return out, nil
}

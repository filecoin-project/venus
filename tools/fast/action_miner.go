package fast

import (
	"context"
	"math/big"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// MinerCreate runs the `miner create` command against the filecoin process
func (f *Filecoin) MinerCreate(ctx context.Context, collateral *big.Int, options ...ActionOption) (address.Address, error) {
	var out commands.MinerCreateResult

	sCollateral := collateral.String()

	args := []string{"go-filecoin", "miner", "create"}

	for _, option := range options {
		args = append(args, option()...)
	}

	args = append(args, sCollateral)

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return address.Undef, err
	}

	return out.Address, nil
}

// MinerUpdatePeerid runs the `miner update-peerid` command against the filecoin process
func (f *Filecoin) MinerUpdatePeerid(ctx context.Context, minerAddr address.Address, pid peer.ID, options ...ActionOption) (cid.Cid, error) {
	var out commands.MinerUpdatePeerIDResult

	args := []string{"go-filecoin", "miner", "update-peerid"}

	for _, option := range options {
		args = append(args, option()...)
	}

	args = append(args, minerAddr.String(), pid.Pretty())

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return cid.Undef, err
	}

	return out.Cid, nil
}

// MinerOwner runs the `miner owner` command against the filecoin process
func (f *Filecoin) MinerOwner(ctx context.Context, minerAddr address.Address) (address.Address, error) {
	var out address.Address

	sMinerAddr := minerAddr.String()

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "miner", "owner", sMinerAddr); err != nil {
		return address.Undef, err
	}

	return out, nil
}

// MinerPower runs the `miner power` command against the filecoin process
func (f *Filecoin) MinerPower(ctx context.Context, minerAddr address.Address) (porcelain.MinerPower, error) {
	var out porcelain.MinerPower

	sMinerAddr := minerAddr.String()

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "miner", "power", sMinerAddr); err != nil {
		return porcelain.MinerPower{}, err
	}

	return out, nil
}

// MinerSetPrice runs the `miner set-price` command against the filecoin process
func (f *Filecoin) MinerSetPrice(ctx context.Context, fil *big.Float, expiry *big.Int, options ...ActionOption) (*porcelain.MinerSetPriceResponse, error) {
	var out commands.MinerSetPriceResult

	sExpiry := expiry.String()
	sFil := fil.Text('f', -1)

	args := []string{"go-filecoin", "miner", "set-price"}

	for _, option := range options {
		args = append(args, option()...)
	}

	args = append(args, sFil, sExpiry)

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return &out.MinerSetPriceResponse, nil
}

// MinerProvingWindow runs the `miner proving-window` command against the filecoin process
func (f *Filecoin) MinerProvingWindow(ctx context.Context, miner address.Address) (porcelain.MinerProvingWindow, error) {
	var out porcelain.MinerProvingWindow

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "miner", "proving-window", miner.String()); err != nil {
		return porcelain.MinerProvingWindow{}, err
	}

	return out, nil
}

// MinerWorker runs the `miner worker` command against the filecoin process
func (f *Filecoin) MinerWorker(ctx context.Context) (commands.MinerWorkerResult, error) {
	var out commands.MinerWorkerResult

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "miner", "worker"); err != nil {
		return out, err
	}
	return out, nil
}

// MinerSetWorker runs the `miner set-worker` command against the filecoin process
func (f *Filecoin) MinerSetWorker(ctx context.Context, newAddr address.Address, options ...ActionOption) (cid.Cid, error) {
	var out cid.Cid

	args := []string{"go-filecoin", "miner", "set-worker", newAddr.String()}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return out, err
	}
	return out, nil
}

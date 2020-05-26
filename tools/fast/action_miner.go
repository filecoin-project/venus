package fast

import (
	"context"
	"math/big"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"

	commands "github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
)

// MinerCreate runs the `miner create` command against the filecoin process
func (f *Filecoin) MinerCreate(ctx context.Context, collateral *big.Int, options ...ActionOption) (address.Address, error) {
	var out commands.MinerCreateResult

	args := []string{"go-filecoin", "miner", "create"}

	for _, option := range options {
		args = append(args, option()...)
	}

	args = append(args, collateral.String())

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

// MinerStatus runs the `miner power` command against the filecoin process
func (f *Filecoin) MinerStatus(ctx context.Context, minerAddr address.Address) (porcelain.MinerStatus, error) {
	var out porcelain.MinerStatus

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "miner", "status", minerAddr.String()); err != nil {
		return porcelain.MinerStatus{}, err
	}

	return out, nil
}

// MinerSetPrice runs the `miner set-price` command against the filecoin process
func (f *Filecoin) MinerSetPrice(ctx context.Context, fil *big.Float, expiry *big.Int, options ...ActionOption) (*porcelain.MinerSetPriceResponse, error) {
	var out commands.MinerSetPriceResult

	sFil := fil.Text('f', -1)

	args := []string{"go-filecoin", "miner", "set-price"}

	for _, option := range options {
		args = append(args, option()...)
	}

	args = append(args, sFil, expiry.String())

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return &porcelain.MinerSetPriceResponse{
		MinerAddr: out.MinerAddress,
		Price:     out.Price,
	}, nil
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

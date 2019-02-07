package fast

import (
	"context"
	"fmt"
	"math/big"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/commands"
	"github.com/filecoin-project/go-filecoin/porcelain"
)

// MinerCreate runs the `miner create` command against the filecoin process
func (f *Filecoin) MinerCreate(ctx context.Context, pledge uint64, collateral *big.Int, options ...ActionOption) (address.Address, error) {
	var out commands.MinerCreateResult

	sPledge := fmt.Sprintf("%d", pledge)
	sCollateral := collateral.String()

	args := []string{"go-filecoin", "miner", "create"}

	for _, option := range options {
		args = append(args, option()...)
	}

	args = append(args, sPledge, sCollateral)

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

// MinerPledge runs the `miner pledge` command against the filecoin process
func (f *Filecoin) MinerPledge(ctx context.Context, minerAddr address.Address) (*big.Int, error) {
	var out big.Int

	sMinerAddr := minerAddr.String()

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "miner", "pledge", sMinerAddr); err != nil {
		return big.NewInt(0), err
	}

	return &out, nil
}

// MinerPower runs the `miner power` command against the filecoin process
func (f *Filecoin) MinerPower(ctx context.Context, minerAddr address.Address) (*big.Int, error) {
	var out big.Int

	sMinerAddr := minerAddr.String()

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "miner", "power", sMinerAddr); err != nil {
		return big.NewInt(0), err
	}

	return &out, nil
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

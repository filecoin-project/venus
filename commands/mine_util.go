package commands

import (
	"context"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/types"
)

// MinerProvingPeriodResult is the type returned when getting a miner's proving period.
type MinerProvingPeriodResult struct {
	Start types.BlockHeight            `json:"start,omitempty"`
	End   types.BlockHeight            `json:"end,omitempty"`
	Set   map[string]types.Commitments `json:"set,omitempty"`
}

// GetMinerProvingPeriod returns a MinerProvignPeriodResult for the miner at `minerAddress`.
func GetMinerProvingPeriod(ctx context.Context, minerAddress address.Address, api *porcelain.API) (MinerProvingPeriodResult, error) {
	res, err := api.MessageQuery(
		ctx,
		address.Undef,
		minerAddress,
		"getProvingPeriod",
	)
	if err != nil {
		return MinerProvingPeriodResult{}, errors.Wrap(err, "query ProvingPeriod method failed")
	}
	start, end := types.NewBlockHeightFromBytes(res[0]), types.NewBlockHeightFromBytes(res[1])

	res, err = api.MessageQuery(
		ctx,
		address.Undef,
		minerAddress,
		"getProvingSetCommitments",
	)
	if err != nil {
		return MinerProvingPeriodResult{}, errors.Wrap(err, "query SetCommitments method failed")
	}

	sig, err := api.ActorGetSignature(ctx, minerAddress, "getProvingSetCommitments")
	if err != nil {
		return MinerProvingPeriodResult{}, errors.Wrap(err, "query method failed")
	}

	commitmentsVal, err := abi.Deserialize(res[0], sig.Return[0])
	if err != nil {
		return MinerProvingPeriodResult{}, errors.Wrap(err, "deserialization failed")
	}
	commitments, ok := commitmentsVal.Val.(map[string]types.Commitments)
	if !ok {
		return MinerProvingPeriodResult{}, errors.Wrap(err, "type assertion failed")
	}

	return MinerProvingPeriodResult{
		Start: *start,
		End:   *end,
		Set:   commitments,
	}, nil
}

// GetMinerOwner return the address owning `minerAddress`
func GetMinerOwner(ctx context.Context, minerAddress address.Address, api *porcelain.API) (address.Address, error) {
	bytes, err := api.MessageQuery(
		ctx,
		address.Undef,
		minerAddress,
		"getOwner",
	)
	if err != nil {
		return address.Undef, err
	}
	ownerAddr, err := address.NewFromBytes(bytes[0])
	if err != nil {
		return address.Undef, err
	}
	return ownerAddr, nil
}

// MinerPowerResult contains the output of running GetMinerPower
// contains the miners power and total power of the network.
type MinerPowerResult struct {
	Power *types.BytesAmount
	Total *types.BytesAmount
}

// GetMinerPower returns the power of miner `minerAddress`
func GetMinerPower(ctx context.Context, minerAddress address.Address, api *porcelain.API) (MinerPowerResult, error) {
	bytes, err := api.MessageQuery(
		ctx,
		address.Undef,
		minerAddress,
		"getPower",
	)
	if err != nil {
		return MinerPowerResult{}, err
	}
	power := types.NewBytesAmountFromBytes(bytes[0])
	if power.Equal(types.NewBytesAmount(0)) {
		power = types.NewBytesAmount(0)
	}

	bytes, err = api.MessageQuery(
		ctx,
		address.Undef,
		address.StorageMarketAddress,
		"getTotalStorage",
	)
	if err != nil {
		return MinerPowerResult{}, err
	}
	total := types.NewBytesAmountFromBytes(bytes[0])

	return MinerPowerResult{
		Power: power,
		Total: total,
	}, nil
}

// GetMinerCollateral returns the amount of collateral pledged by `minerAddress`
func GetMinerCollateral(ctx context.Context, minerAddress address.Address, api *porcelain.API) (types.AttoFIL, error) {
	rets, err := api.MessageQuery(
		ctx,
		address.Undef,
		minerAddress,
		"getActiveCollateral",
	)
	if err != nil {
		return types.AttoFIL{}, err
	}
	return types.NewAttoFILFromBytes(rets[0]), nil
}

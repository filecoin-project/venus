package commands

import (
	"context"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/types"
)

// GetMinerProvingPeriod returns a MinerProvignPeriodResult for the miner at `minerAddress`.
func GetMinerProvingPeriod(ctx context.Context, minerAddress address.Address, api *porcelain.API) (*MinerProvingPeriodResult, error) {
	res, err := api.MessageQuery(
		ctx,
		address.Undef,
		minerAddress,
		"getProvingPeriod",
	)
	if err != nil {
		return nil, errors.Wrap(err, "query ProvingPeriod method failed")
	}
	start, end := types.NewBlockHeightFromBytes(res[0]), types.NewBlockHeightFromBytes(res[1])

	res, err = api.MessageQuery(
		ctx,
		address.Undef,
		minerAddress,
		"getProvingSetCommitments",
	)
	if err != nil {
		return nil, errors.Wrap(err, "query SetCommitments method failed")
	}

	sig, err := api.ActorGetSignature(ctx, minerAddress, "getProvingSetCommitments")
	if err != nil {
		return nil, errors.Wrap(err, "query method failed")
	}

	commitmentsVal, err := abi.Deserialize(res[0], sig.Return[0])
	if err != nil {
		return nil, errors.Wrap(err, "deserialization failed")
	}
	commitments, ok := commitmentsVal.Val.(map[string]types.Commitments)
	if !ok {
		return nil, errors.Wrap(err, "type assertion failed")
	}

	return &MinerProvingPeriodResult{
		Start: start,
		End:   end,
		Set:   commitments,
	}, nil
}

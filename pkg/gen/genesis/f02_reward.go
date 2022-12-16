package genesis

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-state-types/manifest"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/reward"
	"github.com/filecoin-project/venus/venus-shared/types"

	actorstypes "github.com/filecoin-project/go-state-types/actors"
	"github.com/filecoin-project/go-state-types/big"
	cbor "github.com/ipfs/go-ipld-cbor"

	bstore "github.com/filecoin-project/venus/venus-shared/blockstore"
)

func SetupRewardActor(ctx context.Context, bs bstore.Blockstore, qaPower big.Int, av actorstypes.Version) (*types.Actor, error) {
	cst := cbor.NewCborStore(bs)
	rst, err := reward.MakeState(adt.WrapStore(ctx, cst), av, qaPower)
	if err != nil {
		return nil, err
	}

	statecid, err := cst.Put(ctx, rst.GetState())
	if err != nil {
		return nil, err
	}

	actcid, found := actors.GetActorCodeID(av, manifest.RewardKey)
	if !found {
		return nil, fmt.Errorf("failed to get reward actor code ID for actors version %d", av)
	}

	act := &types.Actor{
		Code:    actcid,
		Balance: types.BigInt{Int: constants.InitialRewardBalance},
		Head:    statecid,
	}

	return act, nil
}

package genesis

import (
	"context"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/reward"

	"github.com/filecoin-project/go-state-types/big"

	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/venus/pkg/types"
	bstore "github.com/filecoin-project/venus/pkg/util/blockstoreutil"
)

func SetupRewardActor(ctx context.Context, bs bstore.Blockstore, qaPower big.Int, av actors.Version) (*types.Actor, error) {
	cst := cbor.NewCborStore(bs)
	rst, err := reward.MakeState(adt.WrapStore(ctx, cst), av, qaPower)
	if err != nil {
		return nil, err
	}

	statecid, err := cst.Put(ctx, rst.GetState())
	if err != nil {
		return nil, err
	}

	actcid, err := reward.GetActorCodeID(av)
	if err != nil {
		return nil, err
	}

	act := &types.Actor{
		Code:    actcid,
		Balance: types.BigInt{Int: constants.InitialRewardBalance},
		Head:    statecid,
	}

	return act, nil
}

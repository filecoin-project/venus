package genesis

import (
	"context"

	"github.com/filecoin-project/go-state-types/big"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/market"

	"github.com/filecoin-project/venus/pkg/types"
	bstore "github.com/filecoin-project/venus/pkg/util/blockstoreutil"
)

func SetupStorageMarketActor(ctx context.Context, bs bstore.Blockstore, av actors.Version) (*types.Actor, error) {
	cst := cbor.NewCborStore(bs)
	mst, err := market.MakeState(adt.WrapStore(ctx, cbor.NewCborStore(bs)), av)
	if err != nil {
		return nil, err
	}

	statecid, err := cst.Put(ctx, mst.GetState())
	if err != nil {
		return nil, err
	}

	actcid, err := market.GetActorCodeID(av)
	if err != nil {
		return nil, err
	}

	act := &types.Actor{
		Code:    actcid,
		Head:    statecid,
		Balance: big.Zero(),
	}

	return act, nil
}

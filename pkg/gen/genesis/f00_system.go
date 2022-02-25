package genesis

import (
	"context"

	"github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/system"

	cbor "github.com/ipfs/go-ipld-cbor"

	bstore "github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func SetupSystemActor(ctx context.Context, bs bstore.Blockstore, av actors.Version) (*types.Actor, error) {
	var st system.State

	cst := cbor.NewCborStore(bs)
	st, err := system.MakeState(adt.WrapStore(ctx, cst), av)
	if err != nil {
		return nil, err
	}

	statecid, err := cst.Put(ctx, st.GetState())
	if err != nil {
		return nil, err
	}

	actcid, err := system.GetActorCodeID(av)
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

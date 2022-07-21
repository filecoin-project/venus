package genesis

import (
	"context"
	"fmt"

	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/power"

	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/actors/util/adt"

	cbor "github.com/ipfs/go-ipld-cbor"

	bstore "github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func SetupStoragePowerActor(ctx context.Context, bs bstore.Blockstore, av actors.Version) (*types.Actor, error) {

	cst := cbor.NewCborStore(bs)
	pst, err := power.MakeState(adt.WrapStore(ctx, cbor.NewCborStore(bs)), av)
	if err != nil {
		return nil, err
	}

	statecid, err := cst.Put(ctx, pst.GetState())
	if err != nil {
		return nil, err
	}

	actcid, found := actors.GetActorCodeID(av, actors.PowerKey)
	if !found {
		return nil, fmt.Errorf("failed to get power actor code ID for actors version %d", av)
	}

	act := &types.Actor{
		Code:    actcid,
		Head:    statecid,
		Balance: big.Zero(),
	}

	return act, nil
}

package genesis

import (
	"context"

	"github.com/filecoin-project/go-state-types/big"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/util/adt"

	"github.com/filecoin-project/venus/pkg/types"
	bstore "github.com/filecoin-project/venus/pkg/util/blockstoreutil"
)

func SetupStorageMarketActor(bs bstore.Blockstore) (*types.Actor, error) {
	store := adt.WrapStore(context.TODO(), cbor.NewCborStore(bs))

	a, err := adt.MakeEmptyArray(store).Root()
	if err != nil {
		return nil, err
	}
	h, err := adt.MakeEmptyMap(store).Root()
	if err != nil {
		return nil, err
	}

	sms := market.ConstructState(a, h, h)

	stcid, err := store.Put(store.Context(), sms)
	if err != nil {
		return nil, err
	}

	act := &types.Actor{
		Code:    builtin.StorageMarketActorCodeID,
		Head:    stcid,
		Balance: big.Zero(),
	}

	return act, nil
}

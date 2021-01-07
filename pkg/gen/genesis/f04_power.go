package genesis

import (
	"context"

	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	power0 "github.com/filecoin-project/specs-actors/actors/builtin/power"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/venus/pkg/types"
	bstore "github.com/filecoin-project/venus/pkg/util/blockstoreutil"
)

func SetupStoragePowerActor(bs bstore.Blockstore) (*types.Actor, error) {
	store := adt.WrapStore(context.TODO(), cbor.NewCborStore(bs))
	emptyMap, err := adt.MakeEmptyMap(store).Root()
	if err != nil {
		return nil, err
	}

	multiMap, err := adt.AsMultimap(store, emptyMap)
	if err != nil {
		return nil, err
	}

	emptyMultiMap, err := multiMap.Root()
	if err != nil {
		return nil, err
	}

	sms := power0.ConstructState(emptyMap, emptyMultiMap)

	stcid, err := store.Put(store.Context(), sms)
	if err != nil {
		return nil, err
	}

	return &types.Actor{
		Code:    builtin.StoragePowerActorCodeID,
		Head:    stcid,
		Nonce:   0,
		Balance: big.NewInt(0),
	}, nil
}

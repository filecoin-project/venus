package genesis

import (
	"context"

	"github.com/filecoin-project/venus/pkg/types/specactors"
	"github.com/filecoin-project/venus/pkg/types/specactors/builtin/verifreg"

	"github.com/filecoin-project/go-address"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/specs-actors/actors/util/adt"

	"github.com/filecoin-project/venus/pkg/types"
	bstore "github.com/filecoin-project/venus/pkg/util/blockstoreutil"
)

var RootVerifierID address.Address

func init() {

	idk, err := address.NewFromString("t080")
	if err != nil {
		panic(err)
	}

	RootVerifierID = idk
}

func SetupVerifiedRegistryActor(ctx context.Context, bs bstore.Blockstore, av specactors.Version) (*types.Actor, error) {
	cst := cbor.NewCborStore(bs)
	vst, err := verifreg.MakeState(adt.WrapStore(ctx, cbor.NewCborStore(bs)), av, RootVerifierID)
	if err != nil {
		return nil, err
	}

	statecid, err := cst.Put(ctx, vst.GetState())
	if err != nil {
		return nil, err
	}

	actcid, err := verifreg.GetActorCodeID(av)
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

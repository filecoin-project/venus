package genesis

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-state-types/big"

	systemtypes "github.com/filecoin-project/go-state-types/builtin/v8/system"

	"github.com/filecoin-project/go-state-types/manifest"

	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/system"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"

	bstore "github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func SetupSystemActor(ctx context.Context, bs bstore.Blockstore, av actors.Version) (*types.Actor, error) {
	var st system.State

	cst := cbor.NewCborStore(bs)
	// TODO pass in built-in actors cid for V8 and later
	st, err := system.MakeState(adt.WrapStore(ctx, cst), av, cid.Undef)
	if err != nil {
		return nil, err
	}

	if av >= actors.Version8 {
		mfCid, ok := actors.GetManifest(av)
		if !ok {
			return nil, fmt.Errorf("missing manifest for actors version %d", av)
		}

		mf := manifest.Manifest{}
		if err := cst.Get(ctx, mfCid, &mf); err != nil {
			return nil, fmt.Errorf("loading manifest for actors version %d: %w", av, err)
		}

		st8 := st.GetState().(*systemtypes.State)
		st8.BuiltinActors = mf.Data
	}

	statecid, err := cst.Put(ctx, st.GetState())
	if err != nil {
		return nil, err
	}

	actcid, found := actors.GetActorCodeID(av, actors.SystemKey)
	if !found {
		return nil, fmt.Errorf("failed to get system actor code ID for actors version %d", av)
	}

	act := &types.Actor{
		Code:    actcid,
		Head:    statecid,
		Balance: big.Zero(),
	}

	return act, nil
}

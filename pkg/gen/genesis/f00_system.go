package genesis

import (
	"context"

	"github.com/filecoin-project/go-state-types/big"
	"golang.org/x/xerrors"

	systemtypes "github.com/filecoin-project/go-state-types/builtin/v8/system"

	"github.com/filecoin-project/go-state-types/manifest"

	"github.com/filecoin-project/venus/venus-shared/actors/builtin"

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

	if av >= actors.Version8 {
		mfCid, ok := actors.GetManifest(av)
		if !ok {
			return nil, xerrors.Errorf("missing manifest for actors version %d", av)
		}

		mf := manifest.Manifest{}
		if err := cst.Get(ctx, mfCid, &mf); err != nil {
			return nil, xerrors.Errorf("loading manifest for actors version %d: %w", av, err)
		}

		st8 := st.GetState().(*systemtypes.State)
		st8.BuiltinActors = mf.Data
	}

	statecid, err := cst.Put(ctx, st.GetState())
	if err != nil {
		return nil, err
	}

	actcid, err := builtin.GetSystemActorCodeID(av)
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

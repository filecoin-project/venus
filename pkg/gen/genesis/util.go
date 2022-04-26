package genesis

import (
	"context"

	"github.com/filecoin-project/venus/pkg/state/tree"

	cbg "github.com/whyrusleeping/cbor-gen"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"

	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func mustEnc(i cbg.CBORMarshaler) []byte {
	enc, err := actors.SerializeParams(i)
	if err != nil {
		panic(err) // ok
	}
	return enc
}

func doExecValue(ctx context.Context, vmi vm.Interpreter, to, from address.Address, value types.BigInt, method abi.MethodNum, params []byte) ([]byte, error) {
	act, find, err := vmi.StateTree().GetActor(ctx, from)
	if err != nil {
		return nil, xerrors.Errorf("doExec failed to get from actor (%s): %w", from, err)
	}

	if !find {
		return nil, xerrors.Errorf("actor (%s) not found", from)
	}

	ret, err := vmi.ApplyImplicitMessage(context.TODO(), &types.Message{
		To:       to,
		From:     from,
		Method:   method,
		Params:   params,
		GasLimit: 1_000_000_000_000_000,
		Value:    value,
		Nonce:    act.Nonce,
	})
	if err != nil {
		return nil, xerrors.Errorf("doExec apply message failed: %w", err)
	}

	if ret.Receipt.ExitCode != 0 {
		return nil, xerrors.Errorf("failed to call method: %w", ret.Receipt.String())
	}

	return ret.Receipt.Return, nil
}

func patchManifestCodeCids(st *tree.State, nv network.Version) error {
	av, err := actors.VersionForNetwork(nv)
	if err != nil {
		return err
	}

	var acts []address.Address
	err = st.ForEach(func(a address.Address, _ *types.Actor) error {
		acts = append(acts, a)
		return nil
	})
	if err != nil {
		return xerrors.Errorf("error collecting actors: %w", err)
	}

	for _, a := range acts {
		err = st.MutateActor(a, func(act *types.Actor) error {
			name := actors.CanonicalName(builtin.ActorNameByCode(act.Code))
			code, ok := actors.GetActorCodeID(av, name)
			if ok {
				act.Code = code
			}
			return nil
		})

		if err != nil {
			return xerrors.Errorf("error mutating actor %s: %w", a, err)
		}
	}

	return nil
}

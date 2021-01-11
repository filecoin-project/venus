package genesis

import (
	"context"

	cbg "github.com/whyrusleeping/cbor-gen"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/venus/pkg/specactors"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm"
)

func mustEnc(i cbg.CBORMarshaler) []byte {
	enc, err := specactors.SerializeParams(i)
	if err != nil {
		panic(err) // ok
	}
	return enc
}

func doExecValue(ctx context.Context, vmi vm.Interpreter, to, from address.Address, value big.Int, method abi.MethodNum, params []byte) ([]byte, error) {
	act, find, err := vmi.StateTree().GetActor(ctx, from)
	if err != nil {
		return nil, xerrors.Errorf("doExec failed to get from actor (%s): %w", from, err)
	}

	if !find {
		return nil, xerrors.Errorf("actor (%s) not found", from)
	}

	ret, err := vmi.ApplyImplicitMessage(&types.UnsignedMessage{
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

	return ret.Receipt.ReturnValue, nil
}

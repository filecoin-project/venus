package genesis

import (
	"context"
	"fmt"

	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func mustEnc(i cbg.CBORMarshaler) []byte {
	enc, err := actors.SerializeParams(i)
	if err != nil {
		panic(err) // ok
	}
	return enc
}

func doExecValue(ctx context.Context, vmi vm.Interface, to, from address.Address, value types.BigInt, method abi.MethodNum, params []byte) ([]byte, error) {
	ret, err := vmi.ApplyImplicitMessage(context.TODO(), &types.Message{
		To:       to,
		From:     from,
		Method:   method,
		Params:   params,
		GasLimit: 1_000_000_000_000_000,
		Value:    value,
		Nonce:    0,
	})
	if err != nil {
		return nil, fmt.Errorf("doExec apply message failed: %w", err)
	}

	if ret.Receipt.ExitCode != 0 {
		return nil, fmt.Errorf("failed to call method: %s", ret.Receipt.String())
	}

	return ret.Receipt.Return, nil
}

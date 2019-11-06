// Package actor implements tooling to write and manipulate actors in go.
package actor

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm2/vminternal/dispatch"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm2/vminternal/runtime"
)

// MakeTypedExport finds the correct method on the given actor and returns it.
// The returned function is wrapped such that it takes care of serialization and type checks.
//
// TODO: the work of creating the wrapper should be ideally done at compile time, otherwise at least only once + cached
// TODO: find a better name, naming is hard..
// TODO: Ensure the method is not empty. We need to be paranoid we're not calling methods on transfer messages.
func MakeTypedExport(actor dispatch.ExecutableActor, method types.MethodID) (dispatch.ExportedFunc, bool) {
	fn, signature, ok := actor.Method(method)
	if !ok {
		return nil, false
	}

	t := fn.Type()

	badImpl := func() {
		params := []string{"runtime.Runtime"}
		for _, p := range signature.Params {
			params = append(params, p.String())
		}
		ret := []string{}
		for _, r := range signature.Return {
			ret = append(ret, r.String())
		}
		ret = append(ret, "uint8", "error")
		sig := fmt.Sprintf("func (%s) (%s)", strings.Join(params, ", "), strings.Join(ret, ", "))
		panic(fmt.Sprintf("MakeTypedExport must receive a function with signature: %s, but got: %s", sig, t))
	}

	// The implementation funtction does not have the same signature as the one described by the actor:
	// - signature input vs. impl input: K.. vs. context, K.. = (k..).len() + 1
	// - signature output vs. impl output: T vs. (T, exit code, error) = out.len() + 2
	if t.Kind() != reflect.Func || t.NumIn() != len(signature.Params)+1 || t.NumOut() != len(signature.Return)+2 {
		badImpl()
	}

	for i, p := range signature.Params {
		if !abi.TypeMatches(p, t.In(i+1)) {
			badImpl()
		}
	}

	for i, r := range signature.Return {
		if !abi.TypeMatches(r, t.Out(i)) {
			badImpl()
		}
	}

	exitType := reflect.Uint8
	errorType := reflect.TypeOf((*error)(nil)).Elem()

	if t.Out(t.NumOut()-2).Kind() != exitType {
		badImpl()
	}

	if !t.Out(t.NumOut() - 1).Implements(errorType) {
		badImpl()
	}

	return func(ctx runtime.Runtime) ([]byte, uint8, error) {
		params, err := abi.DecodeValues(ctx.Message().Params, signature.Params)
		if err != nil {
			return nil, 1, errors.RevertErrorWrap(err, "invalid params")
		}

		args := []reflect.Value{
			reflect.ValueOf(ctx),
		}

		for _, param := range params {
			args = append(args, reflect.ValueOf(param.Val))
		}

		out := fn.Call(args)
		exitCode := uint8(out[len(out)-2].Uint())

		outErr, ok := out[len(out)-1].Interface().(error)
		if ok {
			if !(errors.ShouldRevert(outErr) || errors.IsFault(outErr)) {
				var paramStr []string
				for _, param := range params {
					paramStr = append(paramStr, param.String())
				}
				msg := fmt.Sprintf("actor: %#+v, method: %v, args: %v, error: %s", actor, method, paramStr, outErr.Error())
				panic(fmt.Sprintf("you are a bad person: error must be either a reverterror or a fault: %v", msg))
			}

			return nil, exitCode, outErr
		}

		vals := make([]interface{}, 0, len(out)-2)
		for _, vv := range out[:len(out)-2] {
			vals = append(vals, vv.Interface())
		}
		retVal, err := abi.ToEncodedValues(vals...)
		if err != nil {
			return nil, 1, errors.FaultErrorWrap(err, "failed to marshal output value")
		}

		return retVal, exitCode, nil
	}, true
}

// MarshalValue serializes a given go type into a byte slice.
// The returned format matches the format that is expected to be interoperapble between VM and
// the rest of the system.
func MarshalValue(val interface{}) ([]byte, error) {
	switch t := val.(type) {
	case *big.Int:
		if t == nil {
			return []byte{}, nil
		}
		return t.Bytes(), nil
	case *types.ChannelID:
		if t == nil {
			return []byte{}, nil
		}
		return t.Bytes(), nil
	case *types.BlockHeight:
		if t == nil {
			return []byte{}, nil
		}
		return t.Bytes(), nil
	case []byte:
		return t, nil
	case string:
		return []byte(t), nil
	case address.Address:
		if t.Empty() {
			return []byte{}, nil
		}
		return t.Bytes(), nil
	default:
		return nil, fmt.Errorf("unknown type: %s", reflect.TypeOf(t))
	}
}

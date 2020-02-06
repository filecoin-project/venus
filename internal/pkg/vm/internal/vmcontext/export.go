package vmcontext

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
)

// ExportedFunc is the signature an exported method of an actor is expected to have.
type ExportedFunc func(ctx ExportContext) ([]interface{}, uint8, error)

// ExportContext is the context forwarded to the actors.
type ExportContext interface {
	Params() []byte
}

// makeTypedExport finds the correct method on the given actor and returns it.
// The returned function is wrapped such that it takes care of serialization and type checks.
//
// TODO: the work of creating the wrapper should be ideally done at compile time, otherwise at least only once + cached
// TODO: find a better name, naming is hard..
// TODO: Ensure the method is not empty. We need to be paranoid we're not calling methods on transfer messages.
func makeTypedExport(actor dispatch.ExecutableActor, method types.MethodID) (ExportedFunc, bool) {
	fn, signature, ok := actor.Method(method)
	if !ok {
		return nil, false
	}

	t := fn.Type()

	badImpl := func() {
		params := []string{"runtime.InvocationContext"}
		for _, p := range signature.Params {
			params = append(params, p.String())
		}
		ret := []string{}
		for _, r := range signature.Return {
			ret = append(ret, r.String())
		}
		ret = append(ret, "uint8", "error")
		sig := fmt.Sprintf("func (%s) (%s)", strings.Join(params, ", "), strings.Join(ret, ", "))
		panic(fmt.Sprintf("makeTypedExport must receive a function with signature: %s, but got: %s", sig, t))
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

	return func(ctx ExportContext) ([]interface{}, uint8, error) {
		params, err := abi.DecodeValues(ctx.Params(), signature.Params)
		if err != nil {
			return nil, 1, fmt.Errorf("invalid params")
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
			return nil, exitCode, outErr
		}

		vals := make([]interface{}, 0, len(out)-2)
		for _, vv := range out[:len(out)-2] {
			vals = append(vals, vv.Interface())
		}
		return vals, exitCode, nil
	}, true
}

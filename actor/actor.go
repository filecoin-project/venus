// Package actor implements tooling to write and manipulate actors in go.
package actor

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	cbor "gx/ipfs/QmPbqRavwDZLfmpeW6eoyAoQ5rT2LoCW98JhvRc22CqkZS/go-ipld-cbor"
	"gx/ipfs/QmSkuaNgyGmV8c1L3cZNWcUxRJV6J3nsD96JVQPcWcwtyW/go-hamt-ipld"
	block "gx/ipfs/QmVzK524a2VWLqyvtBeiHKsUAWYgeAk4DBeZoY7vpNPNRx/go-block-format"
	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

// MakeTypedExport finds the correct method on the given actor and returns it.
// The returned function is wrapped such that it takes care of serialization and type checks.
//
// TODO: the work of creating the wrapper should be ideally done at compile time, otherwise at least only once + cached
// TODO: find a better name, naming is hard..
// TODO: Ensure the method is not empty. We need to be paranoid we're not calling methods on transfer messages.
func MakeTypedExport(actor exec.ExecutableActor, method string) exec.ExportedFunc {
	f, ok := reflect.TypeOf(actor).MethodByName(strings.Title(method))
	if !ok {
		panic(fmt.Sprintf("MakeTypedExport could not find passed in method in actor: %s", method))
	}

	exports := actor.Exports()
	signature, ok := exports[method]
	if !ok {
		panic(fmt.Sprintf("MakeTypedExport could not find passed in method in exports: %s", method))
	}

	val := f.Func
	t := f.Type

	badImpl := func() {
		params := []string{"exec.VMContext"}
		for _, p := range signature.Params {
			params = append(params, p.String())
		}
		ret := []string{}
		for _, r := range signature.Return {
			ret = append(ret, r.String())
		}
		ret = append(ret, "uint8", "error")
		sig := fmt.Sprintf("func (Actor, %s) (%s)", strings.Join(params, ", "), strings.Join(ret, ", "))
		panic(fmt.Sprintf("MakeTypedExport must receive a function with signature: %s, but got: %s", sig, t))
	}

	if t.Kind() != reflect.Func || t.NumIn() != 2+len(signature.Params) || t.NumOut() != 2+len(signature.Return) {
		badImpl()
	}

	for i, p := range signature.Params {
		if !abi.TypeMatches(p, t.In(i+2)) {
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

	return func(ctx exec.VMContext) ([]byte, uint8, error) {
		params, err := abi.DecodeValues(ctx.Message().Params, signature.Params)
		if err != nil {
			return nil, 1, errors.RevertErrorWrap(err, "invalid params")
		}

		args := []reflect.Value{
			reflect.ValueOf(actor),
			reflect.ValueOf(ctx),
		}

		for _, param := range params {
			args = append(args, reflect.ValueOf(param.Val))
		}

		toInterfaces := func(v []reflect.Value) []interface{} {
			r := make([]interface{}, 0, len(v))
			for _, vv := range v {
				r = append(r, vv.Interface())
			}
			return r
		}

		out := toInterfaces(val.Call(args))

		exitCode, ok := out[len(out)-2].(uint8)
		if !ok {
			panic("invalid return value")
		}

		var retVal []byte
		outErr, ok := out[len(out)-1].(error)
		if ok {
			if !(errors.ShouldRevert(outErr) || errors.IsFault(outErr)) {
				panic("you are a bad person: error must be either a reverterror or a fault")
			}
		} else {
			// The value of the returned error was nil.
			outErr = nil

			retVal, err = abi.ToEncodedValues(out[:len(out)-2]...)
			if err != nil {
				return nil, 1, errors.FaultErrorWrap(err, "failed to marshal output value")
			}
		}

		return retVal, exitCode, outErr
	}
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
	case types.Address:
		if t == (types.Address{}) {
			return []byte{}, nil
		}
		return t.Bytes(), nil
	default:
		return nil, fmt.Errorf("unknown type: %s", reflect.TypeOf(t))
	}
}

// --
// Below are helper functions that are used to implement actors.

// MarshalStorage encodes the passed in data into bytes.
func MarshalStorage(in interface{}) ([]byte, error) {
	bytes, err := cbor.DumpObject(in)
	if err != nil {
		return nil, errors.FaultErrorWrap(err, "Could not marshall actor state")
	}
	return bytes, nil
}

// UnmarshalStorage decodes the passed in bytes into the given object.
func UnmarshalStorage(raw []byte, to interface{}) error {
	return cbor.DecodeInto(raw, to)
}

// WithState is a helper method that makes dealing with storage serialization
// easier for implementors.
// It is designed to be used like:
//
// var st MyStorage
// ret, err := WithState(ctx, &st, func() (interface{}, error) {
//   fmt.Println("hey look, my storage is loaded: ", st)
//   return st.Thing, nil
// })
//
// Note that if 'f' returns an error, modifications to the storage are not
// saved.
func WithState(ctx exec.VMContext, st interface{}, f func() (interface{}, error)) (interface{}, error) {
	chunk, err := ctx.ReadStorage()
	if err != nil {
		return nil, err
	}

	if err := UnmarshalStorage(chunk, st); err != nil {
		return nil, err
	}

	ret, err := f()
	if err != nil {
		return nil, err
	}

	data, err := MarshalStorage(st)
	if err != nil {
		return nil, err
	}

	if err := ctx.WriteStorage(data); err != nil {
		return nil, err
	}

	return ret, nil
}

// SetKeyValue convenience method to load a lookup, set one key value pair and commit.
// This function is inefficient when multiple values need to be set into the lookup.
func SetKeyValue(ctx context.Context, storage exec.Storage, id *cid.Cid, key string, value interface{}) (*cid.Cid, error) {
	lookup, err := LoadLookup(ctx, storage, id)
	if err != nil {
		return nil, err
	}

	err = lookup.Set(ctx, key, value)
	if err != nil {
		return nil, err
	}

	return lookup.Commit(ctx)
}

// LoadLookup loads hamt-ipld node from storage if the cid exists, or creates a new on if it is nil.
// The lookup provides access to a HAMT/CHAMP tree stored in storage.
func LoadLookup(ctx context.Context, storage exec.Storage, cid *cid.Cid) (exec.Lookup, error) {
	cborStore := &hamt.CborIpldStore{
		Blocks: &storageAsBlocks{s: storage},
		Atlas:  &cbor.CborAtlas,
	}
	var root *hamt.Node
	var err error

	if cid == nil {
		root = hamt.NewNode(cborStore)
	} else {
		root, err = hamt.LoadNode(ctx, cborStore, cid)
		if err != nil {
			return nil, err
		}
	}

	return &lookup{n: root, s: storage}, nil
}

// storageAsBlocks allows us to use an exec.Storage as a Blockstore
type storageAsBlocks struct {
	s exec.Storage
}

// GetBlock gets a block from underlying storage by cid
func (sab *storageAsBlocks) GetBlock(ctx context.Context, c *cid.Cid) (block.Block, error) {
	chunk, err := sab.s.Get(c)
	if err != nil {
		return nil, err
	}

	return block.NewBlock(chunk), nil
}

// AddBlock add a block to underlying storage
func (sab *storageAsBlocks) AddBlock(b block.Block) error {
	_, err := sab.s.Put(b.RawData())
	return err
}

// lookup implements exec.Lookup and provides structured key-value storage for actors
type lookup struct {
	n *hamt.Node
	s exec.Storage
}

var _ exec.Lookup = (*lookup)(nil)

// Find retrieves a value by key
func (l *lookup) Find(ctx context.Context, k string) (interface{}, error) {
	return l.n.Find(ctx, k)
}

// Set adds a value under the given key
func (l *lookup) Set(ctx context.Context, k string, v interface{}) error {
	return l.n.Set(ctx, k, v)
}

// Commit ensures all data in the tree is flushed to storage and returns the cid of the head node.
func (l *lookup) Commit(ctx context.Context) (*cid.Cid, error) {
	if err := l.n.Flush(ctx); err != nil {
		return nil, err
	}

	chunk, err := cbor.DumpObject(l.n)
	if err != nil {
		return nil, err
	}

	return l.s.Put(chunk)
}

// Values returns a slice of all key-values stored in the lookup
func (l *lookup) Values(ctx context.Context) ([]*hamt.KV, error) {
	return l.values(ctx, []*hamt.KV{})
}

func (l *lookup) values(ctx context.Context, vs []*hamt.KV) ([]*hamt.KV, error) {
	for _, p := range l.n.Pointers {
		vs = append(vs, p.KVs...)

		if p.Link == nil {
			continue
		}

		subtree, err := LoadLookup(ctx, l.s, p.Link)
		if err != nil {
			return nil, err
		}

		sublookup, ok := subtree.(*lookup)
		if !ok {
			return nil, errors.NewFaultError("Non-actor.lookup found in hamt tree")
		}

		vs, err = sublookup.values(ctx, vs)
		if err != nil {
			return nil, err
		}
	}
	return vs, nil
}

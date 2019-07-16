package actor

import (
	"context"
	"reflect"

	block "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/polydawn/refmt/obj"
	"github.com/polydawn/refmt/shared"

	"github.com/filecoin-project/go-filecoin/exec"
	vmerrors "github.com/filecoin-project/go-filecoin/vm/errors"
)

// MarshalStorage encodes the passed in data into bytes.
func MarshalStorage(in interface{}) ([]byte, error) {
	return cbor.DumpObject(in)
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
	if err := ReadState(ctx, st); err != nil {
		return nil, err
	}

	ret, err := f()
	if err != nil {
		return nil, err
	}

	stage := ctx.Storage()

	cid, err := stage.Put(st)
	if err != nil {
		return nil, vmerrors.RevertErrorWrap(err, "Could not stage memory chunk")
	}

	err = stage.Commit(cid, stage.Head())
	if err != nil {
		return nil, vmerrors.RevertErrorWrap(err, "Could not commit actor memory")
	}

	return ret, nil
}

// ReadState is a helper method to read the cbor node at the actor's Head into the given struct
func ReadState(ctx exec.VMContext, st interface{}) error {
	storage := ctx.Storage()

	memory, err := storage.Get(storage.Head())
	if err != nil {
		return vmerrors.FaultErrorWrap(err, "Could not read actor storage")
	}

	if err := UnmarshalStorage(memory, st); err != nil {
		return vmerrors.FaultErrorWrap(err, "Could not unmarshall actor storage")
	}

	return nil
}

// SetKeyValue convenience method to load a lookup, set one key value pair and commit.
// This function is inefficient when multiple values need to be set into the lookup.
func SetKeyValue(ctx context.Context, storage exec.Storage, id cid.Cid, key string, value interface{}) (cid.Cid, error) {
	lookup, err := LoadLookup(ctx, storage, id)
	if err != nil {
		return cid.Undef, err
	}

	err = lookup.Set(ctx, key, value)
	if err != nil {
		return cid.Undef, err
	}

	return lookup.Commit(ctx)
}

// WithLookup allows one to read and write to a hamt-ipld node from storage via a callback function.
// This function commits the lookup before returning.
func WithLookup(ctx context.Context, storage exec.Storage, id cid.Cid, f func(exec.Lookup) error) (cid.Cid, error) {
	lookup, err := LoadLookup(ctx, storage, id)
	if err != nil {
		return cid.Undef, err
	}

	if err = f(lookup); err != nil {
		return cid.Undef, err
	}

	return lookup.Commit(ctx)
}

// WithLookupForReading allows one to read from a hamt-ipld node from storage via a callback function.
// Unlike WithLookup, this function will not attempt to commit.
func WithLookupForReading(ctx context.Context, storage exec.Storage, id cid.Cid, f func(exec.Lookup) error) error {
	lookup, err := LoadLookup(ctx, storage, id)
	if err != nil {
		return err
	}

	return f(lookup)
}

// LoadLookup loads hamt-ipld node from storage if the cid exists, or creates a new one if it is nil.
// The lookup provides access to a HAMT/CHAMP tree stored in storage.
func LoadLookup(ctx context.Context, storage exec.Storage, cid cid.Cid) (exec.Lookup, error) {
	return LoadTypedLookup(ctx, storage, cid, nil)
}

// LoadTypedLookup loads hamt-ipld node from storage if the cid exists, or creates a new one if it is nil.
// The provided type allows the lookup to correctly unmarshal values
func LoadTypedLookup(ctx context.Context, storage exec.Storage, cid cid.Cid, valueType interface{}) (exec.Lookup, error) {
	cborStore := &hamt.CborIpldStore{
		Blocks: &storageAsBlocks{s: storage},
		Atlas:  &cbor.CborAtlas,
	}
	var root *hamt.Node
	var err error

	if !cid.Defined() {
		root = hamt.NewNode(cborStore)
	} else {
		root, err = hamt.LoadNode(ctx, cborStore, cid)
		if err != nil {
			return nil, err
		}
	}

	var vt reflect.Type
	if valueType != nil {
		vt = reflect.TypeOf(valueType)
	}

	return &lookup{n: root, s: storage, t: vt}, nil
}

// storageAsBlocks allows us to use an exec.Storage as a Blockstore
type storageAsBlocks struct {
	s exec.Storage
}

// GetBlock gets a block from underlying storage by cid
func (sab *storageAsBlocks) GetBlock(ctx context.Context, c cid.Cid) (block.Block, error) {
	chunk, err := sab.s.Get(c)
	if err != nil {
		return nil, err
	}

	return block.NewBlock(chunk), nil
}

// AddBlock add a block to underlying storage
func (sab *storageAsBlocks) AddBlock(b block.Block) error {
	_, err := sab.s.Put(b)
	return err
}

// lookup implements exec.Lookup and provides structured key-value storage for actors
type lookup struct {
	n *hamt.Node
	s exec.Storage
	t reflect.Type
}

var _ exec.Lookup = (*lookup)(nil)

// Find retrieves a value by key
// If the return value is not primitive, you will need to load the lookup using the LoadTypedLookup
// to ensure the return value is correctly unmarshaled.
func (l *lookup) Find(ctx context.Context, k string) (interface{}, error) {
	value, err := l.n.Find(ctx, k)
	if err != nil {
		return nil, err
	}

	// correct type of response
	if l.t != nil {
		value, err = newCorrectlyUnmarshaled(value, l.t)
		if err != nil {
			return nil, err
		}
	}

	return value, nil
}

// Set adds a value under the given key
func (l *lookup) Set(ctx context.Context, k string, v interface{}) error {
	return l.n.Set(ctx, k, v)
}

// Delete removes a key value from the lookup
func (l *lookup) Delete(ctx context.Context, k string) error {
	return l.n.Delete(ctx, k)
}

// Commit ensures all data in the tree is flushed to storage and returns the cid of the head node.
func (l *lookup) Commit(ctx context.Context) (cid.Cid, error) {
	if err := l.n.Flush(ctx); err != nil {
		return cid.Undef, err
	}

	return l.s.Put(l.n)
}

// IsEmpty returns true if this node contains no key values
func (l *lookup) IsEmpty() bool {
	return len(l.n.Pointers) == 0
}

// Values returns a slice of all key-values stored in the lookup
func (l *lookup) Values(ctx context.Context) ([]*hamt.KV, error) {
	kvs, err := l.values(ctx, []*hamt.KV{})
	if err != nil {
		return nil, err
	}

	// The values coming out of the hamt are not correctly unmarshaled. Correct that now.
	if l.t != nil {
		for _, kv := range kvs {
			kv.Value, err = newCorrectlyUnmarshaled(kv.Value, l.t)
			if err != nil {
				return nil, err
			}
		}
	}

	return kvs, nil
}

// values recursively traverses the hamt and retreives all the key value pairs.
func (l *lookup) values(ctx context.Context, vs []*hamt.KV) ([]*hamt.KV, error) {
	for _, p := range l.n.Pointers {
		vs = append(vs, p.KVs...)

		if !p.Link.Defined() {
			continue
		}

		subtree, err := LoadLookup(ctx, l.s, p.Link)
		if err != nil {
			return nil, err
		}

		sublookup, ok := subtree.(*lookup)
		if !ok {
			return nil, vmerrors.NewFaultError("Non-actor.lookup found in hamt tree")
		}

		vs, err = sublookup.values(ctx, vs)
		if err != nil {
			return nil, err
		}
	}
	return vs, nil
}

// newCorrectlyUnmarshaled creates a interface of the correct type, unmarshals into it, and returns the value
func newCorrectlyUnmarshaled(from interface{}, valueType reflect.Type) (interface{}, error) {
	to := reflect.New(valueType).Interface()
	err := correctUnmarshaling(from, to)
	if err != nil {
		return nil, err
	}
	return reflect.ValueOf(to).Elem().Interface(), nil
}

// correctUnmarshaling uses refmt to translate between a map and the given struct interface
func correctUnmarshaling(from, to interface{}) error {
	m := obj.NewMarshaller(cbor.CborAtlas)
	if err := m.Bind(from); err != nil {
		return err
	}

	u := obj.NewUnmarshaller(cbor.CborAtlas)
	if err := u.Bind(to); err != nil {
		return err
	}

	return shared.TokenPump{
		TokenSource: m,
		TokenSink:   u,
	}.Run()
}

package actor

import (
	"context"
	"reflect"

	block "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	cbor "github.com/ipfs/go-ipld-cbor"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	vmerrors "github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storage"
)

const (
	// TreeBitWidth is the bit width of the HAMT used to by an actor to
	// store its state
	TreeBitWidth = 5
)

// MarshalStorage encodes the passed in data into bytes.
func MarshalStorage(in interface{}) ([]byte, error) {
	return encoding.Encode(in)
}

// UnmarshalStorage decodes the passed in bytes into the given object.
func UnmarshalStorage(raw []byte, to interface{}) error {
	return encoding.Decode(raw, to)
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
func WithState(ctx runtime.InvocationContext, st interface{}, f func() (interface{}, error)) (interface{}, error) {
	if err := ReadState(ctx, st); err != nil {
		return nil, err
	}

	ret, err := f()
	if err != nil {
		return nil, err
	}

	stage := ctx.Runtime().LegacyStorage()

	cid, err := stage.Put(st)
	if err != nil {
		return nil, vmerrors.RevertErrorWrap(err, "Could not stage memory chunk")
	}

	err = stage.LegacyCommit(cid, stage.LegacyHead())
	if err != nil {
		return nil, vmerrors.RevertErrorWrap(err, "Could not commit actor memory")
	}

	return ret, nil
}

// ReadState is a helper method to read the cbor node at the actor's Head into the given struct
func ReadState(ctx runtime.InvocationContext, st interface{}) error {
	storage := ctx.Runtime().LegacyStorage()

	memory, err := storage.Get(storage.LegacyHead())
	if err != nil {
		return vmerrors.FaultErrorWrap(err, "Could not read actor storage")
	}

	if err := UnmarshalStorage(memory, st); err != nil {
		return vmerrors.FaultErrorWrap(err, "Could not unmarshall actor storage")
	}

	return nil
}

// WriteState stores state and commits it as the actor's head
func WriteState(ctx runtime.InvocationContext, state interface{}) error {
	stage := ctx.Runtime().LegacyStorage()

	cid, err := stage.Put(state)
	if err != nil {
		return vmerrors.RevertErrorWrap(err, "Could not stage memory chunk")
	}

	err = stage.LegacyCommit(cid, stage.LegacyHead())
	if err != nil {
		return vmerrors.RevertErrorWrap(err, "Could not commit actor memory")
	}

	return nil
}

// SetKeyValue convenience method to load a lookup, set one key value pair and commit.
// This function is inefficient when multiple values need to be set into the lookup.
func SetKeyValue(ctx context.Context, storage runtime.Storage, id cid.Cid, key string, value interface{}) (cid.Cid, error) {
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
func WithLookup(ctx context.Context, storage runtime.Storage, id cid.Cid, f func(storage.Lookup) error) (cid.Cid, error) {
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
func WithLookupForReading(ctx context.Context, storage runtime.Storage, id cid.Cid, f func(storage.Lookup) error) error {
	lookup, err := LoadLookup(ctx, storage, id)
	if err != nil {
		return err
	}

	return f(lookup)
}

// LoadLookup loads hamt-ipld node from storage if the cid exists, or creates a new one if it is nil.
// The lookup provides access to a HAMT/CHAMP tree stored in storage.
func LoadLookup(ctx context.Context, storage runtime.Storage, cid cid.Cid) (storage.Lookup, error) {
	cborStore := &hamt.CborIpldStore{
		Blocks: &storageAsBlocks{s: storage},
		Atlas:  &cbor.CborAtlas,
	}
	var root *hamt.Node
	var err error

	if !cid.Defined() {
		root = hamt.NewNode(cborStore, hamt.UseTreeBitWidth(TreeBitWidth))
	} else {
		root, err = hamt.LoadNode(ctx, cborStore, cid, hamt.UseTreeBitWidth(TreeBitWidth))
		if err != nil {
			return nil, err
		}
	}

	return &lookup{n: root, s: storage}, nil
}

// storageAsBlocks allows us to use an runtime.LegacyStorage as a Blockstore
type storageAsBlocks struct {
	s runtime.Storage
}

// GetBlock gets a block from underlying storage by cid
func (sab *storageAsBlocks) GetBlock(ctx context.Context, c cid.Cid) (block.Block, error) {
	chunk, _ := sab.s.GetRaw(c)
	return block.NewBlock(chunk), nil
}

// AddBlock add a block to underlying storage
func (sab *storageAsBlocks) AddBlock(b block.Block) error {
	sab.s.Put(b)
	return nil
}

// lookup implements storage.Lookup and provides structured key-value storage for actors
type lookup struct {
	n *hamt.Node
	s runtime.Storage
}

var _ storage.Lookup = (*lookup)(nil)

// Find retrieves a value by key
// If the return value is not primitive, you will need to load the lookup using the LoadTypedLookup
// to ensure the return value is correctly unmarshaled.
func (l *lookup) Find(ctx context.Context, k string, out interface{}) error {
	return l.n.Find(ctx, k, out)
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
		panic(err)
	}

	return l.s.Put(l.n), nil
}

// IsEmpty returns true if this node contains no key values
func (l *lookup) IsEmpty() bool {
	return len(l.n.Pointers) == 0
}

// ForEachValue iterates all the values in a lookup
func (l *lookup) ForEachValue(ctx context.Context, valueType interface{}, callback storage.ValueCallbackFunc) error {
	var vt reflect.Type
	if valueType != nil {
		vt = reflect.TypeOf(valueType)
	}

	// The values coming out of the hamt are not correctly unmarshaled. Correct that now.
	return l.n.ForEach(ctx, func(k string, v interface{}) error {
		valueAsDeferred := v.(*cbg.Deferred)
		var decodedValue interface{}
		if vt != nil {
			to := reflect.New(vt).Interface()
			if err := encoding.Decode(valueAsDeferred.Raw, to); err != nil {
				return err
			}
			decodedValue = reflect.ValueOf(to).Elem().Interface()
		}
		if err := callback(k, decodedValue); err != nil {
			return err
		}
		return nil
	})
}

package actor

import (
	"fmt"
	"context"
	"reflect"

	block "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storage"
)

const (
	// TreeBitWidth is the bit width of the HAMT used to by an actor to
	// store its state
	TreeBitWidth = 5
)

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
		fmt.Printf("load lookup error\n")
		return cid.Undef, err
	}

	if err = f(lookup); err != nil {
		fmt.Printf("f lookup error\n")
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
	cborStore := cborutil.NewIpldStore(&storageAsIpldBlockstore{s: storage})
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

// storageAsIpldBlockstore allows us to use an runtime.LegacyStorage as an IpldBlockstore
type storageAsIpldBlockstore struct {
	s runtime.Storage
}

// GetBlock gets a block from underlying storage by cid
func (sab *storageAsIpldBlockstore) Get(c cid.Cid) (block.Block, error) {
	chunk, _ := sab.s.GetRaw(c)
	return block.NewBlock(chunk), nil
}

// AddBlock add a block to underlying storage
func (sab *storageAsIpldBlockstore) Put(b block.Block) error {
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

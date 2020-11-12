package vmcontext

import (
	"bytes"
	"github.com/filecoin-project/go-state-types/cbor"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/venus/internal/pkg/constants"
	"github.com/filecoin-project/venus/internal/pkg/encoding"
	"github.com/ipfs/go-cid"
	cborUtil "github.com/ipfs/go-ipld-cbor"
)

// TestStorage is a fake storage used for testing.
type TestStorage struct {
	state interface{}
}

// NewTestStorage returns a new "TestStorage"
func NewTestStorage(state interface{}) *TestStorage {
	return &TestStorage{
		state: state,
	}
}

var _ specsruntime.Store = (*TestStorage)(nil)

// Put implements runtime.Store.
func (ts *TestStorage) StorePut(v cbor.Marshaler) cid.Cid {
	ts.state = v
	if cm, ok := v.(cbor.Marshaler); ok {
		buf := new(bytes.Buffer)
		err := cm.MarshalCBOR(buf)
		if err == nil {
			return cid.NewCidV1(cid.Raw, buf.Bytes())
		}
	}
	raw, err := encoding.Encode(v)
	if err != nil {
		panic("failed to encode")
	}
	return cid.NewCidV1(cid.Raw, raw)
}

// Get implements runtime.Store.
func (ts *TestStorage) StoreGet(cid cid.Cid, obj cbor.Unmarshaler) bool {
	node, err := cborUtil.WrapObject(ts.state, constants.DefaultHashFunction, -1)
	if err != nil {
		return false
	}

	err = encoding.Decode(node.RawData(), obj)
	if err != nil {
		return false
	}

	return true
}

// CidOf returns the cid of the object.
func (ts *TestStorage) CidOf(obj interface{}) cid.Cid {
	if obj == nil {
		return cid.Undef
	}
	raw, err := encoding.Encode(obj)
	if err != nil {
		panic("failed to encode")
	}
	return cid.NewCidV1(cid.Raw, raw)
}

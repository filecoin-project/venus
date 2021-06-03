package vmcontext

import (
	"bytes"

	"github.com/filecoin-project/go-state-types/cbor"
	rt5 "github.com/filecoin-project/specs-actors/v5/actors/runtime"
	"github.com/filecoin-project/venus/pkg/constants"
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

var _ rt5.Store = (*TestStorage)(nil)

// Put implements runtime.Store.
func (ts *TestStorage) StorePut(v cbor.Marshaler) cid.Cid {
	ts.state = v
	buf := new(bytes.Buffer)
	err := v.MarshalCBOR(buf)
	if err == nil {
		return cid.NewCidV1(cid.Raw, buf.Bytes())
	}
	panic("failed to encode")
}

// Get implements runtime.Store.
func (ts *TestStorage) StoreGet(cid cid.Cid, obj cbor.Unmarshaler) bool {
	node, err := cborUtil.WrapObject(ts.state, constants.DefaultHashFunction, -1)
	if err != nil {
		return false
	}

	err = obj.UnmarshalCBOR(bytes.NewReader(node.RawData()))

	return err == nil
}

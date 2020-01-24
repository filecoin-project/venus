package enccid_test

import (
	"testing"

	. "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	cbor "github.com/fxamacker/cbor"
	cid "github.com/ipfs/go-cid"
	olcbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mh "github.com/multiformats/go-multihash"
)

func TestCid(t *testing.T) {
	prefix := cid.V1Builder{Codec: cid.DagCBOR, MhType: mh.BLAKE2B_MIN + 31}
	c, err := prefix.Sum([]byte("epigram"))
	require.NoError(t, err)
	w := NewCid(c)
	cbytes, err := cbor.Marshal(w, cbor.EncOptions{})
	require.NoError(t, err)

	olcbytes, err := olcbor.DumpObject(c)
	require.NoError(t, err)
	assert.Equal(t, olcbytes, cbytes)
	var rtOlC cid.Cid
	err = olcbor.DecodeInto(olcbytes, &rtOlC)
	require.NoError(t, err)

	var newC Cid
	err = cbor.Unmarshal(cbytes, &newC)
	require.NoError(t, err)
	assert.Equal(t, w, newC)
}

func TestEmptyCid(t *testing.T) {
	nullCid := NewCid(cid.Undef)
	cbytes, err := cbor.Marshal(nullCid, cbor.EncOptions{})
	require.NoError(t, err)

	var retUndefCid Cid
	err = cbor.Unmarshal(cbytes, &retUndefCid)
	require.NoError(t, err)
	assert.True(t, retUndefCid.Equals(cid.Undef))
}

func TestCidJSON(t *testing.T) {
	prefix := cid.V1Builder{Codec: cid.DagCBOR, MhType: mh.BLAKE2B_MIN + 31}
	c, err := prefix.Sum([]byte("epigram"))
	require.NoError(t, err)
	w := NewCid(c)

	jBs, err := w.MarshalJSON()
	require.NoError(t, err)

	var rt Cid
	err = rt.UnmarshalJSON(jBs)
	require.NoError(t, err)
	assert.True(t, rt.Equals(w.Cid))
}

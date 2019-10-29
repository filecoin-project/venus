package block_test

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blk "github.com/filecoin-project/go-filecoin/internal/pkg/block"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

func TestTriangleEncoding(t *testing.T) {
	tf.UnitTest(t)

	// We want to be sure that:
	//      Block => json => Block
	// yields exactly the same thing as:
	//      Block => IPLD node => CBOR => IPLD node => json => IPLD node => Block (!)
	// because we want the output encoding of a Block directly from memory
	// (first case) to be exactly the same as the output encoding of a Block from
	// storage (second case). WTF you might say, and you would not be wrong. The
	// use case is machine-parsing command output. For example dag_daemon_test
	// dumps the block from memory as json (first case). It then dag gets
	// the block by cid which yeilds a json-encoded ipld node (first half of
	// the second case). It json decodes this ipld node and then decodes the node
	// into a block (second half of the second case). I don't claim this is ideal,
	// see: https://github.com/filecoin-project/go-filecoin/issues/599

	newAddress := address.NewForTestGetter()

	testRoundTrip := func(t *testing.T, exp *blk.Block) {
		jb, err := json.Marshal(exp)
		require.NoError(t, err)
		var jsonRoundTrip blk.Block
		err = json.Unmarshal(jb, &jsonRoundTrip)
		require.NoError(t, err)

		ipldNodeOrig, err := encoding.Encode(exp)
		assert.NoError(t, err)
		// NOTICE: skips the intermediate json steps from above.
		var cborJSONRoundTrip blk.Block
		err = encoding.Decode(ipldNodeOrig, &cborJSONRoundTrip)
		assert.NoError(t, err)

		types.AssertHaveSameCid(t, &jsonRoundTrip, &cborJSONRoundTrip)
	}

	t.Run("encoding block with zero fields works", func(t *testing.T) {
		testRoundTrip(t, &blk.Block{})
	})

	t.Run("encoding block with nonzero fields works", func(t *testing.T) {
		// We should ensure that every field is set -- zero values might
		// pass when non-zero values do not due to nil/null encoding.

		b := &blk.Block{
			Miner:           newAddress(),
			Ticket:          blk.Ticket{VRFProof: []byte{0x01, 0x02, 0x03}},
			Height:          types.Uint64(2),
			Messages:        types.TxMeta{SecpRoot: types.CidFromString(t, "somecid"), BLSRoot: types.EmptyMessagesCID},
			MessageReceipts: types.CidFromString(t, "somecid"),
			Parents:         blk.NewTipSetKey(types.CidFromString(t, "somecid")),
			ParentWeight:    types.Uint64(1000),
			ElectionProof:   types.NewTestPoSt(),
			StateRoot:       types.CidFromString(t, "somecid"),
			Timestamp:       types.Uint64(1),
			BlockSig:        []byte{0x3},
			BLSAggregateSig: []byte{0x3},
		}
		s := reflect.TypeOf(*b)
		// This check is here to request that you add a non-zero value for new fields
		// to the above (and update the field count below).
		// Also please add non zero fields to "b" and "diff" in TestSignatureData
		// and add a new check that different values of the new field result in
		// different output data.
		require.Equal(t, 14, s.NumField()) // Note: this also counts private fields
		testRoundTrip(t, b)
	})
}

func TestBlockString(t *testing.T) {
	tf.UnitTest(t)

	var b blk.Block

	cid := b.Cid()

	got := b.String()
	assert.Contains(t, got, cid.String())
}

func TestBlockScore(t *testing.T) {
	tf.UnitTest(t)

	source := rand.NewSource(time.Now().UnixNano())

	t.Run("block score equals block height", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			n := uint64(source.Int63())

			var b blk.Block
			b.Height = types.Uint64(n)

			assert.Equal(t, uint64(b.Height), b.Score(), "block height: %d - block score %d", b.Height, b.Score())
		}
	})
}

func TestDecodeBlock(t *testing.T) {
	tf.UnitTest(t)

	t.Run("successfully decodes raw bytes to a Filecoin block", func(t *testing.T) {
		addrGetter := address.NewForTestGetter()

		c1 := types.CidFromString(t, "a")
		c2 := types.CidFromString(t, "b")
		cM := types.CidFromString(t, "messages")
		cR := types.CidFromString(t, "receipts")

		before := &blk.Block{
			Miner:           addrGetter(),
			Ticket:          blk.Ticket{VRFProof: []uint8{}},
			Parents:         blk.NewTipSetKey(c1),
			Height:          2,
			Messages:        types.TxMeta{SecpRoot: cM, BLSRoot: types.EmptyMessagesCID},
			StateRoot:       c2,
			MessageReceipts: cR,
		}

		after, err := blk.DecodeBlock(before.ToNode().RawData())
		assert.NoError(t, err)
		assert.Equal(t, after.Cid(), before.Cid())
		assert.Equal(t, before, after)
	})

	t.Run("decode failure results in an error", func(t *testing.T) {
		_, err := blk.DecodeBlock([]byte{1, 2, 3})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "malformed stream")
	})
}

func TestEquals(t *testing.T) {
	tf.UnitTest(t)

	c1 := types.CidFromString(t, "a")
	c2 := types.CidFromString(t, "b")

	s1 := types.CidFromString(t, "state1")
	s2 := types.CidFromString(t, "state2")

	var h1 types.Uint64 = 1
	var h2 types.Uint64 = 2

	b1 := &blk.Block{Parents: blk.NewTipSetKey(c1), StateRoot: s1, Height: h1}
	b2 := &blk.Block{Parents: blk.NewTipSetKey(c1), StateRoot: s1, Height: h1}
	b3 := &blk.Block{Parents: blk.NewTipSetKey(c1), StateRoot: s2, Height: h1}
	b4 := &blk.Block{Parents: blk.NewTipSetKey(c2), StateRoot: s1, Height: h1}
	b5 := &blk.Block{Parents: blk.NewTipSetKey(c1), StateRoot: s1, Height: h2}
	b6 := &blk.Block{Parents: blk.NewTipSetKey(c2), StateRoot: s1, Height: h2}
	b7 := &blk.Block{Parents: blk.NewTipSetKey(c1), StateRoot: s2, Height: h2}
	b8 := &blk.Block{Parents: blk.NewTipSetKey(c2), StateRoot: s2, Height: h1}
	b9 := &blk.Block{Parents: blk.NewTipSetKey(c2), StateRoot: s2, Height: h2}
	assert.True(t, b1.Equals(b1))
	assert.True(t, b1.Equals(b2))
	assert.False(t, b1.Equals(b3))
	assert.False(t, b1.Equals(b4))
	assert.False(t, b1.Equals(b5))
	assert.False(t, b1.Equals(b6))
	assert.False(t, b1.Equals(b7))
	assert.False(t, b1.Equals(b8))
	assert.False(t, b1.Equals(b9))
	assert.True(t, b3.Equals(b3))
	assert.False(t, b3.Equals(b4))
	assert.False(t, b3.Equals(b6))
	assert.False(t, b3.Equals(b9))
	assert.False(t, b4.Equals(b5))
	assert.False(t, b5.Equals(b6))
	assert.False(t, b6.Equals(b7))
	assert.False(t, b7.Equals(b8))
	assert.False(t, b8.Equals(b9))
	assert.True(t, b9.Equals(b9))
}

func TestBlockJsonMarshal(t *testing.T) {
	tf.UnitTest(t)

	var parent, child blk.Block
	child.Miner = address.NewForTestGetter()()
	child.Height = 1
	child.Parents = blk.NewTipSetKey(parent.Cid())
	child.StateRoot = parent.Cid()

	child.Messages = types.TxMeta{SecpRoot: types.CidFromString(t, "somecid"), BLSRoot: types.EmptyMessagesCID}
	child.MessageReceipts = types.CidFromString(t, "somecid")

	marshalled, e1 := json.Marshal(&child)
	assert.NoError(t, e1)
	str := string(marshalled)

	assert.Contains(t, str, child.Miner.String())
	assert.Contains(t, str, parent.Cid().String())
	assert.Contains(t, str, child.Messages.SecpRoot.String())
	assert.Contains(t, str, child.MessageReceipts.String())

	// marshal/unmarshal symmetry
	var unmarshalled blk.Block
	e2 := json.Unmarshal(marshalled, &unmarshalled)
	assert.NoError(t, e2)

	assert.Equal(t, child, unmarshalled)
	types.AssertHaveSameCid(t, &child, &unmarshalled)
	assert.True(t, child.Equals(&unmarshalled))
}

func TestSignatureData(t *testing.T) {
	tf.UnitTest(t)
	newAddress := address.NewForTestGetter()

	b := &blk.Block{
		Miner:           newAddress(),
		Ticket:          blk.Ticket{VRFProof: []byte{0x01, 0x02, 0x03}},
		Height:          types.Uint64(2),
		Messages:        types.TxMeta{SecpRoot: types.CidFromString(t, "somecid"), BLSRoot: types.EmptyMessagesCID},
		MessageReceipts: types.CidFromString(t, "somecid"),
		Parents:         blk.NewTipSetKey(types.CidFromString(t, "somecid")),
		ParentWeight:    types.Uint64(1000),
		ElectionProof:   []byte{0x1},
		StateRoot:       types.CidFromString(t, "somecid"),
		Timestamp:       types.Uint64(1),
		BlockSig:        []byte{0x3},
	}

	diff := &blk.Block{
		Miner:           newAddress(),
		Ticket:          blk.Ticket{VRFProof: []byte{0x03, 0x01, 0x02}},
		Height:          types.Uint64(3),
		Messages:        types.TxMeta{SecpRoot: types.CidFromString(t, "someothercid"), BLSRoot: types.EmptyMessagesCID},
		MessageReceipts: types.CidFromString(t, "someothercid"),
		Parents:         blk.NewTipSetKey(types.CidFromString(t, "someothercid")),
		ParentWeight:    types.Uint64(1001),
		ElectionProof:   []byte{0x2},
		StateRoot:       types.CidFromString(t, "someothercid"),
		Timestamp:       types.Uint64(4),
		BlockSig:        []byte{0x4},
	}

	// Changing BlockSig does not affect output
	func() {
		before := b.SignatureData()

		cpy := b.BlockSig
		defer func() { b.BlockSig = cpy }()

		b.BlockSig = diff.BlockSig
		after := b.SignatureData()
		assert.True(t, bytes.Equal(before, after))
	}()

	// Changing all other fields does affect output
	// Note: using reflectors doesn't seem to make this much less tedious
	// because it appears that there is no generic field setting function.
	func() {
		before := b.SignatureData()

		cpy := b.Miner
		defer func() { b.Miner = cpy }()

		b.Miner = diff.Miner
		after := b.SignatureData()
		assert.False(t, bytes.Equal(before, after))
	}()

	func() {
		before := b.SignatureData()

		cpy := b.Ticket
		defer func() { b.Ticket = cpy }()

		b.Ticket = diff.Ticket
		after := b.SignatureData()
		assert.False(t, bytes.Equal(before, after))
	}()

	func() {
		before := b.SignatureData()

		cpy := b.Height
		defer func() { b.Height = cpy }()

		b.Height = diff.Height
		after := b.SignatureData()
		assert.False(t, bytes.Equal(before, after))
	}()

	func() {
		before := b.SignatureData()

		cpy := b.Messages
		defer func() { b.Messages = cpy }()

		b.Messages = diff.Messages
		after := b.SignatureData()
		assert.False(t, bytes.Equal(before, after))
	}()

	func() {
		before := b.SignatureData()

		cpy := b.MessageReceipts
		defer func() { b.MessageReceipts = cpy }()

		b.MessageReceipts = diff.MessageReceipts
		after := b.SignatureData()
		assert.False(t, bytes.Equal(before, after))
	}()

	func() {
		before := b.SignatureData()

		cpy := b.Parents
		defer func() { b.Parents = cpy }()

		b.Parents = diff.Parents
		after := b.SignatureData()
		assert.False(t, bytes.Equal(before, after))
	}()

	func() {
		before := b.SignatureData()

		cpy := b.ParentWeight
		defer func() { b.ParentWeight = cpy }()

		b.ParentWeight = diff.ParentWeight
		after := b.SignatureData()
		assert.False(t, bytes.Equal(before, after))
	}()

	func() {
		before := b.SignatureData()

		cpy := b.ElectionProof
		defer func() { b.ElectionProof = cpy }()

		b.ElectionProof = diff.ElectionProof
		after := b.SignatureData()
		assert.False(t, bytes.Equal(before, after))
	}()

	func() {
		before := b.SignatureData()

		cpy := b.StateRoot
		defer func() { b.StateRoot = cpy }()

		b.StateRoot = diff.StateRoot
		after := b.SignatureData()
		assert.False(t, bytes.Equal(before, after))
	}()

	func() {
		before := b.SignatureData()

		cpy := b.Timestamp
		defer func() { b.Timestamp = cpy }()

		b.Timestamp = diff.Timestamp
		after := b.SignatureData()
		assert.False(t, bytes.Equal(before, after))
	}()

}

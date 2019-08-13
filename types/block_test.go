package types

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"
	"time"

	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
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

	testRoundTrip := func(t *testing.T, exp *Block) {
		jb, err := json.Marshal(exp)
		require.NoError(t, err)
		var jsonRoundTrip Block
		err = json.Unmarshal(jb, &jsonRoundTrip)
		require.NoError(t, err)

		ipldNodeOrig, err := cbor.DumpObject(exp)
		assert.NoError(t, err)
		// NOTICE: skips the intermediate json steps from above.
		var cborJSONRoundTrip Block
		err = cbor.DecodeInto(ipldNodeOrig, &cborJSONRoundTrip)
		assert.NoError(t, err)

		AssertHaveSameCid(t, &jsonRoundTrip, &cborJSONRoundTrip)
	}

	t.Run("encoding block with zero fields works", func(t *testing.T) {
		testRoundTrip(t, &Block{})
	})

	t.Run("encoding block with nonzero fields works", func(t *testing.T) {
		// We should ensure that every field is set -- zero values might
		// pass when non-zero values do not due to nil/null encoding.

		b := &Block{
			Miner:           newAddress(),
			Ticket:          []byte{0x01, 0x02, 0x03},
			Height:          Uint64(2),
			Nonce:           3,
			Messages:        CidFromString(t, "somecid"),
			MessageReceipts: CidFromString(t, "somecid"),
			Parents:         NewTipSetKey(CidFromString(t, "somecid")),
			ParentWeight:    Uint64(1000),
			Proof:           NewTestPoSt(),
			StateRoot:       CidFromString(t, "somecid"),
			Timestamp:       Uint64(1),
		}
		s := reflect.TypeOf(*b)
		// This check is here to request that you add a non-zero value for new fields
		// to the above (and update the field count below).
		require.Equal(t, 13, s.NumField()) // Note: this also counts private fields
		testRoundTrip(t, b)
	})
}

func TestBlockString(t *testing.T) {
	tf.UnitTest(t)

	var b Block

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

			var b Block
			b.Height = Uint64(n)

			assert.Equal(t, uint64(b.Height), b.Score(), "block height: %d - block score %d", b.Height, b.Score())
		}
	})
}

func TestDecodeBlock(t *testing.T) {
	tf.UnitTest(t)

	t.Run("successfully decodes raw bytes to a Filecoin block", func(t *testing.T) {
		addrGetter := address.NewForTestGetter()

		c1 := CidFromString(t, "a")
		c2 := CidFromString(t, "b")
		cM := CidFromString(t, "messages")
		cR := CidFromString(t, "receipts")

		before := &Block{
			Miner:           addrGetter(),
			Ticket:          []uint8{},
			Parents:         NewTipSetKey(c1),
			Height:          2,
			Messages:        cM,
			StateRoot:       c2,
			MessageReceipts: cR,
		}

		after, err := DecodeBlock(before.ToNode().RawData())
		assert.NoError(t, err)
		assert.Equal(t, after.Cid(), before.Cid())
		assert.Equal(t, before, after)
	})

	t.Run("decode failure results in an error", func(t *testing.T) {
		_, err := DecodeBlock([]byte{1, 2, 3})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "malformed stream")
	})
}

func TestEquals(t *testing.T) {
	tf.UnitTest(t)

	c1 := CidFromString(t, "a")
	c2 := CidFromString(t, "b")

	var n1 Uint64 = 1234
	var n2 Uint64 = 9876

	b1 := &Block{Parents: NewTipSetKey(c1), Nonce: n1}
	b2 := &Block{Parents: NewTipSetKey(c1), Nonce: n1}
	b3 := &Block{Parents: NewTipSetKey(c1), Nonce: n2}
	b4 := &Block{Parents: NewTipSetKey(c2), Nonce: n1}
	assert.True(t, b1.Equals(b1))
	assert.True(t, b1.Equals(b2))
	assert.False(t, b1.Equals(b3))
	assert.False(t, b1.Equals(b4))
	assert.False(t, b3.Equals(b4))
}

func TestParanoidPanic(t *testing.T) {
	tf.UnitTest(t)

	paranoid = true

	b1 := &Block{Nonce: 1}
	b1.Cid()

	b1.Nonce = 2
	assert.Panics(t, func() {
		b1.Cid()
	})
}

func TestBlockJsonMarshal(t *testing.T) {
	tf.UnitTest(t)

	var parent, child Block
	child.Miner = address.NewForTestGetter()()
	child.Height = 1
	child.Nonce = Uint64(2)
	child.Parents = NewTipSetKey(parent.Cid())
	child.StateRoot = parent.Cid()

	child.Messages = CidFromString(t, "somecid")
	child.MessageReceipts = CidFromString(t, "somecid")

	marshalled, e1 := json.Marshal(&child)
	assert.NoError(t, e1)
	str := string(marshalled)

	assert.Contains(t, str, child.Miner.String())
	assert.Contains(t, str, parent.Cid().String())
	assert.Contains(t, str, child.Messages.String())
	assert.Contains(t, str, child.MessageReceipts.String())

	// marshal/unmarshal symmetry
	var unmarshalled Block
	e2 := json.Unmarshal(marshalled, &unmarshalled)
	assert.NoError(t, e2)

	assert.Equal(t, child, unmarshalled)
	AssertHaveSameCid(t, &child, &unmarshalled)
	assert.True(t, child.Equals(&unmarshalled))
}

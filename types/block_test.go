package types

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"
	"time"

	cbor "gx/ipfs/QmRiRJhn427YVuufBEHofLreKWNw7P7BWNq86Sb9kzqdbd/go-ipld-cbor"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTriangleEncoding(t *testing.T) {
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

	newAddress := NewAddressForTestGetter()

	// REVIVE AFTER https://github.com/filecoin-project/go-filecoin/issues/599 is fixed.
	//
	// testRoundTripThatIThinkWeWant := func(t *testing.T, exp *Block) {
	// assert := assert.New(t)
	// require := require.New(t)
	//
	// // Simulate first half of the dag_daemon_test above.
	// jb, err := json.Marshal(exp)
	// require.NoError(err)
	// var jsonRoundTrip Block
	// err = json.Unmarshal(jb, &jsonRoundTrip)
	// require.NoError(err)

	// // Simulate the second half.
	// cborRaw, err := cbor.DumpObject(exp)
	// assert.NoError(err)
	// ipldNodeOrig, err := cbor.Decode(cborRaw, DefaultHashFunction, -1)
	// assert.NoError(err)
	// jin, err := json.Marshal(ipldNodeOrig)
	// require.NoError(err)
	// ipldNodeFromJSON, err := cbor.FromJSON(bytes.NewReader(jin), DefaultHashFunction, -1)
	// require.NoError(err)
	// var cborJSONRoundTrip Block
	// err = cbor.DecodeInto(ipldNodeFromJSON.RawData(), &cborJSONRoundTrip)
	// assert.NoError(err)
	//
	// AssertHaveSameCid(assert, &jsonRoundTrip, &cborJSONRoundTrip)
	// }

	testRoundTrip := func(t *testing.T, exp *Block) {
		assert := assert.New(t)
		require := require.New(t)

		jb, err := json.Marshal(exp)
		require.NoError(err)
		var jsonRoundTrip Block
		err = json.Unmarshal(jb, &jsonRoundTrip)
		require.NoError(err)

		ipldNodeOrig, err := cbor.DumpObject(exp)
		assert.NoError(err)
		// NOTICE: skips the intermediate json steps from above.
		var cborJSONRoundTrip Block
		err = cbor.DecodeInto(ipldNodeOrig, &cborJSONRoundTrip)
		assert.NoError(err)

		AssertHaveSameCid(assert, &jsonRoundTrip, &cborJSONRoundTrip)
	}

	t.Run("encoding block with zero fields works", func(t *testing.T) {
		testRoundTrip(t, &Block{})
	})

	t.Run("encoding block with nonzero fields works", func(t *testing.T) {
		// We should ensure that every field is set -- zero values might
		// pass when non-zero values do not due to nil/null encoding.
		b := &Block{
			Miner:             newAddress(),
			Ticket:            Bytes([]byte{0x01, 0x02, 0x03}),
			Parents:           NewSortedCidSet(SomeCid()),
			ParentWeightNum:   Uint64(1),
			ParentWeightDenom: Uint64(1),
			Height:            Uint64(2),
			Nonce:             3,
			Messages:          []*Message{{To: newAddress()}},
			StateRoot:         SomeCid(),
			MessageReceipts:   []*MessageReceipt{{ExitCode: 1}},
		}
		s := reflect.TypeOf(*b)
		// This check is here to request that you add a non-zero value for new fields
		// to the above (and update the field count below).
		require.Equal(t, 10, s.NumField())
		testRoundTrip(t, b)
	})
}

func TestBlockIsParentOf(t *testing.T) {
	var p, c Block
	assert.False(t, p.IsParentOf(c))
	assert.False(t, c.IsParentOf(p))

	c.Parents.Add(p.Cid())
	assert.True(t, p.IsParentOf(c))
	assert.False(t, c.IsParentOf(p))
}

func TestBlockScore(t *testing.T) {
	source := rand.NewSource(time.Now().UnixNano())

	t.Run("block score equals block height", func(t *testing.T) {
		assert := assert.New(t)

		for i := 0; i < 100; i++ {
			n := uint64(source.Int63())

			var b Block
			b.Height = Uint64(n)

			assert.Equal(uint64(b.Height), b.Score(), "block height: %d - block score %d", b.Height, b.Score())
		}
	})
}

func TestDecodeBlock(t *testing.T) {
	t.Run("successfully decodes raw bytes to a Filecoin block", func(t *testing.T) {
		assert := assert.New(t)

		addrGetter := NewAddressForTestGetter()
		m1 := NewMessage(addrGetter(), addrGetter(), 0, NewAttoFILFromFIL(10), "hello", []byte("cat"))
		m2 := NewMessage(addrGetter(), addrGetter(), 0, NewAttoFILFromFIL(2), "yes", []byte("dog"))

		c1, err := cidFromString("a")
		assert.NoError(err)
		c2, err := cidFromString("b")
		assert.NoError(err)

		before := &Block{
			Miner:     addrGetter(),
			Parents:   NewSortedCidSet(c1),
			Height:    2,
			Messages:  []*Message{m1, m2},
			StateRoot: c2,
			MessageReceipts: []*MessageReceipt{
				{ExitCode: 1, Return: []Bytes{[]byte{1, 2}}},
				{ExitCode: 1, Return: []Bytes{[]byte{1, 2, 3}}},
			},
		}

		after, err := DecodeBlock(before.ToNode().RawData())
		assert.NoError(err)
		assert.Equal(after.Cid(), before.Cid())
		assert.Equal(after, before)
	})

	t.Run("decode failure results in an error", func(t *testing.T) {
		assert := assert.New(t)

		_, err := DecodeBlock([]byte{1, 2, 3})
		assert.Error(err)
		assert.Contains(err.Error(), "malformed stream")
	})
}

func TestEquals(t *testing.T) {
	assert := assert.New(t)

	c1, err := cidFromString("a")
	assert.NoError(err)
	c2, err := cidFromString("b")
	assert.NoError(err)

	var n1 Uint64 = 1234
	var n2 Uint64 = 9876

	b1 := &Block{Parents: NewSortedCidSet(c1), Nonce: n1}
	b2 := &Block{Parents: NewSortedCidSet(c1), Nonce: n1}
	b3 := &Block{Parents: NewSortedCidSet(c1), Nonce: n2}
	b4 := &Block{Parents: NewSortedCidSet(c2), Nonce: n1}
	assert.True(b1.Equals(b1))
	assert.True(b1.Equals(b2))
	assert.False(b1.Equals(b3))
	assert.False(b1.Equals(b4))
	assert.False(b3.Equals(b4))
}

func TestBlockJsonMarshal(t *testing.T) {
	assert := assert.New(t)

	var parent, child Block
	child.Miner = NewAddressForTestGetter()()
	child.Height = 1
	child.Nonce = Uint64(2)
	child.Parents = NewSortedCidSet(parent.Cid())
	child.StateRoot = parent.Cid()

	mkMsg := NewMessageForTestGetter()

	message := mkMsg()

	receipt := &MessageReceipt{ExitCode: 0}
	child.Messages = []*Message{message}
	child.MessageReceipts = []*MessageReceipt{receipt}

	marshalled, e1 := json.Marshal(child)
	assert.NoError(e1)
	str := string(marshalled)

	assert.Contains(str, parent.Cid().String())
	assert.Contains(str, message.From.String())
	assert.Contains(str, message.To.String())

	// marshal/unmarshal symmetry
	var unmarshalled Block
	e2 := json.Unmarshal(marshalled, &unmarshalled)
	assert.NoError(e2)

	assert.True(child.Equals(&unmarshalled))
}

package types

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/address"
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

	newAddress := address.NewForTestGetter()
	newSignedMessage := NewSignedMessageForTestGetter(mockSigner)

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
			Miner:           newAddress(),
			Ticket:          []byte{0x01, 0x02, 0x03},
			Height:          Uint64(2),
			Nonce:           3,
			Messages:        []*SignedMessage{newSignedMessage()},
			MessageReceipts: []*MessageReceipt{{ExitCode: 1}},
			Parents:         NewSortedCidSet(SomeCid()),
			ParentWeight:    Uint64(1000),
			Proof:           NewTestPoSt(),
			StateRoot:       SomeCid(),
		}
		s := reflect.TypeOf(*b)
		// This check is here to request that you add a non-zero value for new fields
		// to the above (and update the field count below).
		require.Equal(t, 12, s.NumField()) // Note: this also counts private fields
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

func TestBlockString(t *testing.T) {
	assert := assert.New(t)
	var b Block

	cid := b.Cid()

	got := b.String()
	assert.Contains(got, cid.String())
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

func cidFromString(input string) (cid.Cid, error) {
	prefix := cid.V1Builder{Codec: cid.DagCBOR, MhType: DefaultHashFunction}
	return prefix.Sum([]byte(input))
}

func TestDecodeBlock(t *testing.T) {
	newSignedMessage := NewSignedMessageForTestGetter(mockSigner)
	t.Run("successfully decodes raw bytes to a Filecoin block", func(t *testing.T) {
		assert := assert.New(t)

		addrGetter := address.NewForTestGetter()

		c1, err := cidFromString("a")
		assert.NoError(err)
		c2, err := cidFromString("b")
		assert.NoError(err)

		before := &Block{
			Miner:     addrGetter(),
			Ticket:    []uint8{},
			Parents:   NewSortedCidSet(c1),
			Height:    2,
			Messages:  []*SignedMessage{newSignedMessage(), newSignedMessage()},
			StateRoot: c2,
			MessageReceipts: []*MessageReceipt{
				{ExitCode: 1, Return: [][]byte{{1, 2}}},
				{ExitCode: 1, Return: [][]byte{{1, 2, 3}}},
			},
		}

		after, err := DecodeBlock(before.ToNode().RawData())
		assert.NoError(err)
		assert.Equal(after.Cid(), before.Cid())
		assert.Equal(before, after)
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

func TestParanoidPanic(t *testing.T) {
	assert := assert.New(t)
	paranoid = true

	b1 := &Block{Nonce: 1}
	b1.Cid()

	b1.Nonce = 2
	assert.Panics(func() {
		b1.Cid()
	})
}

func TestBlockJsonMarshal(t *testing.T) {
	assert := assert.New(t)
	newSignedMessage := NewSignedMessageForTestGetter(mockSigner)

	var parent, child Block
	child.Miner = address.NewForTestGetter()()
	child.Height = 1
	child.Nonce = Uint64(2)
	child.Parents = NewSortedCidSet(parent.Cid())
	child.StateRoot = parent.Cid()

	message := newSignedMessage()

	retVal := []byte{1, 2, 3}
	receipt := &MessageReceipt{
		ExitCode: 123,
		Return:   [][]byte{retVal},
	}
	child.Messages = []*SignedMessage{message}
	child.MessageReceipts = []*MessageReceipt{receipt}

	marshalled, e1 := json.Marshal(&child)
	assert.NoError(e1)
	str := string(marshalled)

	assert.Contains(str, child.Miner.String())
	assert.Contains(str, parent.Cid().String())
	assert.Contains(str, message.From.String())
	assert.Contains(str, message.To.String())

	// marshal/unmarshal symmetry
	var unmarshalled Block
	e2 := json.Unmarshal(marshalled, &unmarshalled)
	assert.NoError(e2)

	AssertHaveSameCid(assert, &child, &unmarshalled)
	assert.True(child.Equals(&unmarshalled))

	assert.Equal(uint8(123), unmarshalled.MessageReceipts[0].ExitCode)
	assert.Equal([][]byte{{1, 2, 3}}, unmarshalled.MessageReceipts[0].Return)
}

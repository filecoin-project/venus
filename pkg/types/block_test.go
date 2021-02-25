package types

import (
	"bytes"
	"encoding/json"
	proof2 "github.com/filecoin-project/specs-actors/v2/actors/runtime/proof"
	"reflect"
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/crypto"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestTriangleEncoding(t *testing.T) {
	tf.UnitTest(t)

	// We want to be sure that:
	//      BlockHeader => json => BlockHeader
	// yields exactly the same thing as:
	//      BlockHeader => IPLD node => CBOR => IPLD node => json => IPLD node => BlockHeader (!)
	// because we want the output encoding of a BlockHeader directly from memory
	// (first case) to be exactly the same as the output encoding of a BlockHeader from
	// storage (second case). WTF you might say, and you would not be wrong. The
	// use case is machine-parsing command output. For example dag_daemon_test
	// dumps the newBlock from memory as json (first case). It then dag gets
	// the newBlock by cid which yeilds a json-encoded ipld node (first half of
	// the second case). It json decodes this ipld node and then decodes the node
	// into a newBlock (second half of the second case). I don't claim this is ideal,
	// see: https://github.com/filecoin-project/venus/issues/599

	newAddress := NewForTestGetter()

	testRoundTrip := func(t *testing.T, exp *BlockHeader) {
		jb, err := json.Marshal(exp)
		require.NoError(t, err)
		var jsonRoundTrip BlockHeader
		err = json.Unmarshal(jb, &jsonRoundTrip)
		require.NoError(t, err)
		AssertHaveSameCid(t, exp, &jsonRoundTrip)

		buf := new(bytes.Buffer)
		err = jsonRoundTrip.MarshalCBOR(buf)
		assert.NoError(t, err)
		var cborJSONRoundTrip BlockHeader
		err = cborJSONRoundTrip.UnmarshalCBOR(buf)
		assert.NoError(t, err)
		AssertHaveSameCid(t, exp, &cborJSONRoundTrip)
	}

	// // TODO: to make this test pass.
	// // This will fail with output: "varints malformed, could not reach the end"
	// //     which is from go-varint package, need to check.
	// t.Run("encoding newBlock with zero fields works", func(t *testing.T) {
	// 	testRoundTrip(t, &blk.BlockHeader{})
	// })

	t.Run("encoding newBlock with nonzero fields works", func(t *testing.T) {
		// We should ensure that every field is set -- zero values might
		// pass when non-zero values do not due to nil/null encoding.
		// posts := []blk.PoStProof{blk.NewPoStProof(constants.DevRegisteredWinningPoStProof, []byte{0x07})}
		b := &BlockHeader{
			Miner:         newAddress(),
			Ticket:        Ticket{VRFProof: []byte{0x01, 0x02, 0x03}},
			ElectionProof: &ElectionProof{VRFProof: []byte{0x0a, 0x0b}},
			Height:        2,
			BeaconEntries: []*BeaconEntry{
				{
					Round: 1,
					Data:  []byte{0x3},
				},
			},
			Messages:              CidFromString(t, "somecid"),
			ParentMessageReceipts: CidFromString(t, "somecid"),
			Parents:               NewTipSetKey(CidFromString(t, "somecid")),
			ParentWeight:          fbig.NewInt(1000),
			ParentStateRoot:       CidFromString(t, "somecid"),
			Timestamp:             1,
			BlockSig: &crypto.Signature{
				Type: crypto.SigTypeBLS,
				Data: []byte{0x3},
			},
			BLSAggregate: &crypto.Signature{
				Type: crypto.SigTypeBLS,
				Data: []byte{0x3},
			},
			// PoStProofs:    posts,
			ForkSignaling: 6,
		}
		s := reflect.TypeOf(*b)
		// This check is here to request that you add a non-zero value for new fields
		// to the above (and update the field count below).
		// Also please add non zero fields to "b" and "diff" in TestSignatureData
		// and add a new check that different values of the new field result in
		// different output data.
		require.Equal(t, 19, s.NumField()) // Note: this also counts private fields
		testRoundTrip(t, b)
	})
}

func TestBlockString(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := NewForTestGetter()
	c1 := CidFromString(t, "a")
	c2 := CidFromString(t, "b")
	cM := CidFromString(t, "messages")
	cR := CidFromString(t, "receipts")

	b := &BlockHeader{
		Miner:                 addrGetter(),
		Ticket:                Ticket{VRFProof: []uint8{}},
		Parents:               NewTipSetKey(c1),
		Height:                2,
		ParentWeight:          fbig.Zero(),
		Messages:              cM,
		ParentStateRoot:       c2,
		ParentMessageReceipts: cR,
		BlockSig:              &crypto.Signature{Type: crypto.SigTypeSecp256k1, Data: []byte{}},
		BLSAggregate:          &crypto.Signature{Type: crypto.SigTypeBLS, Data: []byte{}},
		ParentBaseFee:         abi.NewTokenAmount(1),
	}

	cid := b.Cid()

	got := b.String()
	assert.Contains(t, got, cid.String())
}

func TestDecodeBlock(t *testing.T) {
	tf.UnitTest(t)

	t.Run("successfully decodes raw bytes to a Filecoin newBlock", func(t *testing.T) {
		addrGetter := NewForTestGetter()

		c1 := CidFromString(t, "a")
		c2 := CidFromString(t, "b")
		cM := CidFromString(t, "messages")
		cR := CidFromString(t, "receipts")

		before := &BlockHeader{
			Miner:                 addrGetter(),
			Ticket:                Ticket{VRFProof: nil},
			Parents:               NewTipSetKey(c1),
			Height:                2,
			ParentWeight:          fbig.Zero(),
			Messages:              cM,
			ParentStateRoot:       c2,
			ParentMessageReceipts: cR,
			BlockSig:              &crypto.Signature{Type: crypto.SigTypeSecp256k1, Data: []byte{}},
			BLSAggregate:          &crypto.Signature{Type: crypto.SigTypeBLS, Data: []byte{}},
			ParentBaseFee:         abi.NewTokenAmount(1),
		}

		after, err := DecodeBlock(before.ToNode().RawData())
		require.NoError(t, err)
		assert.Equal(t, after.Cid(), before.Cid())
		assert.Equal(t, before, after)
	})

	t.Run("decode failure results in an error", func(t *testing.T) {
		_, err := DecodeBlock([]byte{1, 2, 3})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cbor input should be of type array")
	})
}

func TestEquals(t *testing.T) {
	tf.UnitTest(t)
	addrGetter := NewForTestGetter()
	minerAddr := addrGetter()
	mockCid := CidFromString(t, "mock")
	c1 := CidFromString(t, "a")
	c2 := CidFromString(t, "b")

	s1 := CidFromString(t, "state1")
	s2 := CidFromString(t, "state2")

	var h1 abi.ChainEpoch = 1
	var h2 abi.ChainEpoch = 2

	b1 := &BlockHeader{Miner: minerAddr, Messages: mockCid, ParentMessageReceipts: mockCid, Parents: NewTipSetKey(c1), ParentStateRoot: s1, Height: h1}
	b2 := &BlockHeader{Miner: minerAddr, Messages: mockCid, ParentMessageReceipts: mockCid, Parents: NewTipSetKey(c1), ParentStateRoot: s1, Height: h1}
	b3 := &BlockHeader{Miner: minerAddr, Messages: mockCid, ParentMessageReceipts: mockCid, Parents: NewTipSetKey(c1), ParentStateRoot: s2, Height: h1}
	b4 := &BlockHeader{Miner: minerAddr, Messages: mockCid, ParentMessageReceipts: mockCid, Parents: NewTipSetKey(c2), ParentStateRoot: s1, Height: h1}
	b5 := &BlockHeader{Miner: minerAddr, Messages: mockCid, ParentMessageReceipts: mockCid, Parents: NewTipSetKey(c1), ParentStateRoot: s1, Height: h2}
	b6 := &BlockHeader{Miner: minerAddr, Messages: mockCid, ParentMessageReceipts: mockCid, Parents: NewTipSetKey(c2), ParentStateRoot: s1, Height: h2}
	b7 := &BlockHeader{Miner: minerAddr, Messages: mockCid, ParentMessageReceipts: mockCid, Parents: NewTipSetKey(c1), ParentStateRoot: s2, Height: h2}
	b8 := &BlockHeader{Miner: minerAddr, Messages: mockCid, ParentMessageReceipts: mockCid, Parents: NewTipSetKey(c2), ParentStateRoot: s2, Height: h1}
	b9 := &BlockHeader{Miner: minerAddr, Messages: mockCid, ParentMessageReceipts: mockCid, Parents: NewTipSetKey(c2), ParentStateRoot: s2, Height: h2}
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

	proot := CidFromString(t, "mock")
	var child BlockHeader
	child.Miner = NewForTestGetter()()
	child.Height = 1
	child.ParentWeight = fbig.Zero()
	child.Parents = NewTipSetKey(proot)
	child.ParentStateRoot = proot

	child.Messages = CidFromString(t, "somecid")
	child.ParentMessageReceipts = CidFromString(t, "somecid")

	child.ParentBaseFee = abi.NewTokenAmount(1)

	marshalled, e1 := json.Marshal(&child)
	assert.NoError(t, e1)
	str := string(marshalled)

	assert.Contains(t, str, child.Miner.String())
	assert.Contains(t, str, proot.String())
	assert.Contains(t, str, child.Messages.String())
	assert.Contains(t, str, child.ParentMessageReceipts.String())

	// marshal/unmarshal symmetry
	var unmarshalled BlockHeader
	e2 := json.Unmarshal(marshalled, &unmarshalled)
	assert.NoError(t, e2)

	assert.Equal(t, child, unmarshalled)
	AssertHaveSameCid(t, &child, &unmarshalled)
	assert.True(t, child.Equals(&unmarshalled))
}

func TestSignatureData(t *testing.T) {
	tf.UnitTest(t)
	newAddress := NewForTestGetter()
	posts := []proof2.PoStProof{{PoStProof: abi.RegisteredPoStProof_StackedDrgWinning32GiBV1, ProofBytes: []byte{0x07}}}

	b := &BlockHeader{
		Miner:         newAddress(),
		Ticket:        Ticket{VRFProof: []byte{0x01, 0x02, 0x03}},
		ElectionProof: &ElectionProof{VRFProof: []byte{0x0a, 0x0b}},
		BeaconEntries: []*BeaconEntry{
			{
				Round: 5,
				Data:  []byte{0x0c},
			},
		},
		Height:                2,
		Messages:              CidFromString(t, "somecid"),
		ParentMessageReceipts: CidFromString(t, "somecid"),
		Parents:               NewTipSetKey(CidFromString(t, "somecid")),
		ParentWeight:          fbig.NewInt(1000),
		ForkSignaling:         3,
		ParentStateRoot:       CidFromString(t, "somecid"),
		Timestamp:             1,
		ParentBaseFee:         abi.NewTokenAmount(10),
		WinPoStProof:          posts,
		BlockSig: &crypto.Signature{
			Type: crypto.SigTypeBLS,
			Data: []byte{0x3},
		},
	}

	diffposts := []proof2.PoStProof{{PoStProof: abi.RegisteredPoStProof_StackedDrgWinning32GiBV1, ProofBytes: []byte{0x07, 0x08}}}

	diff := &BlockHeader{
		Miner:         newAddress(),
		Ticket:        Ticket{VRFProof: []byte{0x03, 0x01, 0x02}},
		ElectionProof: &ElectionProof{VRFProof: []byte{0x0c, 0x0d}},
		BeaconEntries: []*BeaconEntry{
			{
				Round: 44,
				Data:  []byte{0xc0},
			},
		},
		Height:                3,
		Messages:              CidFromString(t, "someothercid"),
		ParentMessageReceipts: CidFromString(t, "someothercid"),
		Parents:               NewTipSetKey(CidFromString(t, "someothercid")),
		ParentWeight:          fbig.NewInt(1001),
		ForkSignaling:         2,
		ParentStateRoot:       CidFromString(t, "someothercid"),
		Timestamp:             4,
		ParentBaseFee:         abi.NewTokenAmount(20),
		WinPoStProof:          diffposts,
		BlockSig: &crypto.Signature{
			Type: crypto.SigTypeBLS,
			Data: []byte{0x4},
		},
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

		cpy := b.ElectionProof
		defer func() { b.ElectionProof = cpy }()

		b.ElectionProof = diff.ElectionProof
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

		cpy := b.ParentMessageReceipts
		defer func() { b.ParentMessageReceipts = cpy }()

		b.ParentMessageReceipts = diff.ParentMessageReceipts
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

		cpy := b.ForkSignaling
		defer func() { b.ForkSignaling = cpy }()

		b.ForkSignaling = diff.ForkSignaling
		after := b.SignatureData()
		assert.False(t, bytes.Equal(before, after))
	}()

	func() {
		before := b.SignatureData()

		cpy := b.ParentStateRoot
		defer func() { b.ParentStateRoot = cpy }()

		b.ParentStateRoot = diff.ParentStateRoot
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

	func() {
		before := b.SignatureData()

		cpy := b.WinPoStProof
		defer func() { b.WinPoStProof = cpy }()

		b.WinPoStProof = diff.WinPoStProof
		after := b.SignatureData()
		assert.False(t, bytes.Equal(before, after))
	}()

	func() {
		before := b.SignatureData()
		cpy := b.BeaconEntries
		defer func() {
			b.BeaconEntries = cpy
		}()

		b.BeaconEntries = diff.BeaconEntries
		after := b.SignatureData()

		assert.False(t, bytes.Equal(before, after))
	}()

}

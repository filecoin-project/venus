package block_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blk "github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/crypto"
	"github.com/filecoin-project/venus/internal/pkg/enccid"
	"github.com/filecoin-project/venus/internal/pkg/encoding"
	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/internal/pkg/types"
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
	// see: https://github.com/filecoin-project/venus/issues/599

	newAddress := types.NewForTestGetter()

	testRoundTrip := func(t *testing.T, exp *blk.Block) {
		jb, err := json.Marshal(exp)
		require.NoError(t, err)
		var jsonRoundTrip blk.Block
		err = json.Unmarshal(jb, &jsonRoundTrip)
		require.NoError(t, err)
		types.AssertHaveSameCid(t, exp, &jsonRoundTrip)

		ipldNodeOrig, err := encoding.Encode(jsonRoundTrip)
		assert.NoError(t, err)
		var cborJSONRoundTrip blk.Block
		err = encoding.Decode(ipldNodeOrig, &cborJSONRoundTrip)
		assert.NoError(t, err)
		types.AssertHaveSameCid(t, exp, &cborJSONRoundTrip)
	}

	// // TODO: to make this test pass.
	// // This will fail with output: "varints malformed, could not reach the end"
	// //     which is from go-varint package, need to check.
	// t.Run("encoding block with zero fields works", func(t *testing.T) {
	// 	testRoundTrip(t, &blk.Block{})
	// })

	t.Run("encoding block with nonzero fields works", func(t *testing.T) {
		// We should ensure that every field is set -- zero values might
		// pass when non-zero values do not due to nil/null encoding.
		// posts := []blk.PoStProof{blk.NewPoStProof(constants.DevRegisteredWinningPoStProof, []byte{0x07})}
		b := &blk.Block{
			Miner:         newAddress(),
			Ticket:        blk.Ticket{VRFProof: []byte{0x01, 0x02, 0x03}},
			ElectionProof: &crypto.ElectionProof{VRFProof: []byte{0x0a, 0x0b}},
			Height:        2,
			BeaconEntries: []*blk.BeaconEntry{
				{
					Round: 1,
					Data:  []byte{0x3},
				},
			},
			Messages:        enccid.NewCid(types.CidFromString(t, "somecid")),
			MessageReceipts: enccid.NewCid(types.CidFromString(t, "somecid")),
			Parents:         blk.NewTipSetKey(types.CidFromString(t, "somecid")),
			ParentWeight:    fbig.NewInt(1000),
			StateRoot:       enccid.NewCid(types.CidFromString(t, "somecid")),
			Timestamp:       1,
			BlockSig: &crypto.Signature{
				Type: crypto.SigTypeBLS,
				Data: []byte{0x3},
			},
			BLSAggregateSig: &crypto.Signature{
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

	var b blk.Block
	cid := b.Cid()

	got := b.String()
	assert.Contains(t, got, cid.String())
}

func TestDecodeBlock(t *testing.T) {
	tf.UnitTest(t)

	t.Run("successfully decodes raw bytes to a Filecoin block", func(t *testing.T) {
		addrGetter := types.NewForTestGetter()

		c1 := types.CidFromString(t, "a")
		c2 := types.CidFromString(t, "b")
		cM := types.CidFromString(t, "messages")
		cR := types.CidFromString(t, "receipts")

		before := &blk.Block{
			Miner:           addrGetter(),
			Ticket:          blk.Ticket{VRFProof: []uint8{}},
			Parents:         blk.NewTipSetKey(c1),
			Height:          2,
			ParentWeight:    fbig.Zero(),
			Messages:        enccid.NewCid(cM),
			StateRoot:       enccid.NewCid(c2),
			MessageReceipts: enccid.NewCid(cR),
			BlockSig:        &crypto.Signature{Type: crypto.SigTypeSecp256k1, Data: []byte{}},
			BLSAggregateSig: &crypto.Signature{Type: crypto.SigTypeBLS, Data: []byte{}},
			ParentBaseFee:   abi.NewTokenAmount(1),
		}

		after, err := blk.DecodeBlock(before.ToNode().RawData())
		require.NoError(t, err)
		assert.Equal(t, after.Cid(), before.Cid())
		assert.Equal(t, before, after)
	})

	t.Run("decode failure results in an error", func(t *testing.T) {
		_, err := blk.DecodeBlock([]byte{1, 2, 3})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cbor: cannot unmarshal")
	})
}

func TestEquals(t *testing.T) {
	tf.UnitTest(t)

	c1 := types.CidFromString(t, "a")
	c2 := types.CidFromString(t, "b")

	s1 := types.CidFromString(t, "state1")
	s2 := types.CidFromString(t, "state2")

	var h1 abi.ChainEpoch = 1
	var h2 abi.ChainEpoch = 2

	b1 := &blk.Block{Parents: blk.NewTipSetKey(c1), StateRoot: enccid.NewCid(s1), Height: h1}
	b2 := &blk.Block{Parents: blk.NewTipSetKey(c1), StateRoot: enccid.NewCid(s1), Height: h1}
	b3 := &blk.Block{Parents: blk.NewTipSetKey(c1), StateRoot: enccid.NewCid(s2), Height: h1}
	b4 := &blk.Block{Parents: blk.NewTipSetKey(c2), StateRoot: enccid.NewCid(s1), Height: h1}
	b5 := &blk.Block{Parents: blk.NewTipSetKey(c1), StateRoot: enccid.NewCid(s1), Height: h2}
	b6 := &blk.Block{Parents: blk.NewTipSetKey(c2), StateRoot: enccid.NewCid(s1), Height: h2}
	b7 := &blk.Block{Parents: blk.NewTipSetKey(c1), StateRoot: enccid.NewCid(s2), Height: h2}
	b8 := &blk.Block{Parents: blk.NewTipSetKey(c2), StateRoot: enccid.NewCid(s2), Height: h1}
	b9 := &blk.Block{Parents: blk.NewTipSetKey(c2), StateRoot: enccid.NewCid(s2), Height: h2}
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
	child.Miner = types.NewForTestGetter()()
	child.Height = 1
	child.ParentWeight = fbig.Zero()
	child.Parents = blk.NewTipSetKey(parent.Cid())
	child.StateRoot = enccid.NewCid(parent.Cid())

	child.Messages = enccid.NewCid(types.CidFromString(t, "somecid"))
	child.MessageReceipts = enccid.NewCid(types.CidFromString(t, "somecid"))

	child.ParentBaseFee = abi.NewTokenAmount(1)

	marshalled, e1 := json.Marshal(&child)
	assert.NoError(t, e1)
	str := string(marshalled)

	assert.Contains(t, str, child.Miner.String())
	assert.Contains(t, str, parent.Cid().String())
	assert.Contains(t, str, child.Messages.String())
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
	newAddress := types.NewForTestGetter()
	posts := []builtin.PoStProof{{PoStProof: abi.RegisteredPoStProof_StackedDrgWinning32GiBV1, ProofBytes: []byte{0x07}}}

	b := &blk.Block{
		Miner:         newAddress(),
		Ticket:        blk.Ticket{VRFProof: []byte{0x01, 0x02, 0x03}},
		ElectionProof: &crypto.ElectionProof{VRFProof: []byte{0x0a, 0x0b}},
		BeaconEntries: []*blk.BeaconEntry{
			{
				Round: 5,
				Data:  []byte{0x0c},
			},
		},
		Height:          2,
		Messages:        enccid.NewCid(types.CidFromString(t, "somecid")),
		MessageReceipts: enccid.NewCid(types.CidFromString(t, "somecid")),
		Parents:         blk.NewTipSetKey(types.CidFromString(t, "somecid")),
		ParentWeight:    fbig.NewInt(1000),
		ForkSignaling:   3,
		StateRoot:       enccid.NewCid(types.CidFromString(t, "somecid")),
		Timestamp:       1,
		ParentBaseFee:   abi.NewTokenAmount(10),
		WinPoStProof:    blk.FromAbiProofArr(posts),
		BlockSig: &crypto.Signature{
			Type: crypto.SigTypeBLS,
			Data: []byte{0x3},
		},
	}

	diffposts := []builtin.PoStProof{{PoStProof: abi.RegisteredPoStProof_StackedDrgWinning32GiBV1, ProofBytes: []byte{0x07, 0x08}}}

	diff := &blk.Block{
		Miner:         newAddress(),
		Ticket:        blk.Ticket{VRFProof: []byte{0x03, 0x01, 0x02}},
		ElectionProof: &crypto.ElectionProof{VRFProof: []byte{0x0c, 0x0d}},
		BeaconEntries: []*blk.BeaconEntry{
			{
				Round: 44,
				Data:  []byte{0xc0},
			},
		},
		Height:          3,
		Messages:        enccid.NewCid(types.CidFromString(t, "someothercid")),
		MessageReceipts: enccid.NewCid(types.CidFromString(t, "someothercid")),
		Parents:         blk.NewTipSetKey(types.CidFromString(t, "someothercid")),
		ParentWeight:    fbig.NewInt(1001),
		ForkSignaling:   2,
		StateRoot:       enccid.NewCid(types.CidFromString(t, "someothercid")),
		Timestamp:       4,
		ParentBaseFee:   abi.NewTokenAmount(20),
		WinPoStProof:    blk.FromAbiProofArr(diffposts),
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

		cpy := b.ForkSignaling
		defer func() { b.ForkSignaling = cpy }()

		b.ForkSignaling = diff.ForkSignaling
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

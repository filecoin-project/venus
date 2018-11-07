package consensus_test

import (
	"context"
	"encoding/hex"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"

	"gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmXTpwq2AkzQsPjKqFQDNY2bMdsAT53hUBETeyj8QRHTZU/sha256-simd"
	"gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"
	"testing"
)

func TestIsWinningTicket(t *testing.T) {
	assert := assert.New(t)

	cases := []struct {
		ticket     byte
		myPower    uint64
		totalPower uint64
		wins       bool
	}{
		{0x00, 1, 5, true},
		{0x30, 1, 5, true},
		{0x40, 1, 5, false},
		{0xF0, 1, 5, false},
		{0x00, 5, 5, true},
		{0x33, 5, 5, true},
		{0x44, 5, 5, true},
		{0xFF, 5, 5, true},
		{0x00, 0, 5, false},
		{0x33, 0, 5, false},
		{0x44, 0, 5, false},
		{0xFF, 0, 5, false},
	}

	minerAddress := address.NewForTestGetter()()
	ctx := context.Background()
	d := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(d)
	var st state.Tree

	for _, c := range cases {
		ptv := consensus.NewTestPowerTableView(c.myPower, c.totalPower)
		ticket := [sha256.Size]byte{}
		ticket[0] = c.ticket
		r, err := consensus.IsWinningTicket(ctx, bs, ptv, st, ticket[:], minerAddress)
		assert.NoError(err)
		assert.Equal(c.wins, r, "%+v", c)
	}
}

func TestIsWinningTicketErrors(t *testing.T) {
	assert := assert.New(t)

	testCase := struct {
		ticket     byte
		myPower    int64
		totalPower int64
		wins       bool
	}{0x00, 1, 5, true}

	minerAddress := address.NewForTestGetter()()
	ctx := context.Background()
	d := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(d)
	var st state.Tree

	ptv := NewFailingTestPowerTableView(testCase.myPower, testCase.totalPower)
	ticket := [sha256.Size]byte{}
	ticket[0] = testCase.ticket
	r, err := consensus.IsWinningTicket(ctx, bs, ptv, st, ticket[:], minerAddress)
	assert.False(r)
	assert.Equal(err.Error(), "Couldn't get totalPower: something went wrong")
}

func TestCreateChallenge(t *testing.T) {
	assert := assert.New(t)

	cases := []struct {
		parentTickets  [][]byte
		nullBlockCount uint64
		challenge      string
	}{
		// From https://www.di-mgt.com.au/sha_testvectors.html
		{[][]byte{[]byte("ac"), []byte("ab"), []byte("xx")}, uint64('c'),
			"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		{[][]byte{[]byte("z"), []byte("x"), []byte("abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnop")},
			uint64('q'), "248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1"},
		{[][]byte{[]byte("abcdefghbcdefghicdefghijdefghijkefghijklfghijklmghijklmnhijklmnoijklmnopjklmnopqklmnopqrlmnopqrsmnopqrstnopqrst"), []byte("z"), []byte("x")},
			uint64('u'), "cf5b16a778af8380036ce59e7b0492370b249b11e8f07a51afac45037afee9d1"},
	}

	for _, c := range cases {
		decoded, err := hex.DecodeString(c.challenge)
		assert.NoError(err)

		parents := consensus.TipSet{}
		for _, t := range c.parentTickets {
			b := types.Block{Ticket: t}
			parents.AddBlock(&b)
		}
		r, err := consensus.CreateChallenge(parents, c.nullBlockCount)
		assert.NoError(err)
		assert.Equal(decoded, r)
	}
}

type FailingTestPowerTableView struct{ minerPower, totalPower uint64 }

func NewFailingTestPowerTableView(minerPower int64, totalPower int64) *FailingTestPowerTableView {
	return &FailingTestPowerTableView{uint64(minerPower), uint64(totalPower)}
}

func (tv *FailingTestPowerTableView) Total(ctx context.Context, st state.Tree, bstore blockstore.Blockstore) (uint64, error) {
	return tv.totalPower, errors.New("something went wrong")
}

func (tv *FailingTestPowerTableView) Miner(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) (uint64, error) {
	return uint64(tv.minerPower), nil
}

func (tv *FailingTestPowerTableView) HasPower(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) bool {
	return true
}

package consensus_test

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestIsElectionWinner(t *testing.T) {
	cases := []struct {
		ticket        []byte
		electionProof []byte
		myPower       uint64
		totalPower    uint64
		wins          bool
	}{
		// Good proof with proper sig and enough power
		{
			ticket:        requireDecodeHex(t, "0ba5272600ac4123c39ae01b4fc574aa430f156710a9a912f24dda2b5aa9e7741f2f3a0fdc129c519ca35f208b419ec9ada9903e1bb26d048a39950cf434902801"),
			electionProof: requireDecodeHex(t, "030196349ea47653a062c6b00e7cd8074eb8a23ede5b7f95de3ccd2dde62cf3174220b867d72366486a55cd1943dee28d6b78533df6dd2ef9b56158dd1317a2a01"),
			myPower:       1,
			totalPower:    42,
			wins:          true,
		},
		// Bad proof with proper sig but not enough power
		{
			ticket:        requireDecodeHex(t, "f13dda9840314b7ece70d8c4e3350ce5e42a09989ca3f7d70470d4085fd8cba54aa8d9dcce53595560f1c76018af210eeacebfb0c1dec9eca3f03f9185f4a27f01"),
			electionProof: requireDecodeHex(t, "d944206748d1d5a554031d43b629c7bef111f6c9c864692ce35e8bcf38e118cd5495bdea99e21cfeab3486252fef3d2e93e7183f8a837029e7af8a1fa0509b7501"),
			myPower:       1,
			totalPower:    42,
			wins:          false,
		},
		// Bad proof with enough power and improper sig
		{
			ticket:        requireDecodeHex(t, "0ba5272600ac4123c39ae01b4fc574aa430f156710a9a912f24dda2b5aa9e7741f2f3a0fdc129c519ca35f208b419ec9ada9903e1bb26d048a39950cf434902801"),
			electionProof: requireDecodeHex(t, "1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"),
			myPower:       1,
			totalPower:    3,
			wins:          false,
		},
		// Totally bogus proof with no power or good sig
		{
			ticket:        requireDecodeHex(t, "0ba5272600ac4123c39ae01b4fc574aa430f156710a9a912f24dda2b5aa9e7741f2f3a0fdc129c519ca35f208b419ec9ada9903e1bb26d048a39950cf434902801"),
			electionProof: requireDecodeHex(t, "FFFF000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"),
			myPower:       1,
			totalPower:    42,
			wins:          false,
		},
	}

	_, kis := types.NewMockSignersAndKeyInfo(2)
	minerAddress := requireAddress(t, &kis[1]) // Test cases were signed with second key info

	ctx := context.Background()
	d := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(d)
	var st state.Tree

	t.Run("IsElectionWinner performs as expected on cases", func(t *testing.T) {
		minerToWorker := map[address.Address]address.Address{minerAddress: minerAddress}
		for _, c := range cases {
			ptv := th.NewTestPowerTableView(types.NewBytesAmount(c.myPower), types.NewBytesAmount(c.totalPower), minerToWorker)
			r, err := consensus.ElectionMachine{}.IsElectionWinner(ctx, bs, ptv, st, types.Ticket{VDFResult: c.ticket[:]}, c.electionProof, minerAddress, minerAddress)
			assert.NoError(t, err)
			assert.Equal(t, c.wins, r, "%+v", c)
		}
	})

	t.Run("IsElectionWinner returns false + error when we fail to get total power", func(t *testing.T) {
		ptv1 := NewFailingTestPowerTableView(types.NewBytesAmount(cases[0].myPower), types.NewBytesAmount(cases[0].totalPower))
		r, err := consensus.ElectionMachine{}.IsElectionWinner(ctx, bs, ptv1, st, types.Ticket{VDFResult: cases[0].ticket[:]}, cases[0].electionProof, minerAddress, minerAddress)
		assert.False(t, r)
		assert.Equal(t, err.Error(), "Couldn't get totalPower: something went wrong with the total power")

	})

	t.Run("IsWinningTicket returns false + error when we fail to get miner power", func(t *testing.T) {
		ptv2 := NewFailingMinerTestPowerTableView(types.NewBytesAmount(cases[0].myPower), types.NewBytesAmount(cases[0].totalPower))
		r, err := consensus.ElectionMachine{}.IsElectionWinner(ctx, bs, ptv2, st, types.Ticket{VDFResult: cases[0].ticket[:]}, cases[0].electionProof, minerAddress, minerAddress)
		assert.False(t, r)
		assert.Equal(t, err.Error(), "Couldn't get minerPower: something went wrong with the miner power")
	})
}

func TestRunElection(t *testing.T) {
	signer, kis := types.NewMockSignersAndKeyInfo(1)
	electionAddr := requireAddress(t, &kis[0])

	electionProof, err := consensus.ElectionMachine{}.RunElection(consensus.MakeFakeTicketForTest(), electionAddr, signer)
	assert.NoError(t, err)
	assert.Equal(t, 65, len(electionProof))
}

func TestCompareElectionPower(t *testing.T) {
	tf.UnitTest(t)

	cases := []struct {
		electionProof byte
		myPower       uint64
		totalPower    uint64
		wins          bool
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
	for _, c := range cases {
		ep := make([]byte, 65)
		ep[0] = c.electionProof
		res := consensus.CompareElectionPower(ep, types.NewBytesAmount(c.myPower), types.NewBytesAmount(c.totalPower))
		assert.Equal(t, c.wins, res, "%+v", c)
	}
}

func TestGenValidTicketChain(t *testing.T) {
	// Start with an arbitrary ticket
	base := consensus.MakeFakeTicketForTest()

	// Interleave 3 signers
	signer, kis := types.NewMockSignersAndKeyInfo(3)
	addr1 := requireAddress(t, &kis[0])
	addr2 := requireAddress(t, &kis[1])
	addr3 := requireAddress(t, &kis[2])

	schedule := struct {
		Addrs     []address.Address
		NullCount []uint64
	}{
		Addrs:     []address.Address{addr1, addr1, addr1, addr2, addr3, addr3, addr1, addr2},
		NullCount: []uint64{0, 0, 1, 33, 2, 0, 0, 0},
	}

	// Grow the specified ticket chain without error
	for i := 0; i < len(schedule.Addrs); i++ {
		base = requireValidTicket(t, base, signer, schedule.Addrs[i], schedule.NullCount[i])
	}
}

func requireValidTicket(t *testing.T, parent types.Ticket, signer types.Signer, signerAddr address.Address, nullBlkCount uint64) types.Ticket {
	tm := consensus.TicketMachine{}

	ticket, err := tm.NextTicket(parent, signerAddr, signer, nullBlkCount)
	require.NoError(t, err)

	err = tm.NotarizeTime(&ticket)
	assert.NoError(t, err)

	valid, err := tm.ValidateTicket(parent, ticket, signerAddr, nullBlkCount)
	require.NoError(t, err)
	require.True(t, valid)
	return ticket
}

func TestNextTicketFailsWithInvalidSigner(t *testing.T) {
	parent := consensus.MakeFakeTicketForTest()
	signer, _ := types.NewMockSignersAndKeyInfo(1)
	badAddr := address.TestAddress
	tm := consensus.TicketMachine{}
	badTicket, err := tm.NextTicket(parent, badAddr, signer, 0)
	assert.Error(t, err)
	assert.Nil(t, badTicket.VRFProof)
}

func TestElectionFailsWithInvalidSigner(t *testing.T) {
	parent := types.Ticket{VDFResult: types.VDFY{0xbb}}
	signer, _ := types.NewMockSignersAndKeyInfo(1)
	badAddress := address.TestAddress
	ep, err := consensus.ElectionMachine{}.RunElection(parent, badAddress, signer)
	assert.Error(t, err)
	assert.Equal(t, types.VRFPi{}, ep)
}

func requireAddress(t *testing.T, ki *types.KeyInfo) address.Address {
	addr, err := ki.Address()
	require.NoError(t, err)
	return addr
}

func requireDecodeHex(t *testing.T, hexStr string) []byte {
	b, err := hex.DecodeString(hexStr)
	require.NoError(t, err)

	return b
}

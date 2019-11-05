package consensus_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

type electionCase struct {
	ticket        block.Ticket
	electionProof []byte
	nonce         uint64
	myPower       uint64
	totalPower    uint64
	wins          bool
}

func makeCases(t *testing.T, ki *types.KeyInfo, signer types.Signer) []electionCase {
	// makeCases creates specific election test cases
	//
	// 1. A correctly generated proof with enough power
	// 2. A correctly generated proof without enough power
	// 3. An incorrectly generated proof with enough power
	// 4. An incorrectly generated proof without enough power
	em := &consensus.ElectionMachine{}

	minerPower1 := uint64(1)
	totalPower1 := uint64(42)
	nonce1 := uint64(0)
	t1 := consensus.SeedFirstWinnerInNRounds(t, int(nonce1), ki, minerPower1, totalPower1)
	ep1, err := em.RunElection(t1, requireAddress(t, ki), signer, nonce1)
	require.NoError(t, err)
	case1 := electionCase{
		ticket:        t1,
		electionProof: ep1,
		nonce:         nonce1,
		myPower:       minerPower1,
		totalPower:    totalPower1,
		wins:          true,
	}

	minerPower2 := uint64(1)
	totalPower2 := uint64(42)
	nonce2 := uint64(0)
	t2 := consensus.SeedLoserInNRounds(t, int(nonce2), ki, minerPower2, totalPower2)
	ep2, err := em.RunElection(t1, requireAddress(t, ki), signer, nonce2)
	require.NoError(t, err)
	case2 := electionCase{
		ticket:        t2,
		electionProof: ep2,
		nonce:         nonce2,
		myPower:       minerPower2,
		totalPower:    totalPower2,
		wins:          false,
	}

	minerPower3 := uint64(1)
	totalPower3 := uint64(3)
	nonce3 := uint64(0)
	t3 := consensus.SeedFirstWinnerInNRounds(t, int(nonce3), ki, minerPower3, totalPower3)
	ep3, err := em.RunElection(t3, requireAddress(t, ki), signer, nonce3)
	require.NoError(t, err)
	ep3[len(ep3)-1] ^= 0xFF // flip bits
	case3 := electionCase{
		ticket:        t3,
		electionProof: ep3,
		nonce:         nonce3,
		myPower:       minerPower3,
		totalPower:    totalPower3,
		wins:          false,
	}

	minerPower4 := uint64(1)
	totalPower4 := uint64(3)
	nonce4 := uint64(0)
	t4 := consensus.SeedLoserInNRounds(t, int(nonce4), ki, minerPower4, totalPower4)
	ep4, err := em.RunElection(t4, requireAddress(t, ki), signer, nonce4)
	require.NoError(t, err)
	ep4[0] = 0xFF           // This proof only wins with > 1/2 total power in the system
	ep4[len(ep4)-1] ^= 0xFF // ensure ep4 has changed
	case4 := electionCase{
		ticket:        t4,
		electionProof: ep4,
		nonce:         nonce4,
		myPower:       minerPower4,
		totalPower:    totalPower4,
		wins:          false,
	}

	return []electionCase{case1, case2, case3, case4}
}

func TestIsElectionWinner(t *testing.T) {
	tf.UnitTest(t)

	signer, kis := types.NewMockSignersAndKeyInfo(1)
	minerAddress := requireAddress(t, &kis[0])
	cases := makeCases(t, &kis[0], signer)

	ctx := context.Background()

	t.Run("IsElectionWinner performs as expected on cases", func(t *testing.T) {
		minerToWorker := map[address.Address]address.Address{minerAddress: minerAddress}
		for _, c := range cases {
			ptv := consensus.NewFakePowerTableView(types.NewBytesAmount(c.myPower), types.NewBytesAmount(c.totalPower), minerToWorker)
			r, err := consensus.ElectionMachine{}.IsElectionWinner(ctx, ptv, c.ticket, c.nonce, c.electionProof, minerAddress, minerAddress)
			assert.NoError(t, err)
			assert.Equal(t, c.wins, r, "%+v", c)
		}
	})

	t.Run("IsElectionWinner returns false + error when we fail to get total power", func(t *testing.T) {
		ptv1 := consensus.NewPowerTableView(&consensus.FakePowerTableViewSnapshot{MinerPower: types.NewBytesAmount(cases[0].myPower)})
		r, err := consensus.ElectionMachine{}.IsElectionWinner(ctx, ptv1, cases[0].ticket, cases[0].nonce, cases[0].electionProof, minerAddress, minerAddress)
		assert.False(t, r)
		require.Error(t, err)
		assert.Equal(t, err.Error(), "Couldn't get totalPower: something went wrong with the total power")

	})

	t.Run("IsWinningTicket returns false + error when we fail to get miner power", func(t *testing.T) {
		ptv2 := consensus.NewPowerTableView(&consensus.FakePowerTableViewSnapshot{TotalPower: types.NewBytesAmount(cases[0].totalPower)})
		r, err := consensus.ElectionMachine{}.IsElectionWinner(ctx, ptv2, cases[0].ticket, cases[0].nonce, cases[0].electionProof, minerAddress, minerAddress)
		assert.False(t, r)
		require.Error(t, err)
		assert.Equal(t, err.Error(), "Couldn't get minerPower: something went wrong with the miner power")
	})
}

func TestRunElection(t *testing.T) {
	signer, kis := types.NewMockSignersAndKeyInfo(1)
	electionAddr := requireAddress(t, &kis[0])

	electionProof, err := consensus.ElectionMachine{}.RunElection(consensus.MakeFakeTicketForTest(), electionAddr, signer, 0)
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
		Addrs []address.Address
	}{
		Addrs: []address.Address{addr1, addr1, addr1, addr2, addr3, addr3, addr1, addr2},
	}

	// Grow the specified ticket chain without error
	for i := 0; i < len(schedule.Addrs); i++ {
		base = requireValidTicket(t, base, signer, schedule.Addrs[i])
	}
}

func requireValidTicket(t *testing.T, parent block.Ticket, signer types.Signer, signerAddr address.Address) block.Ticket {
	tm := consensus.TicketMachine{}

	ticket, err := tm.NextTicket(parent, signerAddr, signer)
	require.NoError(t, err)

	valid := tm.IsValidTicket(parent, ticket, signerAddr)
	require.True(t, valid)
	return ticket
}

func TestNextTicketFailsWithInvalidSigner(t *testing.T) {
	parent := consensus.MakeFakeTicketForTest()
	signer, _ := types.NewMockSignersAndKeyInfo(1)
	badAddr := address.TestAddress
	tm := consensus.TicketMachine{}
	badTicket, err := tm.NextTicket(parent, badAddr, signer)
	assert.Error(t, err)
	assert.Nil(t, badTicket.VRFProof)
}

func TestElectionFailsWithInvalidSigner(t *testing.T) {
	parent := block.Ticket{VRFProof: block.VRFPi{0xbb}}
	signer, _ := types.NewMockSignersAndKeyInfo(1)
	badAddress := address.TestAddress
	ep, err := consensus.ElectionMachine{}.RunElection(parent, badAddress, signer, 0)
	assert.Error(t, err)
	assert.Equal(t, block.VRFPi{}, ep)
}

func requireAddress(t *testing.T, ki *types.KeyInfo) address.Address {
	addr, err := ki.Address()
	require.NoError(t, err)
	return addr
}

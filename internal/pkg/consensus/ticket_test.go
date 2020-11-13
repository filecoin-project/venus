package consensus_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/crypto"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"

	"github.com/filecoin-project/venus/internal/pkg/consensus"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

func TestGenValidTicketChain(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	head := block.NewTipSetKey() // Tipset key is unused by fake randomness

	// Interleave 3 signers
	kis := types.MustGenerateBLSKeyInfo(3, 0)

	miner, err := address.NewIDAddress(uint64(1))
	require.NoError(t, err)
	signer := types.NewMockSigner(kis)
	addr1 := requireAddress(t, &kis[0])
	addr2 := requireAddress(t, &kis[1])
	addr3 := requireAddress(t, &kis[2])

	schedule := struct {
		Addrs []address.Address
	}{
		Addrs: []address.Address{addr1, addr1, addr1, addr2, addr3, addr3, addr1, addr2},
	}

	rnd := consensus.FakeSampler{Seed: 0}
	tm := consensus.NewTicketMachine(&rnd)

	// Grow the specified ticket chain without error
	for i := 0; i < len(schedule.Addrs); i++ {
		requireValidTicket(ctx, t, tm, head, abi.ChainEpoch(i), miner, schedule.Addrs[i], signer)
	}
}

func requireValidTicket(ctx context.Context, t *testing.T, tm *consensus.TicketMachine, head block.TipSetKey, epoch abi.ChainEpoch,
	miner, worker address.Address, signer types.Signer) {
	electionEntry := &block.BeaconEntry{}
	newPeriod := false
	ticket, err := tm.MakeTicket(ctx, head, epoch, miner, electionEntry, newPeriod, worker, signer)
	require.NoError(t, err)

	err = tm.IsValidTicket(ctx, head, electionEntry, newPeriod, epoch, miner, worker, ticket)
	require.NoError(t, err)
}

func TestNextTicketFailsWithInvalidSigner(t *testing.T) {
	ctx := context.Background()
	head := block.NewTipSetKey() // Tipset key is unused by fake randomness
	miner, err := address.NewIDAddress(uint64(1))
	require.NoError(t, err)

	signer, _ := types.NewMockSignersAndKeyInfo(1)
	badAddr := types.RequireIDAddress(t, 100)
	rnd := consensus.FakeSampler{Seed: 0}
	tm := consensus.NewTicketMachine(&rnd)
	electionEntry := &block.BeaconEntry{}
	newPeriod := false
	badTicket, err := tm.MakeTicket(ctx, head, abi.ChainEpoch(1), miner, electionEntry, newPeriod, badAddr, signer)
	assert.Error(t, err)
	assert.Nil(t, badTicket.VRFProof)
}

func requireAddress(t *testing.T, ki *crypto.KeyInfo) address.Address {
	addr, err := ki.Address()
	require.NoError(t, err)
	return addr
}

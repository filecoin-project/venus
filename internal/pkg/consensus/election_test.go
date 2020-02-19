package consensus_test

import (
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"

	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

func TestGenValidTicketChain(t *testing.T) {
	tf.UnitTest(t)
	// Start with an arbitrary ticket
	base := consensus.MakeFakeTicketForTest()

	// Interleave 3 signers
	kis := []crypto.KeyInfo{
		crypto.NewBLSKeyRandom(),
		crypto.NewBLSKeyRandom(),
		crypto.NewBLSKeyRandom(),
	}

	signer := types.NewMockSigner(kis)
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
	badAddr := vmaddr.TestAddress
	tm := consensus.TicketMachine{}
	badTicket, err := tm.NextTicket(parent, badAddr, signer)
	assert.Error(t, err)
	assert.Nil(t, badTicket.VRFProof)
}

func requireAddress(t *testing.T, ki *crypto.KeyInfo) address.Address {
	addr, err := ki.Address()
	require.NoError(t, err)
	return addr
}

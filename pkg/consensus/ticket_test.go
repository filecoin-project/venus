// stm: #unit
package consensus_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/venus/pkg/testhelpers"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/types"

	fbig "github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/crypto"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"

	"github.com/filecoin-project/venus/pkg/consensus"
)

func TestGenValidTicketChain(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	head, _ := types.NewTipSet([]*types.BlockHeader{mockBlock()}) // Tipset key is unused by fake randomness
	loader := newMockTipsetLoader(head)

	// Interleave 3 signers
	kis := testhelpers.MustGenerateBLSKeyInfo(3, 0)

	miner, err := address.NewIDAddress(uint64(1))
	require.NoError(t, err)
	signer := testhelpers.NewMockSigner(kis)
	addr1 := requireAddress(t, &kis[0])
	addr2 := requireAddress(t, &kis[1])
	addr3 := requireAddress(t, &kis[2])

	schedule := struct {
		Addrs []address.Address
	}{
		Addrs: []address.Address{addr1, addr1, addr1, addr2, addr3, addr3, addr1, addr2},
	}

	tm := consensus.NewTicketMachine(loader)

	// Grow the specified ticket Chain without error
	for i := 0; i < len(schedule.Addrs); i++ {
		// stm: @CONSENSUS_TICKET_MAKE_001, @CONSENSUS_TICKET_IS_VALID_001
		requireValidTicket(ctx, t, tm, head.Key(), abi.ChainEpoch(i), miner, schedule.Addrs[i], signer)
	}
}

func requireValidTicket(ctx context.Context, t *testing.T, tm *consensus.TicketMachine, head types.TipSetKey, epoch abi.ChainEpoch,
	miner, worker address.Address, signer types.Signer,
) {
	electionEntry := &types.BeaconEntry{}
	newPeriod := false
	ticket, err := tm.MakeTicket(ctx, head, epoch, miner, electionEntry, newPeriod, worker, signer)
	require.NoError(t, err)

	err = tm.IsValidTicket(ctx, head, electionEntry, newPeriod, epoch, miner, worker, ticket)
	require.NoError(t, err)
}

func TestNextTicketFailsWithInvalidSigner(t *testing.T) {
	ctx := context.Background()
	head, _ := types.NewTipSet([]*types.BlockHeader{mockBlock()}) // Tipset key is unused by fake randomness
	loader := newMockTipsetLoader(head)
	miner, err := address.NewIDAddress(uint64(1))
	require.NoError(t, err)

	signer, _ := testhelpers.NewMockSignersAndKeyInfo(1)
	badAddr := testhelpers.RequireIDAddress(t, 100)
	tm := consensus.NewTicketMachine(loader)
	electionEntry := &types.BeaconEntry{}
	newPeriod := false
	badTicket, err := tm.MakeTicket(ctx, head.Key(), abi.ChainEpoch(1), miner, electionEntry, newPeriod, badAddr, signer)
	assert.Error(t, err)
	assert.Nil(t, badTicket.VRFProof)
}

func requireAddress(t *testing.T, ki *crypto.KeyInfo) address.Address {
	addr, err := ki.Address()
	require.NoError(t, err)
	return addr
}

func mockBlock() *types.BlockHeader {
	mockCid, _ := constants.DefaultCidBuilder.Sum([]byte("mock"))
	return &types.BlockHeader{
		Miner:         testhelpers.NewForTestGetter()(),
		Ticket:        &types.Ticket{VRFProof: []byte{0x01, 0x02, 0x03}},
		ElectionProof: &types.ElectionProof{VRFProof: []byte{0x0a, 0x0b}},
		BeaconEntries: []types.BeaconEntry{
			{
				Round: 5,
				Data:  []byte{0x0c},
			},
		},
		Height:        2,
		ParentWeight:  fbig.NewInt(1000),
		ForkSignaling: 3,
		Timestamp:     1,
		ParentBaseFee: abi.NewTokenAmount(10),
		BlockSig: &crypto.Signature{
			Type: crypto.SigTypeBLS,
			Data: []byte{0x3},
		},
		ParentStateRoot:       mockCid,
		ParentMessageReceipts: mockCid,
		Messages:              mockCid,
	}
}

type mockTipsetLoader struct {
	tsk *types.TipSet
}

func newMockTipsetLoader(tsk *types.TipSet) *mockTipsetLoader {
	return &mockTipsetLoader{tsk: tsk}
}

func (m *mockTipsetLoader) GetTipSet(ctx context.Context, tsk types.TipSetKey) (*types.TipSet, error) {
	return m.tsk, nil
}

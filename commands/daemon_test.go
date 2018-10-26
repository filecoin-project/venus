package commands

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/node"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBasicDaemonHeartbeat asserts that the initial heartbeat of a daemon is correct,
// and that adding a new address to the daemon results in a new address being added
// to the heartbeat.
func TestBasicDaemonHeartbeat(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)

	// make a node and start it
	n := node.MakeNodesStarted(t, 1, false, false)[0]

	// generate a heartbeat
	hb, err := NewHeartbeat(n)
	assert.NoError(err)

	// should have a matching peerID
	assert.Equal(n.Host().ID().Pretty(), hb.PeerID)

	// miner address is empty string if one isn't set
	assert.Equal("", hb.MinerAddress)

	// we should only have a singe wallet address
	require.True(len(n.Wallet.Addresses()) == 1)
	addr := n.Wallet.Addresses()[0].String()
	// hb should only have one address
	assert.True(len(hb.WalletAddresses) == 1)
	assert.Equal(addr, hb.WalletAddresses[0])

	// matching heaviest tipssets
	hts := n.ChainReader.Head().ToSortedCidSet()
	assert.Equal(hts, hb.HeaviestTipset)

	// we are only connected to our selves
	require.True(len(n.Host().Peerstore().Peers()) == 1)
	// the heartbeat omits this
	assert.True(len(hb.Peers) == 0)

	// Node makes a new address
	_, err = n.NewAddress()
	require.NoError(err)

	// generate a heartbeat, which should have new address
	hb, err = NewHeartbeat(n)
	assert.NoError(err)

	// we should have a 2 wallet address
	require.True(len(n.Wallet.Addresses()) == 2)
	addr1 := n.Wallet.Addresses()[0].String()
	addr2 := n.Wallet.Addresses()[1].String()
	// hb should have both addresses
	assert.True(len(hb.WalletAddresses) == 2)
	assert.Contains(hb.WalletAddresses, addr1, addr2)
}

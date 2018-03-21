package lookup

import (
	"context"
	"testing"

	wallet "github.com/filecoin-project/go-filecoin/wallet"
	"github.com/stretchr/testify/assert"

	"gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p"
	"gx/ipfs/QmSFihvoND3eDaAYRCeLgLPt62yCPgMZs1NSZmKFEtJQQw/go-libp2p-floodsub"
	peerstore "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
)

func TestLookupAddress(t *testing.T) {
	assert := assert.New(t)

	//make 2 libp2p nodes
	local, err := libp2p.New(context.Background(),
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
	)
	assert.NoError(err)

	remote, err := libp2p.New(context.Background(),
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
	)
	assert.NoError(err)

	//configure local
	lfs, err := floodsub.NewFloodSub(context.Background(), local)
	assert.NoError(err)

	lw := wallet.New()
	lle, err := NewLookupEngine(lfs, lw, local.ID())
	assert.NoError(err)

	//configure remote
	rfs, err := floodsub.NewFloodSub(context.Background(), remote)
	assert.NoError(err)

	rw := wallet.New()
	rle, err := NewLookupEngine(rfs, rw, remote.ID())
	assert.NoError(err)

	//Connect the nodes
	rpi := peerstore.PeerInfo{
		ID:    remote.ID(),
		Addrs: remote.Addrs(),
	}
	err = local.Connect(context.Background(), rpi)
	assert.NoError(err)
	t.Logf("Local Node Conns: %v", local.Network().Conns())
	t.Logf("Remote Node Conns: %v", remote.Network().Conns())

	//begin the test
	//add an address on remote host
	ra := rw.NewAddress()

	//look up the remoteID on local host
	remoteID, err := lle.Lookup(context.Background(), ra)
	assert.NoError(err)
	assert.Equal(remote.ID(), remoteID)

	//add an address on local host
	la := lw.NewAddress()

	//look up the localID on remote host
	localID, err := rle.Lookup(context.Background(), la)
	assert.NoError(err)
	assert.Equal(local.ID(), localID)
}

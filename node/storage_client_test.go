package node

import (
	"context"
	"testing"
	"time"

	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"
	dag "gx/ipfs/QmeCaeBmCCEJrZahwXY4G2G8zRaNBWskrfKWoQ6Xv6c1DR/go-merkledag"

	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/lookup"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/assert"
)

type mockLookupService struct {
	peerIdsByMinerAddr map[types.Address]peer.ID
}

var _ lookup.PeerLookupService = &mockLookupService{}

// GetPeerIdByMinerAddress provides an interface to a map from miner address to libp2p identity.
func (mls *mockLookupService) GetPeerIDByMinerAddress(ctx context.Context, minerAddr types.Address) (peer.ID, error) {
	return mls.peerIdsByMinerAddr[minerAddr], nil
}

func TestDealProtocolClient(t *testing.T) {
	t.Parallel()
	ctx := context.TODO()
	assert := assert.New(t)
	nds := MakeNodesUnstarted(t, 2, false, true)
	connect(t, nds[0], nds[1])
	time.Sleep(time.Millisecond * 10) // wait for connect notifications to complete

	minerAddr, err := nds[0].NewAddress()
	assert.NoError(err)
	minerOwner, err := nds[0].NewAddress()
	assert.NoError(err)

	mls := &mockLookupService{
		peerIdsByMinerAddr: map[types.Address]peer.ID{
			minerAddr: nds[0].Host.ID(),
		},
	}

	nds[0].Lookup = mls
	nds[1].Lookup = mls

	clientAddr, err := nds[1].NewAddress()
	assert.NoError(err)

	sm := NewStorageBroker(nds[0])
	client := NewStorageClient(nds[1])

	msa := newMockMsp()
	client.smi = msa
	msa.minerOwners[minerAddr] = minerOwner
	msa.addAsk(minerAddr, 40, 5000)
	msa.addBid(clientAddr, 35, 5000)
	sm.smi = msa

	data := dag.NewRawNode([]byte("cats"))

	deal := &storagemarket.Deal{
		Ask:     0,
		Bid:     0,
		DataRef: data.Cid().String(),
	}

	propose, err := NewDealProposal(deal, nds[1].Wallet, clientAddr)
	assert.NoError(err)

	resp, err := client.ProposeDeal(ctx, propose)
	assert.NoError(err)
	assert.Equal(Accepted, resp.State)
	id := resp.ID

	time.Sleep(time.Millisecond * 50)

	resp, err = client.QueryDeal(ctx, id)
	assert.NoError(err)

	assert.Equal(Started, resp.State)

	err = nds[1].Blockservice.AddBlock(data)
	assert.NoError(err)

	equal := false
	for i := 0; i < 10; i++ {
		resp, err = client.QueryDeal(ctx, id)
		assert.NoError(err)

		equal = Posted == resp.State
		if equal {
			break
		}
		time.Sleep(time.Millisecond * 10)
	}
	assert.True(equal, "failed to sync")
}

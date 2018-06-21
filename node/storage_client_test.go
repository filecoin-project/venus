package node

import (
	"context"
	"testing"
	"time"

	dag "gx/ipfs/QmNUCLv5fmUBuAcwbkt58NQvMcJgd5FPCYV2yNCXq4Wnd6/go-ipfs/merkledag"

	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/stretchr/testify/assert"
)

func TestDealProtocolClient(t *testing.T) {
	t.Parallel()
	ctx := context.TODO()
	assert := assert.New(t)
	nds := MakeNodesUnstarted(t, 2, false)
	connect(t, nds[0], nds[1])
	time.Sleep(time.Millisecond * 10) // wait for connect notifications to complete

	sm := NewStorageMarket(nds[0])
	client := NewStorageClient(nds[1])

	minerAddr, err := nds[0].NewAddress()
	assert.NoError(err)
	minerOwner, err := nds[0].NewAddress()
	assert.NoError(err)

	msa := newMockMsp()
	client.smi = msa
	msa.minerOwners[minerAddr] = minerOwner
	msa.addAsk(minerAddr, 40, 5000)
	msa.addBid(address.TestAddress, 35, 5000)
	sm.smi = msa

	data := dag.NewRawNode([]byte("cats"))

	propose := &DealProposal{
		Deal: &storagemarket.Deal{
			Ask:     0,
			Bid:     0,
			DataRef: data.Cid(),
		},
		ClientSig: string(address.TestAddress[:]),
	}

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

	time.Sleep(time.Millisecond * 50)

	resp, err = client.QueryDeal(ctx, id)
	assert.NoError(err)

	assert.Equal(Posted, resp.State)
}

package node

import (
	"context"
	"testing"
	"time"

	dag "github.com/ipfs/go-ipfs/merkledag"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/stretchr/testify/assert"
)

func TestDealProtocolClient(t *testing.T) {
	ctx := context.TODO()
	assert := assert.New(t)
	nds := makeNodes(t, 2)
	connect(t, nds[0], nds[1])

	sm := NewStorageMarket(nds[0])
	client := NewStorageClient(nds[1])

	minerAddr := nds[0].Wallet.NewAddress()

	msa := &mockStorageMarketPeeker{}
	msa.addAsk(minerAddr, 40, 5000)
	msa.addBid(core.TestAccount, 35, 5000)
	sm.smi = msa
	sm.minerAddr = minerAddr

	data := dag.NewRawNode([]byte("cats"))

	propose := &DealProposal{
		Deal: &core.Deal{
			Ask:     0,
			Bid:     0,
			DataRef: data.Cid(),
		},
		ClientSig: string(core.TestAccount[:]),
	}

	// TODO: need a way of mapping address -> peerID
	minerPid := nds[0].Host.ID()

	resp, err := client.ProposeDeal(ctx, minerPid, propose)
	assert.NoError(err)
	assert.Equal(Accepted, resp.State)
	id := resp.ID

	time.Sleep(time.Millisecond * 50)

	resp, err = client.QueryDeal(ctx, minerPid, id)
	assert.NoError(err)

	assert.Equal(Started, resp.State)

	err = nds[1].Blockservice.AddBlock(data)
	assert.NoError(err)

	time.Sleep(time.Millisecond * 50)

	resp, err = client.QueryDeal(ctx, minerPid, id)
	assert.NoError(err)

	assert.Equal(Posted, resp.State)
}

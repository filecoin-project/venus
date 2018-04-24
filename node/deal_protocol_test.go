package node

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	dag "github.com/ipfs/go-ipfs/merkledag"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
)

type mockStorageMarketPeeker struct {
	asks []*core.Ask
	bids []*core.Bid
	//deals []*core.Deal

	minerOwners map[types.Address]types.Address
}

func newMockMsp() *mockStorageMarketPeeker {
	return &mockStorageMarketPeeker{
		minerOwners: make(map[types.Address]types.Address),
	}
}

func (msa *mockStorageMarketPeeker) GetAsk(ask uint64) (*core.Ask, error) {
	if uint64(len(msa.asks)) <= ask {
		return nil, fmt.Errorf("no such ask")
	}
	return msa.asks[ask], nil
}

func (msa *mockStorageMarketPeeker) GetBid(bid uint64) (*core.Bid, error) {
	if uint64(len(msa.bids)) <= bid {
		return nil, fmt.Errorf("no such bid")
	}
	return msa.bids[bid], nil
}

func (msa *mockStorageMarketPeeker) GetAskSet() (core.AskSet, error) {
	return nil, nil
}

func (msa *mockStorageMarketPeeker) GetBidSet() (core.BidSet, error) {
	return nil, nil
}

func (msa *mockStorageMarketPeeker) GetDealList() ([]*core.Deal, error) {
	return nil, nil
}

func (msa *mockStorageMarketPeeker) GetMinerOwner(ctx context.Context, a types.Address) (types.Address, error) {
	mo, ok := msa.minerOwners[a]
	if !ok {
		return types.Address{}, fmt.Errorf("no such miner")
	}

	return mo, nil
}

// makes mocking existing asks easier
func (msa *mockStorageMarketPeeker) addAsk(owner types.Address, price, size uint64) uint64 {
	id := uint64(len(msa.asks))
	msa.asks = append(msa.asks, &core.Ask{
		ID:    id,
		Owner: owner,
		Price: types.NewTokenAmount(price),
		Size:  types.NewBytesAmount(size),
	})
	return id
}

// makes mocking existing bids easier
func (msa *mockStorageMarketPeeker) addBid(owner types.Address, price, size uint64) uint64 {
	id := uint64(len(msa.bids))
	msa.bids = append(msa.bids, &core.Bid{
		ID:    id,
		Owner: owner,
		Price: types.NewTokenAmount(price),
		Size:  types.NewBytesAmount(size),
	})
	return id
}

func (msa *mockStorageMarketPeeker) AddDeal(ctx context.Context, miner types.Address, ask, bid uint64, sig string, data *cid.Cid) (*cid.Cid, error) {
	// TODO: something useful
	msg := types.NewMessage(types.Address{}, types.Address{}, nil, "", nil)
	return msg.Cid()
}

/* TODO: add tests for:
- test query for deal not found
- test deal fails once posted on chain (maybe)
*/
func TestDealProtocol(t *testing.T) {
	assert := assert.New(t)
	nd := MakeNodesUnstarted(t, 1, false)[0]

	sm := NewStorageMarket(nd)

	minerAddr, err := nd.NewAddress()
	assert.NoError(err)
	minerOwner, err := nd.NewAddress()
	assert.NoError(err)
	_ = minerOwner

	msa := newMockMsp()
	msa.minerOwners[minerAddr] = minerOwner
	msa.addAsk(minerAddr, 40, 5500)
	msa.addBid(core.TestAddress, 35, 5000)

	sm.smi = msa

	data := dag.NewRawNode([]byte("cats"))

	propose := &DealProposal{
		Deal: &core.Deal{
			Ask:     0,
			Bid:     0,
			DataRef: data.Cid(),
		},
		ClientSig: string(core.TestAddress[:]),
	}

	resp, err := sm.ProposeDeal(propose)
	assert.NoError(err)
	assert.Equal(Accepted, resp.State)
	id := resp.ID

	time.Sleep(time.Millisecond * 50)

	resp, err = sm.QueryDeal(id)
	assert.NoError(err)

	assert.Equal(Started, resp.State)

	err = nd.Blockservice.AddBlock(data)
	assert.NoError(err)

	time.Sleep(time.Millisecond * 50)

	resp, err = sm.QueryDeal(id)
	assert.NoError(err)

	assert.Equal(Posted, resp.State)
}

func TestDealProtocolMissing(t *testing.T) {
	assert := assert.New(t)
	nd := MakeNodesUnstarted(t, 1, false)[0]

	sm := NewStorageMarket(nd)

	minerAddr, err := nd.NewAddress()
	assert.NoError(err)
	minerOwner, err := nd.NewAddress()
	assert.NoError(err)

	msa := newMockMsp()
	msa.minerOwners[minerAddr] = minerOwner
	msa.addAsk(minerAddr, 40, 5500)
	msa.addAsk(minerAddr, 20, 1000)
	msa.addBid(core.TestAddress, 35, 5000)
	msa.addBid(core.TestAddress, 15, 2000)

	sm.smi = msa

	data := dag.NewRawNode([]byte("cats"))

	propose := &DealProposal{
		Deal:      &core.Deal{Ask: 0, Bid: 3, DataRef: data.Cid()},
		ClientSig: string(core.TestAddress[:]),
	}

	resp, err := sm.ProposeDeal(propose)
	assert.NoError(err)
	assert.Equal(Rejected, resp.State)
	assert.Equal("unknown bid: no such bid", resp.Message)

	propose = &DealProposal{
		Deal:      &core.Deal{Ask: 3, Bid: 0, DataRef: data.Cid()},
		ClientSig: string(core.TestAddress[:]),
	}

	resp, err = sm.ProposeDeal(propose)
	assert.NoError(err)
	assert.Equal(Rejected, resp.State)
	assert.Equal("unknown ask: no such ask", resp.Message)

	propose = &DealProposal{
		Deal:      &core.Deal{Ask: 1, Bid: 1, DataRef: data.Cid()},
		ClientSig: string(core.TestAddress[:]),
	}

	resp, err = sm.ProposeDeal(propose)
	assert.NoError(err)
	assert.Equal(Rejected, resp.State)
	assert.Equal("ask does not have enough space for bid", resp.Message)
}

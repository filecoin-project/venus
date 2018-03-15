package node

import (
	"context"
	"fmt"
	"testing"
	"time"

	dag "github.com/ipfs/go-ipfs/merkledag"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
)

type mockStorageMarketPeeker struct {
	asks []*core.Ask
	bids []*core.Bid
	//deals []*core.Deal
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

func (msa *mockStorageMarketPeeker) AddDeal(ctx context.Context, miner types.Address, ask, bid uint64, sig string) (*cid.Cid, error) {
	// TODO: something useful
	msg := types.NewMessage(types.Address{}, types.Address{}, nil, "", nil)
	return msg.Cid()
}

/* TODO: add tests for:
- no such ask
- no such bid
- bid too large / ask too small
- test query for deal not found
- test deal fails once posted on chain (maybe)
*/
func TestDealProtocol(t *testing.T) {
	assert := assert.New(t)
	nd := makeNodes(t, 1)[0]

	sm := NewStorageMarket(nd)

	minerAddr := nd.Wallet.NewAddress()

	msa := &mockStorageMarketPeeker{}
	msa.addAsk(minerAddr, 40, 5500)
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

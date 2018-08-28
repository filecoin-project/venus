package node

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer/test"
	dag "gx/ipfs/QmeCaeBmCCEJrZahwXY4G2G8zRaNBWskrfKWoQ6Xv6c1DR/go-merkledag"

	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStorageMarketPeeker struct {
	asks []*storagemarket.Ask
	bids []*storagemarket.Bid
	//deals []*core.Deal

	minerOwners map[types.Address]types.Address
}

func newMockMsp() *mockStorageMarketPeeker {
	return &mockStorageMarketPeeker{
		minerOwners: make(map[types.Address]types.Address),
	}
}

func (msa *mockStorageMarketPeeker) GetStorageAsk(ctx context.Context, ask uint64) (*storagemarket.Ask, error) {
	if uint64(len(msa.asks)) <= ask {
		return nil, fmt.Errorf("no such ask")
	}
	return msa.asks[ask], nil
}

func (msa *mockStorageMarketPeeker) GetBid(ctx context.Context, bid uint64) (*storagemarket.Bid, error) {
	if uint64(len(msa.bids)) <= bid {
		return nil, fmt.Errorf("no such bid")
	}
	return msa.bids[bid], nil
}

func (msa *mockStorageMarketPeeker) GetStorageAskSet(ctx context.Context) (storagemarket.AskSet, error) {
	return nil, nil
}

func (msa *mockStorageMarketPeeker) GetBidSet(ctx context.Context) (storagemarket.BidSet, error) {
	return nil, nil
}

func (msa *mockStorageMarketPeeker) GetDealList() ([]*storagemarket.Deal, error) {
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
	msa.asks = append(msa.asks, &storagemarket.Ask{
		ID:    id,
		Owner: owner,
		Price: types.NewAttoFILFromFIL(price),
		Size:  types.NewBytesAmount(size),
	})
	return id
}

// makes mocking existing bids easier
func (msa *mockStorageMarketPeeker) addBid(owner types.Address, price, size uint64) uint64 {
	id := uint64(len(msa.bids))
	msa.bids = append(msa.bids, &storagemarket.Bid{
		ID:    id,
		Owner: owner,
		Price: types.NewAttoFILFromFIL(price),
		Size:  types.NewBytesAmount(size),
	})
	return id
}

func (msa *mockStorageMarketPeeker) AddDeal(ctx context.Context, miner types.Address, ask, bid uint64, sig types.Signature, data *cid.Cid) (*cid.Cid, error) {
	// TODO: something useful
	msg := types.NewMessage(types.Address{}, types.Address{}, 0, nil, "", nil)
	return msg.Cid()
}

/* TODO: add tests for:
- test query for deal not found
- test deal fails once posted on chain (maybe)
*/
// TODO: does this test even work? I'm not sure the nodes its making get connected
func TestDealProtocol(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	nodes := MakeNodesUnstarted(t, 2, false, true)
	miner := nodes[0]
	client := nodes[1]
	ctx := context.Background()

	sm := NewStorageBroker(miner)

	minerAddr, err := miner.NewAddress()
	assert.NoError(err)
	minerOwner, err := miner.NewAddress()
	assert.NoError(err)
	_ = minerOwner

	clientAddr, err := client.NewAddress()
	assert.NoError(err)

	msa := newMockMsp()
	msa.minerOwners[minerAddr] = minerOwner
	msa.addAsk(minerAddr, 40, 5500)
	msa.addBid(clientAddr, 35, 5000)

	sm.smi = msa

	data := dag.NewRawNode([]byte("cats"))

	deal := &storagemarket.Deal{
		Ask:     0,
		Bid:     0,
		DataRef: data.Cid().String(),
	}

	propose, err := NewDealProposal(deal, client.Wallet, clientAddr)
	assert.NoError(err)
	resp, err := sm.ProposeDeal(ctx, propose)
	assert.NoError(err)
	assert.Equal(Accepted, resp.State)
	id := resp.ID

	time.Sleep(time.Millisecond * 50)

	resp, err = sm.QueryDeal(ctx, id)
	assert.NoError(err)

	assert.Equal(Started, resp.State)

	err = miner.Blockservice.AddBlock(data)
	assert.NoError(err)

	time.Sleep(time.Millisecond * 50)

	resp, err = sm.QueryDeal(ctx, id)
	assert.NoError(err)

	assert.Equal(Posted, resp.State)
}

func TestDealProtocolMissing(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	nodes := MakeNodesUnstarted(t, 2, false, true)
	miner := nodes[0]
	client := nodes[1]
	ctx := context.Background()

	sm := NewStorageBroker(miner)

	minerAddr, err := miner.NewAddress()
	assert.NoError(err)
	minerOwner, err := miner.NewAddress()
	assert.NoError(err)

	clientAddr, err := client.NewAddress()
	assert.NoError(err)

	msa := newMockMsp()
	msa.minerOwners[minerAddr] = minerOwner
	msa.addAsk(minerAddr, 40, 5500)
	msa.addAsk(minerAddr, 20, 1000)
	msa.addBid(clientAddr, 35, 5000)
	msa.addBid(clientAddr, 15, 2000)

	sm.smi = msa

	data := dag.NewRawNode([]byte("cats"))

	deal := &storagemarket.Deal{Ask: 0, Bid: 3, DataRef: data.Cid().String()}
	sig, err := storagemarket.SignDeal(deal, client.Wallet, clientAddr)
	assert.NoError(err)
	propose := &DealProposal{
		Deal:      deal,
		ClientSig: sig,
	}

	resp, err := sm.ProposeDeal(ctx, propose)
	assert.NoError(err)
	assert.Equal(Rejected, resp.State)
	assert.Equal("unknown bid: no such bid", resp.Message)

	deal = &storagemarket.Deal{Ask: 3, Bid: 0, DataRef: data.Cid().String()}
	sig, err = storagemarket.SignDeal(deal, client.Wallet, clientAddr)
	assert.NoError(err)
	propose = &DealProposal{
		Deal:      deal,
		ClientSig: sig,
	}

	resp, err = sm.ProposeDeal(ctx, propose)
	assert.NoError(err)
	assert.Equal(Rejected, resp.State)
	assert.Equal("unknown ask: no such ask", resp.Message)

	deal = &storagemarket.Deal{Ask: 1, Bid: 1, DataRef: data.Cid().String()}
	sig, err = storagemarket.SignDeal(deal, client.Wallet, clientAddr)
	assert.NoError(err)
	propose = &DealProposal{
		Deal:      deal,
		ClientSig: sig,
	}

	resp, err = sm.ProposeDeal(ctx, propose)
	assert.NoError(err)
	assert.Equal(Rejected, resp.State)
	assert.Equal("ask does not have enough space for bid", resp.Message)
}

func TestStateTreeMarketPeekerAddsDeal(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	ctx := context.Background()
	nd := MakeNodesUnstarted(t, 1, true, true)[0]
	nodeAddr, err := nd.NewAddress()
	assert.NoError(err)

	tif := th.MakeGenesisFunc(
		th.ActorAccount(nodeAddr, types.NewAttoFILFromFIL(10000)),
	)
	nd.ChainMgr.Genesis(ctx, tif)
	assert.NoError(err)
	assert.NoError(nd.Start(ctx))

	msa := &stateTreeMarketPeeker{nd}

	data := dag.NewRawNode([]byte("cats"))
	dealCid, err := msa.AddDeal(ctx, nodeAddr, uint64(0), 0, address.TestAddress[:], data.Cid())

	assert.NoError(err)
	assert.NotNil(dealCid)
}

func TestStateTreeMarketPeeker(t *testing.T) {
	require := require.New(t)

	nd := MakeNodesUnstarted(t, 1, true, true)[0]

	ctx := context.Background()
	cm := nd.ChainMgr
	cm.PwrTableView = &core.TestView{}

	// setup miner power in genesis block
	ki := types.MustGenerateKeyInfo(1, types.GenerateKeyInfoSeed())
	sn := types.NewMockSigner(ki)
	testAddress := sn.Addresses[0]

	testGen := th.MakeGenesisFunc(
		th.ActorAccount(testAddress, types.NewAttoFILFromFIL(10000)),
	)
	require.NoError(cm.Genesis(ctx, testGen))

	genesisBlock, err := cm.FetchBlock(ctx, cm.GetGenesisCid())
	require.NoError(err)

	// create miner
	nonce := uint64(0)
	pid, err := testutil.RandPeerID()
	require.NoError(err)
	msg, err := th.CreateMinerMessage(sn.Addresses[0], nonce, *types.NewBytesAmount(10000000), pid, types.NewZeroAttoFIL())
	require.NoError(err)
	b := core.RequireMineOnce(ctx, t, cm, genesisBlock, sn.Addresses[0], core.MustSign(sn, msg)[0])
	nonce++

	minerAddr, err := types.NewAddressFromBytes(b.MessageReceipts[0].Return[0])
	require.NoError(err)

	// add bid
	bidPrice := types.NewAttoFIL(big.NewInt(23))
	bidSize := types.NewBytesAmount(10000)
	msg, err = th.AddBidMessage(sn.Addresses[0], nonce, bidPrice, bidSize)
	require.NoError(err)
	b = core.RequireMineOnce(ctx, t, cm, b, sn.Addresses[0], core.MustSign(sn, msg)[0])
	nonce++

	bidID := big.NewInt(0).SetBytes(b.MessageReceipts[0].Return[0]).Uint64()

	// add ask
	askSize := types.NewBytesAmount(1032300)
	askPrice := types.NewAttoFIL(big.NewInt(96))
	msg, err = th.AddAskMessage(minerAddr, sn.Addresses[0], nonce, askPrice, askSize)
	require.NoError(err)
	b = core.RequireMineOnce(ctx, t, cm, b, sn.Addresses[0], core.MustSign(sn, msg)[0])
	nonce++

	askID := big.NewInt(0).SetBytes(b.MessageReceipts[0].Return[0]).Uint64()

	mstmp := &stateTreeMarketPeeker{nd}

	t.Run("retrieves ask from storage market", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		ask, err := mstmp.GetStorageAsk(ctx, askID)
		require.NoError(err)

		assert.Equal(askID, ask.ID)
		assert.Equal(askPrice, ask.Price)
		assert.Equal(minerAddr, ask.Owner)
		assert.Equal(askSize, ask.Size)
	})

	t.Run("retrieves bid from storage market", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		bid, err := mstmp.GetBid(ctx, bidID)
		require.NoError(err)

		assert.Equal(bidID, bid.ID)
		assert.Equal(bidPrice, bid.Price)
		assert.Equal(sn.Addresses[0], bid.Owner)
		assert.Equal(bidSize, bid.Size)
	})

	t.Run("retrieves all asks from storage market", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		asks, err := mstmp.GetStorageAskSet(ctx)
		require.NoError(err)

		assert.Equal(1, len(asks))
		ask := asks[0]

		assert.Equal(askID, ask.ID)
		assert.Equal(askPrice, ask.Price)
		assert.Equal(minerAddr, ask.Owner)
		assert.Equal(askSize, ask.Size)
	})

	t.Run("retrieves all bids from storage market", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		bids, err := mstmp.GetBidSet(ctx)
		require.NoError(err)

		assert.Equal(1, len(bids))
		bid := bids[0]

		assert.Equal(bidID, bid.ID)
		assert.Equal(bidPrice, bid.Price)
		assert.Equal(sn.Addresses[0], bid.Owner)
		assert.Equal(bidSize, bid.Size)
	})

	t.Run("retrieves owner from miner", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		addr, err := mstmp.GetMinerOwner(ctx, minerAddr)
		require.NoError(err)

		assert.Equal(sn.Addresses[0], addr)
	})
}

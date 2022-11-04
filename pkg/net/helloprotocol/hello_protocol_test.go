// stm: #unit
package helloprotocol_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/filecoin-project/venus/pkg/net/helloprotocol"
	"github.com/filecoin-project/venus/pkg/net/peermgr"

	ds "github.com/ipfs/go-datastore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"

	"github.com/filecoin-project/go-address"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/net"
	"github.com/filecoin-project/venus/pkg/repo"
	th "github.com/filecoin-project/venus/pkg/testhelpers"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type mockHelloCallback struct {
	mock.Mock
}

func (msb *mockHelloCallback) HelloCallback(ci *types.ChainInfo) {
	msb.Called(ci.Sender, ci.Head.Key())
}

func TestHelloHandshake(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mn, err := mocknet.WithNPeers(2)
	require.NoError(t, err)

	a := mn.Hosts()[0]
	b := mn.Hosts()[1]

	builder := chain.NewBuilder(t, address.Undef)

	genesisA := builder.Genesis()
	store := builder.Store()
	mstore := builder.Mstore()
	heavy1 := builder.AppendOn(ctx, genesisA, 1)
	oldStore := copyStoreAndSetHead(ctx, t, store, heavy1)

	heavy2 := builder.AppendOn(ctx, heavy1, 1)
	_ = store.SetHead(ctx, heavy2)
	msc1, msc2 := new(mockHelloCallback), new(mockHelloCallback)

	// peer manager
	aPeerMgr, err := mockPeerMgr(ctx, t, a)
	require.NoError(t, err)

	// stm: @DISCOVERY_HELLO_REGISTER_001
	helloprotocol.NewHelloProtocolHandler(a, aPeerMgr, nil, oldStore, mstore, genesisA.Blocks()[0].Cid(), time.Second*30).Register(msc1.HelloCallback)
	helloprotocol.NewHelloProtocolHandler(b, aPeerMgr, nil, store, mstore, genesisA.Blocks()[0].Cid(), time.Second*30).Register(msc2.HelloCallback)

	msc1.On("HelloCallback", b.ID(), heavy2.Key()).Return()
	msc2.On("HelloCallback", a.ID(), heavy1.Key()).Return()

	require.NoError(t, mn.LinkAll())
	require.NoError(t, mn.ConnectAllButSelf())

	require.NoError(t, th.WaitForIt(10, 50*time.Millisecond, func() (bool, error) {
		var msc1Done bool
		var msc2Done bool
		for _, call := range msc1.Calls {
			if call.Method == "HelloCallback" {
				if _, differences := msc1.ExpectedCalls[0].Arguments.Diff(call.Arguments); differences == 0 {
					msc1Done = true
					break
				}
			}
		}
		for _, call := range msc2.Calls {
			if call.Method == "HelloCallback" {
				if _, differences := msc2.ExpectedCalls[0].Arguments.Diff(call.Arguments); differences == 0 {
					msc2Done = true
					break
				}
			}
		}

		return msc1Done && msc2Done, nil
	}))
}

func TestHelloBadGenesis(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mn, err := mocknet.WithNPeers(2)
	assert.NoError(t, err)

	a := mn.Hosts()[0]
	b := mn.Hosts()[1]

	builder := chain.NewBuilder(t, address.Undef)
	store := builder.Store()
	mstore := builder.Mstore()

	genesisA := builder.AppendOn(ctx, types.UndefTipSet, 1)
	heavy1 := builder.AppendOn(ctx, genesisA, 1)
	heavy2 := builder.AppendOn(ctx, heavy1, 1)
	_ = store.SetHead(ctx, heavy2)

	builder2 := chain.NewBuilder(t, address.Undef)
	genesisB := builder2.Build(ctx, types.UndefTipSet, 1, func(b *chain.BlockBuilder, i int) {
		b.SetTicket([]byte{1, 3, 4, 5, 6, 1, 3, 6, 7, 8})
	})

	fmt.Println(genesisB, genesisA)
	msc1, msc2 := new(mockHelloCallback), new(mockHelloCallback)

	// peer manager
	peerMgr, err := mockPeerMgr(ctx, t, a)
	require.NoError(t, err)

	helloprotocol.NewHelloProtocolHandler(a, peerMgr, nil, store, mstore, genesisA.Blocks()[0].Cid(), time.Second*30).Register(msc1.HelloCallback)
	helloprotocol.NewHelloProtocolHandler(b, peerMgr, nil, builder2.Store(), builder2.Mstore(), genesisB.Blocks()[0].Cid(), time.Second*30).Register(msc2.HelloCallback)

	msc1.On("HelloCallback", mock.Anything, mock.Anything, mock.Anything).Return()
	msc2.On("HelloCallback", mock.Anything, mock.Anything, mock.Anything).Return()

	require.NoError(t, mn.LinkAll())
	require.NoError(t, mn.ConnectAllButSelf())

	time.Sleep(time.Second)

	msc1.AssertNumberOfCalls(t, "HelloCallback", 0)
	msc2.AssertNumberOfCalls(t, "HelloCallback", 0)
}

func TestHelloMultiBlock(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mn, err := mocknet.WithNPeers(2)
	assert.NoError(t, err)

	a := mn.Hosts()[0]
	b := mn.Hosts()[1]

	builder := chain.NewBuilder(t, address.Undef)
	store := builder.Store()
	mstore := builder.Mstore()

	genesisTipset := builder.Genesis()
	assert.Equal(t, 1, genesisTipset.Len())

	heavy1 := builder.AppendOn(ctx, genesisTipset, 3)
	heavy1 = builder.AppendOn(ctx, heavy1, 3)
	oldStore := copyStoreAndSetHead(ctx, t, store, heavy1)

	heavy2 := builder.AppendOn(ctx, heavy1, 3)
	_ = store.SetHead(ctx, heavy2)
	msc1, msc2 := new(mockHelloCallback), new(mockHelloCallback)

	// peer manager
	peerMgr, err := mockPeerMgr(ctx, t, a)
	require.NoError(t, err)

	helloprotocol.NewHelloProtocolHandler(a, peerMgr, nil, oldStore, mstore, genesisTipset.At(0).Cid(), time.Second*30).Register(msc1.HelloCallback)
	helloprotocol.NewHelloProtocolHandler(b, peerMgr, nil, store, mstore, genesisTipset.At(0).Cid(), time.Second*30).Register(msc2.HelloCallback)

	msc1.On("HelloCallback", b.ID(), heavy2.Key()).Return()
	msc2.On("HelloCallback", a.ID(), heavy1.Key()).Return()

	assert.NoError(t, mn.LinkAll())
	assert.NoError(t, mn.ConnectAllButSelf())

	time.Sleep(time.Second * 5)

	msc1.AssertExpectations(t)
	msc2.AssertExpectations(t)
}

func mockPeerMgr(ctx context.Context, t *testing.T, h host.Host) (*peermgr.PeerMgr, error) {
	addrInfo, err := net.ParseAddresses(ctx, repo.NewInMemoryRepo().Config().Bootstrap.Addresses)
	require.NoError(t, err)

	return peermgr.NewPeerMgr(h, dht.NewDHT(ctx, h, ds.NewMapDatastore()), 10, addrInfo)
}

func copyStoreAndSetHead(ctx context.Context, t *testing.T, store *chain.Store, ts *types.TipSet) *chain.Store {
	storeCopy := *store //nolint
	err := storeCopy.SetHead(ctx, ts)
	require.NoError(t, err)
	return &storeCopy
}

package net

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"testing"

	ma "gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	pstore "gx/ipfs/QmRhFARzTHcFh8wUxwN5KvyTGq73FLC65EfFAhz8Ng7aGb/go-libp2p-peerstore"
	inet "gx/ipfs/QmTGxDz2CjBucFzPNTiWwzQmTWdrBnzqbqrMucDYMsjuPb/go-libp2p-net"
	peer "gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	msmux "gx/ipfs/QmabLh8TrJ3emfAoQk5AbqbLTbMyj7XqumMFmAFxa9epo8/go-multistream"
	ifconnmgr "gx/ipfs/QmcCk4LZRJPAKuwY9dusFea7LckELZgo5HagErTbm39o38/go-libp2p-interface-connmgr"
	host "gx/ipfs/Qmd52WKRSwrBK5gUaJKawryZQ5by6UbNB8KVW2Zy6JtbyW/go-libp2p-host"
	mh "gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"

	"github.com/filecoin-project/go-filecoin/types"
)

// RandPeerID is a libp2p random peer ID generator.
// These peer.ID generators were copied from libp2p/go-testutil. We didn't bring in the
// whole repo as a dependency because we only need this small bit. However if we find
// ourselves using more and more pieces we should just take a dependency on it.
func RandPeerID() (peer.ID, error) {
	buf := make([]byte, 16)
	if n, err := rand.Read(buf); n != 16 || err != nil {
		if n != 16 && err == nil {
			err = errors.New("couldnt read 16 random bytes")
		}
		panic(err)
	}
	h, _ := mh.Sum(buf, mh.SHA2_256, -1)
	return peer.ID(h), nil
}

func requireRandPeerID(t testing.TB) peer.ID { // nolint: deadcode
	p, err := RandPeerID()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

var _ host.Host = &fakeHost{}

type fakeHost struct {
	ConnectImpl func(context.Context, pstore.PeerInfo) error
}

func (fh *fakeHost) ID() peer.ID                  { panic("not implemented") }
func (fh *fakeHost) Peerstore() pstore.Peerstore  { panic("not implemented") }
func (fh *fakeHost) Addrs() []ma.Multiaddr        { panic("not implemented") }
func (fh *fakeHost) Network() inet.Network        { panic("not implemented") }
func (fh *fakeHost) Mux() *msmux.MultistreamMuxer { panic("not implemented") }
func (fh *fakeHost) Connect(ctx context.Context, pi pstore.PeerInfo) error {
	return fh.ConnectImpl(ctx, pi)
}
func (fh *fakeHost) SetStreamHandler(protocol.ID, inet.StreamHandler) {
	panic("not implemented")
}
func (fh *fakeHost) SetStreamHandlerMatch(protocol.ID, func(string) bool, inet.StreamHandler) {
	panic("not implemented")
}
func (fh *fakeHost) RemoveStreamHandler(protocol.ID) { panic("not implemented") }
func (fh *fakeHost) NewStream(context.Context, peer.ID, ...protocol.ID) (inet.Stream, error) {
	panic("not implemented")
}
func (fh *fakeHost) Close() error                       { panic("not implemented") }
func (fh *fakeHost) ConnManager() ifconnmgr.ConnManager { panic("not implemented") }

var _ inet.Dialer = &fakeDialer{}

type fakeDialer struct {
	PeersImpl func() []peer.ID
}

func (fd *fakeDialer) Peerstore() pstore.Peerstore                          { panic("not implemented") }
func (fd *fakeDialer) LocalPeer() peer.ID                                   { panic("not implemented") }
func (fd *fakeDialer) DialPeer(context.Context, peer.ID) (inet.Conn, error) { panic("not implemented") }
func (fd *fakeDialer) ClosePeer(peer.ID) error                              { panic("not implemented") }
func (fd *fakeDialer) Connectedness(peer.ID) inet.Connectedness             { panic("not implemented") }
func (fd *fakeDialer) Peers() []peer.ID {
	return fd.PeersImpl()
}
func (fd *fakeDialer) Conns() []inet.Conn              { panic("not implemented") }
func (fd *fakeDialer) ConnsToPeer(peer.ID) []inet.Conn { panic("not implemented") }
func (fd *fakeDialer) Notify(inet.Notifiee)            { panic("not implemented") }
func (fd *fakeDialer) StopNotify(inet.Notifiee)        { panic("not implemented") }

// TestFetcher is an object with the same method set as Fetcher plus a method
// for adding blocks to the source.  It is used to implement an object that
// behaves like Fetcher but does not go to the network for use in tests.
type TestFetcher struct {
	sourceBlocks map[string]*types.Block // sourceBlocks maps block cid strings to blocks.
}

// NewTestFetcher returns a TestFetcher with no source blocks.
func NewTestFetcher() *TestFetcher {
	return &TestFetcher{
		sourceBlocks: make(map[string]*types.Block),
	}
}

// AddSourceBlocks adds the input blocks to the fetcher source.
func (f *TestFetcher) AddSourceBlocks(blocks ...*types.Block) {
	for _, block := range blocks {
		f.sourceBlocks[block.Cid().String()] = block
	}
}

// GetBlocks returns any blocks in the source with matching cids.
func (f *TestFetcher) GetBlocks(ctx context.Context, cids []cid.Cid) ([]*types.Block, error) {
	var ret []*types.Block
	for _, c := range cids {
		if block, ok := f.sourceBlocks[c.String()]; ok {
			ret = append(ret, block)
		} else {
			return nil, fmt.Errorf("failed to fetch block: %s", c.String())
		}
	}
	return ret, nil
}

package filnet

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"

	host "gx/ipfs/QmPMtD39NN63AEUNghk1LFQcTLcCmYL8MtRzdv8BRUsC4Z/go-libp2p-host"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	inet "gx/ipfs/QmQSbtGXCyNrj34LWL8EgXyNNYDZ8r3SwQcpW5pPxVhLnM/go-libp2p-net"
	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	ifconnmgr "gx/ipfs/QmVz2p8ZVZ5GcWPNWGs2HZHiZyHumZcJpQdMRpxkMDhc2C/go-libp2p-interface-connmgr"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	msmux "gx/ipfs/QmbXRda5H2K3MSQyWWxTMtd8DWuguEBUCe6hpxfXVpFUGj/go-multistream"
	pstore "gx/ipfs/QmeKD8YT7887Xu6Z86iZmpYNxrLogJexqxEugSmaf14k64/go-libp2p-peerstore"
)

// These peer.ID generators were copied from libp2p/go-testutil. We didn't bring in the
// whole repo as a dependency because we only need this small bit. However if we find
// ourselves using more and more pieces we should just take a dependency on it.
func randPeerID() (peer.ID, error) {
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
	p, err := randPeerID()
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

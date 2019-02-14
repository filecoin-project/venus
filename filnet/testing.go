package filnet

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"

	ma "gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"
	inet "gx/ipfs/QmZ7cBWUXkyWTMN4qH6NGoyMVs7JugyFChBNP4ZUp5rJHH/go-libp2p-net"
	pstore "gx/ipfs/QmQFFp4ntkd4C14sP3FaH9WJyBuetuGUVo6dShNHvnoEvC/go-libp2p-peerstore"
	ifconnmgr "gx/ipfs/QmebAt96MwXHnbJ5uns6KLm3eSVLPDaaCB4DU7phQUi9a3/go-libp2p-interface-connmgr"
	peer "gx/ipfs/QmPJxxDsX2UbchSHobbYuvz7qnyJTFKvaKMzE2rZWJ4x5B/go-libp2p-peer"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	msmux "gx/ipfs/QmabLh8TrJ3emfAoQk5AbqbLTbMyj7XqumMFmAFxa9epo8/go-multistream"
	host "gx/ipfs/QmfRHxh8bt4jWLKRhNvR5fn7mFACrQBFLqV4wyoymEExKV/go-libp2p-host"
	mh "gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"
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

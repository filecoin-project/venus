package network

import (
	"context"

	multiaddr "gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"
	"gx/ipfs/QmNgLg1NTw37iWbYPKcyK85YJ9Whs1MkPtJwhfqbNYAyKg/go-libp2p-net"
	"gx/ipfs/QmPiemjiKBC9VA7vZF82m4x1oygtg2c2YVqag8PX7dN1BD/go-libp2p-peerstore"
	pstoremem "gx/ipfs/QmPiemjiKBC9VA7vZF82m4x1oygtg2c2YVqag8PX7dN1BD/go-libp2p-peerstore/pstoremem"
	"gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess"
	"gx/ipfs/QmSFo2QrMF4M1mKdB291ZqNtsie4NfwXCRdWgDU3inw4Ff/go-libp2p-interface-connmgr"
	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	peer "gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	"gx/ipfs/QmabLh8TrJ3emfAoQk5AbqbLTbMyj7XqumMFmAFxa9epo8/go-multistream"
)

// BlankValidator is for testing
type BlankValidator struct{}

// Validate is for testing
func (BlankValidator) Validate(_ string, _ []byte) error { return nil }

// Select is for testing
func (BlankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }

// NoopLibP2PHost is for testing
type NoopLibP2PHost struct{}

// ID is for testing
func (NoopLibP2PHost) ID() peer.ID {
	return ""
}

// Peerstore is for testing
func (NoopLibP2PHost) Peerstore() peerstore.Peerstore {
	return pstoremem.NewPeerstore()
}

// Addrs is for testing
func (NoopLibP2PHost) Addrs() []multiaddr.Multiaddr {
	return []multiaddr.Multiaddr{}
}

// Network is for testing
func (NoopLibP2PHost) Network() net.Network {
	return noopLibP2PNetwork{}
}

// Mux is for testing
func (NoopLibP2PHost) Mux() *multistream.MultistreamMuxer {
	panic("implement me")
}

// Connect is for testing
func (NoopLibP2PHost) Connect(ctx context.Context, pi peerstore.PeerInfo) error {
	return errors.New("Connect called on NoopLibP2PHost")
}

// SetStreamHandler is for testing
func (NoopLibP2PHost) SetStreamHandler(pid protocol.ID, handler net.StreamHandler) {

}

// SetStreamHandlerMatch is for testing
func (NoopLibP2PHost) SetStreamHandlerMatch(protocol.ID, func(string) bool, net.StreamHandler) {

}

// RemoveStreamHandler is for testing
func (NoopLibP2PHost) RemoveStreamHandler(pid protocol.ID) {
	panic("implement me")
}

// NewStream is for testing
func (NoopLibP2PHost) NewStream(ctx context.Context, p peer.ID, pids ...protocol.ID) (net.Stream, error) {
	return nil, errors.New("NewStream on NoopLibP2PHost")
}

// Close is for testing
func (NoopLibP2PHost) Close() error {
	return nil
}

// ConnManager is for testing
func (NoopLibP2PHost) ConnManager() ifconnmgr.ConnManager {
	panic("implement me")
}

type noopLibP2PNetwork struct{}

func (noopLibP2PNetwork) Peerstore() peerstore.Peerstore {
	panic("implement me")
}

func (noopLibP2PNetwork) LocalPeer() peer.ID {
	panic("implement me")
}

func (noopLibP2PNetwork) DialPeer(context.Context, peer.ID) (net.Conn, error) {
	panic("implement me")
}

func (noopLibP2PNetwork) ClosePeer(peer.ID) error {
	panic("implement me")
}

func (noopLibP2PNetwork) Connectedness(peer.ID) net.Connectedness {
	panic("implement me")
}

func (noopLibP2PNetwork) Peers() []peer.ID {
	return []peer.ID{}
}

func (noopLibP2PNetwork) Conns() []net.Conn {
	return []net.Conn{}
}

func (noopLibP2PNetwork) ConnsToPeer(p peer.ID) []net.Conn {
	return []net.Conn{}
}

func (noopLibP2PNetwork) Notify(net.Notifiee) {

}

func (noopLibP2PNetwork) StopNotify(net.Notifiee) {
	panic("implement me")
}

func (noopLibP2PNetwork) Close() error {
	panic("implement me")
}

func (noopLibP2PNetwork) SetStreamHandler(net.StreamHandler) {
	panic("implement me")
}

func (noopLibP2PNetwork) SetConnHandler(net.ConnHandler) {
	panic("implement me")
}

func (noopLibP2PNetwork) NewStream(context.Context, peer.ID) (net.Stream, error) {
	panic("implement me")
}

func (noopLibP2PNetwork) Listen(...multiaddr.Multiaddr) error {
	panic("implement me")
}

func (noopLibP2PNetwork) ListenAddresses() []multiaddr.Multiaddr {
	panic("implement me")
}

func (noopLibP2PNetwork) InterfaceListenAddresses() ([]multiaddr.Multiaddr, error) {
	panic("implement me")
}

func (noopLibP2PNetwork) Process() goprocess.Process {
	panic("implement me")
}

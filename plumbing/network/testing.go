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

type BlankValidator struct{}

func (BlankValidator) Validate(_ string, _ []byte) error        { return nil }
func (BlankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }

type NoopLibP2PHost struct{}

func (NoopLibP2PHost) ID() peer.ID {
	return ""
}

func (NoopLibP2PHost) Peerstore() peerstore.Peerstore {
	return pstoremem.NewPeerstore()
}

func (NoopLibP2PHost) Addrs() []multiaddr.Multiaddr {
	return []multiaddr.Multiaddr{}
}

func (NoopLibP2PHost) Network() net.Network {
	return NoopLibP2PNetwork{}
}

func (NoopLibP2PHost) Mux() *multistream.MultistreamMuxer {
	panic("implement me")
}

func (NoopLibP2PHost) Connect(ctx context.Context, pi peerstore.PeerInfo) error {
	return errors.New("Connect called on NoopLibP2PHost")
}

func (NoopLibP2PHost) SetStreamHandler(pid protocol.ID, handler net.StreamHandler) {

}

func (NoopLibP2PHost) SetStreamHandlerMatch(protocol.ID, func(string) bool, net.StreamHandler) {

}

func (NoopLibP2PHost) RemoveStreamHandler(pid protocol.ID) {
	panic("implement me")
}

func (NoopLibP2PHost) NewStream(ctx context.Context, p peer.ID, pids ...protocol.ID) (net.Stream, error) {
	return nil, errors.New("NewStream on NoopLibP2PHost")
}

func (NoopLibP2PHost) Close() error {
	return nil
}

func (NoopLibP2PHost) ConnManager() ifconnmgr.ConnManager {
	panic("implement me")
}

type NoopLibP2PNetwork struct{}

func (NoopLibP2PNetwork) Peerstore() peerstore.Peerstore {
	panic("implement me")
}

func (NoopLibP2PNetwork) LocalPeer() peer.ID {
	panic("implement me")
}

func (NoopLibP2PNetwork) DialPeer(context.Context, peer.ID) (net.Conn, error) {
	panic("implement me")
}

func (NoopLibP2PNetwork) ClosePeer(peer.ID) error {
	panic("implement me")
}

func (NoopLibP2PNetwork) Connectedness(peer.ID) net.Connectedness {
	panic("implement me")
}

func (NoopLibP2PNetwork) Peers() []peer.ID {
	return []peer.ID{}
}

func (NoopLibP2PNetwork) Conns() []net.Conn {
	return []net.Conn{}
}

func (NoopLibP2PNetwork) ConnsToPeer(p peer.ID) []net.Conn {
	return []net.Conn{}
}

func (NoopLibP2PNetwork) Notify(net.Notifiee) {

}

func (NoopLibP2PNetwork) StopNotify(net.Notifiee) {
	panic("implement me")
}

func (NoopLibP2PNetwork) Close() error {
	panic("implement me")
}

func (NoopLibP2PNetwork) SetStreamHandler(net.StreamHandler) {
	panic("implement me")
}

func (NoopLibP2PNetwork) SetConnHandler(net.ConnHandler) {
	panic("implement me")
}

func (NoopLibP2PNetwork) NewStream(context.Context, peer.ID) (net.Stream, error) {
	panic("implement me")
}

func (NoopLibP2PNetwork) Listen(...multiaddr.Multiaddr) error {
	panic("implement me")
}

func (NoopLibP2PNetwork) ListenAddresses() []multiaddr.Multiaddr {
	panic("implement me")
}

func (NoopLibP2PNetwork) InterfaceListenAddresses() ([]multiaddr.Multiaddr, error) {
	panic("implement me")
}

func (NoopLibP2PNetwork) Process() goprocess.Process {
	panic("implement me")
}

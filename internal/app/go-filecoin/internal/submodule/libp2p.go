package submodule

import (
	"context"

	"github.com/jbenet/goprocess"
	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/event"
	net "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/protocol"
	pstoremem "github.com/libp2p/go-libp2p-peerstore/pstoremem"
	multiaddr "github.com/multiformats/go-multiaddr"
	errors "github.com/pkg/errors"
)

type noopLibP2PHost struct{}

func (noopLibP2PHost) ID() peer.ID {
	return ""
}

func (noopLibP2PHost) Peerstore() peerstore.Peerstore {
	return pstoremem.NewPeerstore()
}

func (noopLibP2PHost) Addrs() []multiaddr.Multiaddr {
	return []multiaddr.Multiaddr{}
}

func (noopLibP2PHost) EventBus() event.Bus {
	panic("NYI")
}

func (noopLibP2PHost) Network() net.Network {
	return noopLibP2PNetwork{}
}

func (noopLibP2PHost) Mux() protocol.Switch {
	panic("implement me")
}

func (noopLibP2PHost) Connect(ctx context.Context, pi peer.AddrInfo) error {
	return errors.New("Connect called on noopLibP2PHost")
}

func (noopLibP2PHost) SetStreamHandler(pid protocol.ID, handler net.StreamHandler) {

}

func (noopLibP2PHost) SetStreamHandlerMatch(protocol.ID, func(string) bool, net.StreamHandler) {

}

func (noopLibP2PHost) RemoveStreamHandler(pid protocol.ID) {
	panic("implement me")
}

func (noopLibP2PHost) NewStream(ctx context.Context, p peer.ID, pids ...protocol.ID) (net.Stream, error) {
	return nil, errors.New("NewStream on noopLibP2PHost")
}

func (noopLibP2PHost) Close() error {
	return nil
}

func (noopLibP2PHost) ConnManager() connmgr.ConnManager {
	return &connmgr.NullConnMgr{}
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

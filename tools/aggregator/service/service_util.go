package aggregator

import (
	"context"
	"fmt"

	host "gx/ipfs/QmPMtD39NN63AEUNghk1LFQcTLcCmYL8MtRzdv8BRUsC4Z/go-libp2p-host"
	crypto "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	net "gx/ipfs/QmQSbtGXCyNrj34LWL8EgXyNNYDZ8r3SwQcpW5pPxVhLnM/go-libp2p-net"
	libp2p "gx/ipfs/QmVM6VuGaWcAaYjxG2om6XxMmpP3Rt9rw4nbMXVNYAPLhS/go-libp2p"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
)

// NewLibp2pHost creates a new libp2p host that listens on port `port`, and will
// have a peerID based on `priv`.
func NewLibp2pHost(ctx context.Context, priv crypto.PrivKey, port int) (host.Host, error) {
	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port)),
		libp2p.Identity(priv),
	}
	return libp2p.New(context.Background(), opts...)
}

// NewFullAddr create the multiaddress peers will dial to connect to host `h`
func NewFullAddr(h host.Host) (ma.Multiaddr, error) {
	// create the multiaddress peers will dial to connect to host `h`
	hAddr, err := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", h.ID().Pretty()))
	if err != nil {
		return nil, err
	}

	// Now we can build a full multiaddress to reach host `h`
	// by encapsulating both addresses:
	return h.Addrs()[0].Encapsulate(hAddr), nil
}

// RegisterNotifyBundle adds a net.NotifyBundle to host `h` and will update
// Tracker `t`'s TrackedNodes value.
func RegisterNotifyBundle(h host.Host, t *Tracker) {
	notify := &net.NotifyBundle{
		ListenF:      func(n net.Network, m ma.Multiaddr) { log.Debugf("Listener Opened: %s", m.String()) },
		ListenCloseF: func(n net.Network, m ma.Multiaddr) { log.Debugf("Listened Closed: %s", m.String()) },
		ConnectedF: func(n net.Network, c net.Conn) {
			log.Infof("Node Connected: %s", c.RemotePeer().Pretty())
			t.ConnectNode(c.RemotePeer().String())
		},
		DisconnectedF: func(n net.Network, c net.Conn) {
			log.Warningf("Node Disconnected: %s", c.RemotePeer().Pretty())
			t.DisconnectNode(c.RemotePeer().String())
		},
		OpenedStreamF: func(n net.Network, s net.Stream) { log.Debugf("Stream Opened: %s", s.Conn().RemotePeer().Pretty()) },
		ClosedStreamF: func(n net.Network, s net.Stream) { log.Debugf("Stream Opened: %s", s.Conn().RemotePeer().Pretty()) },
	}
	h.Network().Notify(notify)
}

package aggregator

import (
	"context"
	"encoding/json"

	host "gx/ipfs/QmPMtD39NN63AEUNghk1LFQcTLcCmYL8MtRzdv8BRUsC4Z/go-libp2p-host"
	crypto "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	net "gx/ipfs/QmQSbtGXCyNrj34LWL8EgXyNNYDZ8r3SwQcpW5pPxVhLnM/go-libp2p-net"
	logging "gx/ipfs/QmRREK2CAZ5Re2Bd9zZFG6FeYDppUWt5cMgsoUEp3ktgSr/go-log"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"

	fcmetrics "github.com/filecoin-project/go-filecoin/metrics"
	"github.com/filecoin-project/go-filecoin/tools/aggregator/event"
)

var log = logging.Logger("aggregator/node")

// EvtChan message channel type
type EvtChan chan event.HeartbeatEvent

// Service accepts heartbeats from filecoin nodes via a libp2p stream and
// exports metrics about them. It aggregates state over the connected nodes
// eg.to determine if the nodes are staying in consensus.
type Service struct {
	// Host is an object participating in a p2p network, which
	// implements protocols or provides services. It handles
	// requests like a Server, and issues requests like a Client.
	// It is called Host because it is both Server and Client (and Peer
	// may be confusing).
	Host host.Host

	// FullAddress is the complete multiaddress this Service is dialable on.
	FullAddress ma.Multiaddr

	// Tracer keeps track of how many nodes are connected to the aggregator service
	// as well as how many filecoin nodes are in and not in consensus.
	Tracker *Tracker

	ctx context.Context
}

// New creates a new aggregator service that listens on `listenPort` for
// libp2p connections.
func New(ctx context.Context, listenPort int, priv crypto.PrivKey) (*Service, error) {
	h, err := NewLibp2pHost(ctx, priv, listenPort)
	if err != nil {
		return nil, err
	}

	fullAddr, err := NewFullAddr(h)
	if err != nil {
		return nil, err
	}

	t := NewTracker()

	// Register callbacks for nodes connecting and diconnecting, these callbacks
	// will be used for updating the trackers `TrackedNodes` value.
	RegisterNotifyBundle(h, t)

	log.Infof("created aggregator, peerID: %s, listening on address: %s", h.ID().Pretty(), fullAddr.String())
	return &Service{
		Host:        h,
		FullAddress: fullAddr,
		Tracker:     t,
	}, nil
}

// Run will start a gorutine for each new connection from a filecoin node, and
// add the connected nodes heartbeat to consensus tracking.
func (a *Service) Run(ctx context.Context) {
	// we create an error group the manage error handling in the
	// input, filter, and output routines.
	a.Host.SetStreamHandler(fcmetrics.HeartbeatProtocol, func(s net.Stream) {
		go func(ctx context.Context) {
			log.Infof("new Stream with peer: %s", s.Conn().RemotePeer().Pretty())

			defer s.Close() // nolint: errcheck

			var peer = s.Conn().RemotePeer()
			dec := json.NewDecoder(s)

			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Assume first the message is JSON and try to decode it
					var hb fcmetrics.Heartbeat
					err := dec.Decode(&hb)
					if err != nil {
						if err.Error() == "connection reset" {
							return
						} else {
							log.Errorf("hearbeat decode failed: %s", err)
							return
						}
					}
					a.Tracker.TrackConsensus(peer.String(), hb.Tipset)
				}
			}
		}(ctx)
	})
}

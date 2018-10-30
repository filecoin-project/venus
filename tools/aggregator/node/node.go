package aggregator

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/sync/errgroup"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

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
type EvtChan chan event.LogEvent

// Service is a libp2p service that connectects to a filecoin node(s) via the
// heartbeat protocol, generates metrics, and streams the heartbeat to a sink
type Service struct {
	Host        host.Host
	FullAddress ma.Multiaddr

	Source EvtChan
	Sink   EvtChan

	Tracker *Tracker

	ctx context.Context
	eg  *errgroup.Group
}

// New creates a new aggregator node that listens on `listenPort` for
// libp2p connections, a file `peerKeyFile` may be passed to generate the peerID
// deterministically (helpful with infra).
func New(ctx context.Context, listenPort int, priv crypto.PrivKey) (*Service, error) {
	// create a libp2p host that listens on `listenPort` and has private key `priv`
	h, err := NewLibp2pHost(ctx, priv, listenPort)
	if err != nil {
		return nil, err
	}

	// build a full multiaddress to reach host `h`
	fullAddr, err := NewFullAddr(h)
	if err != nil {
		return nil, err
	}

	// register a net.NofityBundle on host `h`
	RegisterNotifyBundle(h)

	log.Infof("created aggregator, peerID: %s, listening on address: %s", h.ID().Pretty(), fullAddr.String())
	return &Service{
		Host:        h,
		FullAddress: fullAddr,

		Tracker: NewTracker(),

		Source: make(EvtChan),
		Sink:   make(EvtChan),
	}, nil
}

// Start will register interrupt handlers and start the aggregator node process
func (a *Service) Start(ctx context.Context) error {
	// Set up interrupts and child context `cctx`
	osSignalChan := make(chan os.Signal, 1)
	signal.Notify(osSignalChan, syscall.SIGTERM, os.Interrupt)

	ctx, cancel := context.WithCancel(ctx)
	go func(cancel context.CancelFunc) {
		sig := <-osSignalChan
		log.Info(sig)
		cancel()
	}(cancel)

	// we create an error group the manage error handling in the
	// input, filter, and output routines.
	a.eg, a.ctx = errgroup.WithContext(ctx)

	if err := a.startInput(); err != nil {
		return err
	}

	if err := a.startFilter(); err != nil {
		return err
	}

	if err := a.startOutput(); err != nil {
		return err
	}

	return nil
}

// Wait blocks until all function calls from the Go method have returned, then
// returns the first non-nil error (if any) from them.
func (a *Service) Wait() error {
	return a.eg.Wait()
}

// startInput will listen on for connections speaking "filecoin-stats/1.0.0" protocols,
// each new connection will spin off a go routine (manager by the error group) to accept
// Heartbeat events, decode them to json, and push the events into the `Source` channel
func (a *Service) startInput() error {
	a.Host.SetStreamHandler(fcmetrics.HeartbeatProtocol, func(s net.Stream) {
		a.eg.Go(func() error {
			log.Infof("new Stream with peer: %s", s.Conn().RemotePeer().Pretty())
			defer s.Close() // nolint: errcheck

			var peer = s.Conn().RemotePeer()
			dec := json.NewDecoder(s)

			for {
				select {
				case <-a.ctx.Done():
					return nil
				default:
				}
				// Assume first the message is JSON and try to decode it
				var hb fcmetrics.Heartbeat
				err := dec.Decode(&hb)
				if err == io.EOF {
					return err
				}
				if err != nil {
					//TODO better handling
					log.Errorf("hearbeat decode failed: %s", err)
					return s.Close()
				}
				a.Source <- event.LogEvent{
					FromPeer:          peer,
					ReceivedTimestamp: time.Now(),
					Heartbeat:         hb,
				}
			}
		})
	})
	return nil
}

// startFilter currently just passes all events through, although more could be done here
func (a *Service) startFilter() error {
	a.eg.Go(func() error {
		log.Info("starting filter")
		for {
			select {
			case <-a.ctx.Done():
				return nil
			case event := <-a.Source:
				a.Tracker.TrackConsensus(event.FromPeer.String(), event.Heartbeat.Tipset)
				a.Sink <- event
			}
		}
	})
	return nil
}

// startOutput prints all events it receives to stdout
func (a *Service) startOutput() error {
	a.eg.Go(func() error {
		log.Info("starting output")
		for {
			select {
			case <-a.ctx.Done():
				return nil
			case event := <-a.Sink:
				b, err := event.MarshalJSON()
				if err != nil {
					return err
				}
				fmt.Println(string(b))
				fmt.Println(a.Tracker.String())
			}
		}
	})
	return nil
}

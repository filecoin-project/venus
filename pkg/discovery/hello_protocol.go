package discovery

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/chainsync/exchange"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-state-types/abi"
	fbig "github.com/filecoin-project/go-state-types/big"
	fnet "github.com/filecoin-project/venus/pkg/net"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	net "github.com/libp2p/go-libp2p-core/network"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/filecoin-project/venus/pkg/metrics"
)

var log = logging.Logger("/fil/hello")

// helloProtocolID is the libp2p protocol identifier for the hello protocol.
const helloProtocolID = "/fil/hello/1.0.0"

var genesisErrCt = metrics.NewInt64Counter("hello_genesis_error", "Number of errors encountered in hello protocol due to incorrect genesis block")
var helloMsgErrCt = metrics.NewInt64Counter("hello_message_error", "Number of errors encountered in hello protocol due to malformed message")

// HelloMessage is the data structure of a single message in the hello protocol.
type HelloMessage struct {
	HeaviestTipSetCids   types.TipSetKey
	HeaviestTipSetHeight abi.ChainEpoch
	HeaviestTipSetWeight fbig.Int
	GenesisHash          cid.Cid
}

// LatencyMessage is written in response to a hello message for measuring peer
// latency.
type LatencyMessage struct {
	TArrival int64
	TSent    int64
}

// HelloProtocolHandler implements the 'Hello' protocol handler.
//
// Upon connecting to a new node, we send them a message
// containing some information about the state of our chain,
// and receive the same information from them. This is used to
// initiate a chainsync and detect connections to forks.
type HelloProtocolHandler struct {
	host host.Host

	genesis cid.Cid

	// peerDiscovered is called when new peers tell us about their chain
	peerDiscovered PeerDiscoveredCallback

	//  is used to retrieve the current heaviest tipset
	// for filling out our hello messages.
	getHeaviestTipSet GetTipSetFunc

	//helloTimeOut is block delay
	helloTimeOut time.Duration

	peerMgr      fnet.IPeerMgr
	exchange     exchange.Client
	chainStore   *chain.Store
	messageStore *chain.MessageStore
}

type PeerDiscoveredCallback func(ci *types.ChainInfo)

type GetTipSetFunc func() (*types.TipSet, error)

// NewHelloProtocolHandler creates a new instance of the hello protocol `Handler` and registers it to
// the given `host.Host`.
func NewHelloProtocolHandler(h host.Host,
	peerMgr fnet.IPeerMgr,
	exchange exchange.Client,
	chainStore *chain.Store,
	messageStore *chain.MessageStore,
	gen cid.Cid,
	helloTimeOut time.Duration,
) *HelloProtocolHandler {
	return &HelloProtocolHandler{
		host:         h,
		genesis:      gen,
		peerMgr:      peerMgr,
		exchange:     exchange,
		chainStore:   chainStore,
		messageStore: messageStore,
		helloTimeOut: helloTimeOut,
	}
}

// Register registers the handler with the network.
func (h *HelloProtocolHandler) Register(peerDiscoveredCallback PeerDiscoveredCallback, getHeaviestTipSet GetTipSetFunc) {
	// register callbacks
	h.peerDiscovered = peerDiscoveredCallback
	h.getHeaviestTipSet = getHeaviestTipSet

	// register a handle for when a new connection against someone is created
	h.host.SetStreamHandler(helloProtocolID, h.handleNewStream)

	// register for connection notifications
	h.host.Network().Notify((*helloProtocolNotifiee)(h))
}

func (h *HelloProtocolHandler) handleNewStream(s net.Stream) {
	ctx, cancel := context.WithTimeout(context.Background(), h.helloTimeOut)
	defer cancel()

	hello, err := h.receiveHello(ctx, s)
	if err != nil {
		helloMsgErrCt.Inc(ctx, 1)
		log.Debugf("failed to receive hello message:%s", err)
		// can't process a hello received in error, but leave this connection
		// open because we connections are innocent until proven guilty
		// (with bad genesis)
		return
	}
	latencyMsg := &LatencyMessage{TArrival: time.Now().UnixNano()}

	// process the hello message
	from := s.Conn().RemotePeer()
	if !hello.GenesisHash.Equals(h.genesis) {
		log.Debugf("peer genesis cid: %s does not match ours: %s, disconnecting from peer: %s", &hello.GenesisHash, h.genesis, from)
		genesisErrCt.Inc(context.Background(), 1)
		_ = s.Conn().Close()
		return
	}

	go func() {
		defer s.Close() // nolint: errcheck
		// Send the latendy message
		latencyMsg.TSent = time.Now().UnixNano()
		err = sendLatency(latencyMsg, s)
		if err != nil {
			log.Error(err)
		}
	}()

	protos, err := h.host.Peerstore().GetProtocols(from)
	if err != nil {
		log.Warnf("got error from peerstore.GetProtocols: %s", err)
	}
	if len(protos) == 0 {
		log.Warn("other peer hasnt completed libp2p identify, waiting a bit")
		// TODO: this better
		time.Sleep(time.Millisecond * 300)
	}

	fullTipSet, err := h.loadLocalFullTipset(ctx, hello.HeaviestTipSetCids)
	if err != nil {
		fullTipSet, err = h.exchange.GetFullTipSet(ctx, []peer.ID{from}, hello.HeaviestTipSetCids) //nolint
		if err == nil {
			for _, b := range fullTipSet.Blocks {
				_, err = h.chainStore.PutObject(ctx, b.Header)
				if err != nil {
					log.Errorf("fail to save block to tipset")
					return
				}
				_, err = h.messageStore.StoreMessages(ctx, b.SECPMessages, b.BLSMessages)
				if err != nil {
					log.Errorf("fail to save block to tipset")
					return
				}
			}
		}
		h.host.ConnManager().TagPeer(from, "new-block", 40)
	}
	if err != nil {
		log.Warnf("failed to get tipset message from peer %s", from)
		return
	}
	if fullTipSet == nil {
		log.Warnf("handleNewStream get null full tipset, it's scarce!")
		return
	}

	// notify the local node of the new `block.ChainInfo`
	h.peerMgr.AddFilecoinPeer(from)
	ci := types.NewChainInfo(from, from, fullTipSet.TipSet())
	h.peerDiscovered(ci)
}

func (h *HelloProtocolHandler) loadLocalFullTipset(ctx context.Context, tsk types.TipSetKey) (*types.FullTipSet, error) {
	ts, err := h.chainStore.GetTipSet(ctx, tsk)
	if err != nil {
		return nil, err
	}

	fts := &types.FullTipSet{}
	for _, b := range ts.Blocks() {
		smsgs, bmsgs, err := h.messageStore.LoadMetaMessages(ctx, b.Messages)
		if err != nil {
			return nil, err
		}

		fb := &types.FullBlock{
			Header:       b,
			BLSMessages:  bmsgs,
			SECPMessages: smsgs,
		}
		fts.Blocks = append(fts.Blocks, fb)
	}

	return fts, nil
}

// ErrBadGenesis is the error returned when a mismatch in genesis blocks happens.
var ErrBadGenesis = fmt.Errorf("bad genesis block")

func (h *HelloProtocolHandler) getOurHelloMessage() (*HelloMessage, error) {
	heaviest, err := h.getHeaviestTipSet()
	if err != nil {
		return nil, err
	}
	height := heaviest.Height()
	weight := heaviest.ParentWeight()

	return &HelloMessage{
		GenesisHash:          h.genesis,
		HeaviestTipSetCids:   heaviest.Key(),
		HeaviestTipSetHeight: height,
		HeaviestTipSetWeight: weight,
	}, nil
}

func (h *HelloProtocolHandler) receiveHello(ctx context.Context, s net.Stream) (*HelloMessage, error) {
	var hello HelloMessage
	err := hello.UnmarshalCBOR(s)
	return &hello, err

}

func (h *HelloProtocolHandler) receiveLatency(ctx context.Context, s net.Stream) (*LatencyMessage, error) {
	var latency LatencyMessage
	err := latency.UnmarshalCBOR(s)
	if err != nil {
		return nil, err
	}
	return &latency, nil
}

// sendHello send a hello message on stream `s`.
func (h *HelloProtocolHandler) sendHello(s net.Stream) error {
	msg, err := h.getOurHelloMessage()
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	if err := msg.MarshalCBOR(buf); err != nil {
		return err
	}

	n, err := s.Write(buf.Bytes())
	if err != nil {
		return err
	}
	if n != buf.Len() {
		return fmt.Errorf("could not write all hello message bytes")
	}
	return nil
}

// responding to latency
func sendLatency(msg *LatencyMessage, s net.Stream) error {
	buf := new(bytes.Buffer)
	if err := msg.MarshalCBOR(buf); err != nil {
		return err
	}
	n, err := s.Write(buf.Bytes())
	if err != nil {
		return err
	}
	if n != buf.Len() {
		return fmt.Errorf("could not write all latency message bytes")
	}
	return nil
}

// Note: hide `net.Notifyee` impl using a new-type
type helloProtocolNotifiee HelloProtocolHandler

const helloTimeout = time.Second * 10

func (hn *helloProtocolNotifiee) asHandler() *HelloProtocolHandler {
	return (*HelloProtocolHandler)(hn)
}

//
// `net.Notifyee` impl for `helloNotify`
//

func (hn *helloProtocolNotifiee) Connected(n net.Network, c net.Conn) {
	// Connected is invoked when a connection is made to a libp2p node.
	//
	// - open stream on connection
	// - send HelloMessage` on stream
	// - read LatencyMessage response on stream
	//
	// Terminate the connection if it has a different genesis block
	go func() {
		// add timeout
		ctx, cancel := context.WithTimeout(context.Background(), helloTimeout)
		defer cancel()
		s, err := hn.asHandler().host.NewStream(ctx, c.RemotePeer(), helloProtocolID)
		if err != nil {
			// If peer does not do hello keep connection open
			return
		}
		defer func() { _ = s.Close() }()
		// send out the hello message
		err = hn.asHandler().sendHello(s)
		if err != nil {
			log.Debugf("failed to send hello handshake to peer %s: %s", c.RemotePeer(), err)
			// Don't close connection for failed hello protocol impl
			return
		}

		// now receive latency message
		_, err = hn.asHandler().receiveLatency(ctx, s)
		if err != nil {
			log.Debugf("failed to receive hello latency msg from peer %s: %s", c.RemotePeer(), err)
			return
		}

	}()
}

func (hn *helloProtocolNotifiee) Listen(n net.Network, a ma.Multiaddr)      { /* empty */ }
func (hn *helloProtocolNotifiee) ListenClose(n net.Network, a ma.Multiaddr) { /* empty */ }
func (hn *helloProtocolNotifiee) Disconnected(n net.Network, c net.Conn)    { /* empty */ }
func (hn *helloProtocolNotifiee) OpenedStream(n net.Network, s net.Stream)  { /* empty */ }
func (hn *helloProtocolNotifiee) ClosedStream(n net.Network, s net.Stream)  { /* empty */ }

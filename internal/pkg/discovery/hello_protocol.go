package discovery

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	fbig "github.com/filecoin-project/go-state-types/big"
	fnet "github.com/filecoin-project/venus/internal/pkg/net"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	net "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/cborutil"
	"github.com/filecoin-project/venus/internal/pkg/enccid"
	"github.com/filecoin-project/venus/internal/pkg/encoding"
	"github.com/filecoin-project/venus/internal/pkg/metrics"
)

var log = logging.Logger("/fil/hello")

// helloProtocolID is the libp2p protocol identifier for the hello protocol.
const helloProtocolID = "/fil/hello/1.0.0"

var genesisErrCt = metrics.NewInt64Counter("hello_genesis_error", "Number of errors encountered in hello protocol due to incorrect genesis block")
var helloMsgErrCt = metrics.NewInt64Counter("hello_message_error", "Number of errors encountered in hello protocol due to malformed message")

// HelloMessage is the data structure of a single message in the hello protocol.
type HelloMessage struct {
	_                    struct{} `cbor:",toarray"`
	HeaviestTipSetCids   block.TipSetKey
	HeaviestTipSetHeight abi.ChainEpoch
	HeaviestTipSetWeight fbig.Int
	GenesisHash          enccid.Cid
}

// LatencyMessage is written in response to a hello message for measuring peer
// latency.
type LatencyMessage struct {
	_        struct{} `cbor:",toarray"`
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
	peerDiscovered peerDiscoveredCallback

	//  is used to retrieve the current heaviest tipset
	// for filling out our hello messages.
	getHeaviestTipSet getTipSetFunc

	networkName string

	peerMgr fnet.IPeerMgr
}

type peerDiscoveredCallback func(ci *block.ChainInfo)

type getTipSetFunc func() (*block.TipSet, error)

// NewHelloProtocolHandler creates a new instance of the hello protocol `Handler` and registers it to
// the given `host.Host`.
func NewHelloProtocolHandler(h host.Host, peerMgr fnet.IPeerMgr, gen cid.Cid, networkName string) *HelloProtocolHandler {
	return &HelloProtocolHandler{
		host:        h,
		genesis:     gen,
		networkName: networkName,
		peerMgr:     peerMgr,
	}
}

// Register registers the handler with the network.
func (h *HelloProtocolHandler) Register(peerDiscoveredCallback peerDiscoveredCallback, getHeaviestTipSet getTipSetFunc) {
	// register callbacks
	h.peerDiscovered = peerDiscoveredCallback
	h.getHeaviestTipSet = getHeaviestTipSet

	// register a handle for when a new connection against someone is created
	h.host.SetStreamHandler(helloProtocolID, h.handleNewStream)

	// register for connection notifications
	h.host.Network().Notify((*helloProtocolNotifiee)(h))
}

func (h *HelloProtocolHandler) handleNewStream(s net.Stream) {
	defer s.Close() // nolint: errcheck
	ctx := context.Background()
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
	ci, err := h.processHelloMessage(from, hello)
	switch {
	// no error
	case err == nil:
		// notify the local node of the new `block.ChainInfo`
		h.peerMgr.AddFilecoinPeer(from)
		h.peerDiscovered(ci)
	// processing errors
	case err == ErrBadGenesis:
		log.Debugf("peer genesis cid: %s does not match ours: %s, disconnecting from peer: %s", &hello.GenesisHash, h.genesis, from)
		genesisErrCt.Inc(context.Background(), 1)
		_ = s.Conn().Close()
		return
	default:
		// Note: we do not know why it failed, but we do not wish to shut down all protocols because of it
		log.Error(err)
	}

	// Send the latendy message
	latencyMsg.TSent = time.Now().UnixNano()
	err = h.sendLatency(latencyMsg, s)
	if err != nil {
		log.Error(err)
	}

	return
}

// ErrBadGenesis is the error returned when a mismatch in genesis blocks happens.
var ErrBadGenesis = fmt.Errorf("bad genesis block")

func (h *HelloProtocolHandler) processHelloMessage(from peer.ID, msg *HelloMessage) (*block.ChainInfo, error) {
	if !msg.GenesisHash.Equals(h.genesis) {
		return nil, ErrBadGenesis
	}

	// Note: both the sender and the source are the sender for the hello messages
	return block.NewChainInfo(from, from, msg.HeaviestTipSetCids, msg.HeaviestTipSetHeight), nil
}

func (h *HelloProtocolHandler) getOurHelloMessage() (*HelloMessage, error) {
	heaviest, err := h.getHeaviestTipSet()
	if err != nil {
		return nil, err
	}
	height, err := heaviest.Height()
	if err != nil {
		return nil, err
	}
	weight, err := heaviest.ParentWeight()
	if err != nil {
		return nil, err
	}

	return &HelloMessage{
		GenesisHash:          enccid.NewCid(h.genesis),
		HeaviestTipSetCids:   heaviest.Key(),
		HeaviestTipSetHeight: height,
		HeaviestTipSetWeight: weight,
	}, nil
}

func (h *HelloProtocolHandler) receiveHello(ctx context.Context, s net.Stream) (*HelloMessage, error) {
	var hello HelloMessage
	// Read cbor bytes from stream into hello message
	mr := cborutil.NewMsgReader(s)
	err := mr.ReadMsg(&hello)
	return &hello, err
}

func (h *HelloProtocolHandler) receiveLatency(ctx context.Context, s net.Stream) (*LatencyMessage, error) {
	var latency LatencyMessage
	rawLatency, err := ioutil.ReadAll(s)
	if err != nil {
		return nil, err
	}
	err = encoding.Decode(rawLatency, &latency)
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
	msgRaw, err := encoding.Encode(msg)
	if err != nil {
		return err
	}
	n, err := s.Write(msgRaw)
	if err != nil {
		return err
	}
	if n != len(msgRaw) {
		return fmt.Errorf("could not write all hello message bytes")
	}
	return nil
}

// responding to latency
func (h *HelloProtocolHandler) sendLatency(msg *LatencyMessage, s net.Stream) error {
	msgRaw, err := encoding.Encode(msg)
	if err != nil {
		return err
	}
	n, err := s.Write(msgRaw)
	if err != nil {
		return err
	}
	if n != len(msgRaw) {
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

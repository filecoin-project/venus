package helloprotocol

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/net/exchange"
	"github.com/filecoin-project/venus/pkg/net/peermgr"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/libp2p/go-eventbus"
	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-state-types/abi"
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	net "github.com/libp2p/go-libp2p-core/network"

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
type HelloProtocolHandler struct { //nolint
	host host.Host

	genesis cid.Cid

	// peerDiscovered is called when new peers tell us about their chain
	peerDiscovered PeerDiscoveredCallback

	//helloTimeOut is block delay
	helloTimeOut time.Duration

	peerMgr      peermgr.IPeerMgr
	exchange     exchange.Client
	chainStore   *chain.Store
	messageStore *chain.MessageStore
}

type PeerDiscoveredCallback func(ci *types.ChainInfo)

type GetTipSetFunc func() (*types.TipSet, error)

// NewHelloProtocolHandler creates a new instance of the hello protocol `Handler` and registers it to
// the given `host.Host`.
func NewHelloProtocolHandler(h host.Host,
	peerMgr peermgr.IPeerMgr,
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
func (h *HelloProtocolHandler) Register(peerDiscoveredCallback PeerDiscoveredCallback) {
	// register callbacks
	h.peerDiscovered = peerDiscoveredCallback

	// register a handle for when a new connection against someone is created
	h.host.SetStreamHandler(helloProtocolID, h.handleNewStream)

	sub, err := h.host.EventBus().Subscribe(new(event.EvtPeerIdentificationCompleted), eventbus.BufSize(1024))
	if err != nil {
		panic(fmt.Errorf("failed to subscribe to event bus: %w", err))
	}
	go func() {
		for evt := range sub.Out() {
			pic := evt.(event.EvtPeerIdentificationCompleted)
			if err := h.sayHello(context.Background(), pic.Peer); err != nil {
				protos, _ := h.host.Peerstore().GetProtocols(pic.Peer)
				agent, _ := h.host.Peerstore().Get(pic.Peer, "AgentVersion")
				if protosContains(protos, helloProtocolID) {
					log.Warnw("failed to say hello", "error", err, "peer", pic.Peer, "supported", protos, "agent", agent)
				} else {
					log.Debugw("failed to say hello", "error", err, "peer", pic.Peer, "supported", protos, "agent", agent)
				}
				return
			}
		}
	}()
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

	// notify the local node of the new `block.ChainInfo`
	h.peerMgr.AddFilecoinPeer(from)

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
		h.host.ConnManager().TagPeer(from, "fcpeer", 10)
	}
	if err != nil {
		log.Warnf("failed to get tipset message from peer %s", from)
		return
	}
	if fullTipSet == nil {
		log.Warnf("handleNewStream get null full tipset, it's scarce!")
		return
	}

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
	heaviest := h.chainStore.GetHead()
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

const helloTimeout = time.Second * 10

func (h *HelloProtocolHandler) sayHello(ctx context.Context, p peer.ID) error {
	// add timeout
	ctx, cancel := context.WithTimeout(ctx, helloTimeout)
	defer cancel()
	s, err := h.host.NewStream(ctx, p, helloProtocolID)
	if err != nil {
		// If peer does not do hello keep connection open
		return err
	}
	defer func() { _ = s.Close() }()
	// send out the hello message
	err = h.sendHello(s)
	if err != nil {
		log.Debugf("failed to send hello handshake to peer %s: %s", p, err)
		// Don't close connection for failed hello protocol impl
		return err
	}

	// now receive latency message
	_, err = h.receiveLatency(ctx, s)
	if err != nil {
		log.Debugf("failed to receive hello latency msg from peer %s: %s", p, err)
		return nil
	}

	return nil
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

func protosContains(protos []string, search string) bool {
	for _, p := range protos {
		if p == search {
			return true
		}
	}
	return false
}

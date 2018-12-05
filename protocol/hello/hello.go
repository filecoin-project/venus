package hello

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	ma "gx/ipfs/QmRKLtwMw131aK7ugC3G7ybpumMz78YrJe5dzneyindvG1/go-multiaddr"
	host "gx/ipfs/QmahxMNoNuSsgQefo9rkpcfRFmQrMN6Q99aztKXf63K7YJ/go-libp2p-host"
	peer "gx/ipfs/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"
	net "gx/ipfs/QmenvQQy4bFGSiHJUGupVmCRHfetg5rH3vTp9Z2f6v2KXR/go-libp2p-net"

	"github.com/filecoin-project/go-filecoin/consensus"
)

// Protocol is the libp2p protocol identifier for the hello protocol.
const protocol = "/fil/hello/1.0.0"

var log = logging.Logger("/fil/hello")

// Message is the data structure of a single message in the hello protocol.
type Message struct {
	HeaviestTipSetCids   []cid.Cid
	HeaviestTipSetHeight uint64
	GenesisHash          cid.Cid
}

type syncCallback func(from peer.ID, cids []cid.Cid, height uint64)

type getTipSetFunc func() consensus.TipSet

// Handler implements the 'Hello' protocol handler. Upon connecting to a new
// node, we send them a message containing some information about the state of
// our chain, and receive the same information from them. This is used to
// initiate a chainsync and detect connections to forks.
type Handler struct {
	host host.Host

	genesis cid.Cid

	// chainSyncCB is called when new peers tell us about their chain
	chainSyncCB syncCallback

	// getHeaviestTipSet is used to retrieve the current heaviest tipset
	// for filling out our hello messages.
	getHeaviestTipSet getTipSetFunc
}

// New creates a new instance of the hello protocol and registers it to
// the given host, with the provided callbacks.
func New(h host.Host, gen cid.Cid, syncCallback syncCallback, getHeaviestTipSet getTipSetFunc) *Handler {
	hello := &Handler{
		host:              h,
		genesis:           gen,
		chainSyncCB:       syncCallback,
		getHeaviestTipSet: getHeaviestTipSet,
	}
	h.SetStreamHandler(protocol, hello.handleNewStream)

	// register for connection notifications
	h.Network().Notify((*helloNotify)(hello))

	return hello
}

func (h *Handler) handleNewStream(s net.Stream) {
	defer s.Close() // nolint: errcheck

	from := s.Conn().RemotePeer()

	var hello Message
	if err := json.NewDecoder(s).Decode(&hello); err != nil {
		log.Warningf("bad hello message from peer %s: %s", from, err)
		return
	}

	switch err := h.processHelloMessage(from, &hello); err {
	case ErrBadGenesis:
		log.Error("bad genesis, disconnecting from peer")
		s.Conn().Close() // nolint: errcheck
		return
	default:
		log.Error(err)
	case nil:
		// ok
	}
}

// ErrBadGenesis is the error returned when a missmatch in genesis blocks happens.
var ErrBadGenesis = fmt.Errorf("bad genesis block")

func (h *Handler) processHelloMessage(from peer.ID, msg *Message) error {
	if !msg.GenesisHash.Equals(h.genesis) {
		log.Errorf("Their genesis cid: %s", msg.GenesisHash.String())
		log.Errorf("Our genesis cid: %s", h.genesis.String())
		return ErrBadGenesis
	}

	h.chainSyncCB(from, msg.HeaviestTipSetCids, msg.HeaviestTipSetHeight)
	return nil
}

func (h *Handler) getOurHelloMessage() *Message {
	heaviest := h.getHeaviestTipSet()
	height, err := heaviest.Height()
	if err != nil {
		panic("somehow heaviest tipset is empty")
	}

	return &Message{
		GenesisHash:          h.genesis,
		HeaviestTipSetCids:   heaviest.ToSortedCidSet().ToSlice(),
		HeaviestTipSetHeight: height,
	}
}

func (h *Handler) sayHello(ctx context.Context, p peer.ID) error {
	s, err := h.host.NewStream(ctx, p, protocol)
	if err != nil {
		return err
	}
	defer s.Close() // nolint: errcheck

	msg := h.getOurHelloMessage()

	return json.NewEncoder(s).Encode(msg)
}

// New peer connection notifications

type helloNotify Handler

func (hn *helloNotify) hello() *Handler {
	return (*Handler)(hn)
}

const helloTimeout = time.Second * 10

func (hn *helloNotify) Connected(n net.Network, c net.Conn) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), helloTimeout)
		defer cancel()
		p := c.RemotePeer()
		if err := hn.hello().sayHello(ctx, p); err != nil {
			log.Warningf("failed to send hello handshake to peer %s: %s", p, err)
		}
	}()
}

func (hn *helloNotify) Listen(n net.Network, a ma.Multiaddr)      {}
func (hn *helloNotify) ListenClose(n net.Network, a ma.Multiaddr) {}
func (hn *helloNotify) Disconnected(n net.Network, c net.Conn)    {}
func (hn *helloNotify) OpenedStream(n net.Network, s net.Stream)  {}
func (hn *helloNotify) ClosedStream(n net.Network, s net.Stream)  {}

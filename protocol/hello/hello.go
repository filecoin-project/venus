package hello

import (
	"context"
	"fmt"
	"time"

	ma "gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"
	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	net "gx/ipfs/QmTGxDz2CjBucFzPNTiWwzQmTWdrBnzqbqrMucDYMsjuPb/go-libp2p-net"
	peer "gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"
	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"
	host "gx/ipfs/Qmd52WKRSwrBK5gUaJKawryZQ5by6UbNB8KVW2Zy6JtbyW/go-libp2p-host"

	cbu "github.com/filecoin-project/go-filecoin/cborutil"
	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	cbor.RegisterCborType(Message{})
}

// Protocol is the libp2p protocol identifier for the hello protocol.
const protocol = "/fil/hello/1.0.0"

var log = logging.Logger("/fil/hello")

// Message is the data structure of a single message in the hello protocol.
type Message struct {
	HeaviestTipSetCids   []cid.Cid
	HeaviestTipSetHeight uint64
	GenesisHash          cid.Cid
	CommitSha            string
}

type syncCallback func(from peer.ID, cids []cid.Cid, height uint64)

type getTipSetFunc func() types.TipSet

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

	net       string
	commitSha string
}

// New creates a new instance of the hello protocol and registers it to
// the given host, with the provided callbacks.
func New(h host.Host, gen cid.Cid, syncCallback syncCallback, getHeaviestTipSet getTipSetFunc, net string, commitSha string) *Handler {
	hello := &Handler{
		host:              h,
		genesis:           gen,
		chainSyncCB:       syncCallback,
		getHeaviestTipSet: getHeaviestTipSet,
		net:               net,
		commitSha:         commitSha,
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
	if err := cbu.NewMsgReader(s).ReadMsg(&hello); err != nil {
		log.Warningf("bad hello message from peer %s: %s", from, err)
		return
	}

	switch err := h.processHelloMessage(from, &hello); err {
	case ErrBadGenesis:
		log.Warningf("genesis cid: %s does not match: %s, disconnecting from peer: %s", &hello.GenesisHash, h.genesis, from)
		s.Conn().Close() // nolint: errcheck
		return
	case ErrWrongVersion:
		log.Errorf("code not at same version: %s does not match %s, disconnecting from peer: %s", hello.CommitSha, h.commitSha, from)
		s.Conn().Close() // nolint: errcheck
		return
	case nil: // ok, noop
	default:
		log.Error(err)
	}
}

// ErrBadGenesis is the error returned when a mismatch in genesis blocks happens.
var ErrBadGenesis = fmt.Errorf("bad genesis block")

// ErrWrongVersion is the error returned when a mismatch in the code version happens.
var ErrWrongVersion = fmt.Errorf("code version mismatch")

func (h *Handler) processHelloMessage(from peer.ID, msg *Message) error {
	if !msg.GenesisHash.Equals(h.genesis) {
		return ErrBadGenesis
	}
	if h.net == "devnet-user" && msg.CommitSha != h.commitSha {
		return ErrWrongVersion
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
		CommitSha:            h.commitSha,
	}
}

func (h *Handler) sayHello(ctx context.Context, p peer.ID) error {
	s, err := h.host.NewStream(ctx, p, protocol)
	if err != nil {
		return err
	}
	defer s.Close() // nolint: errcheck

	msg := h.getOurHelloMessage()

	return cbu.NewMsgWriter(s).WriteMsg(&msg)
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

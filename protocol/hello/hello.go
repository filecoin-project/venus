package hello

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/host"
	net "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/filecoin-project/go-filecoin/block"
	cbu "github.com/filecoin-project/go-filecoin/cborutil"
	"github.com/filecoin-project/go-filecoin/metrics"
)

var log = logging.Logger("/fil/hello")

var genesisErrCt = metrics.NewInt64Counter("hello_genesis_error", "Number of errors encountered in hello protocol due to incorrect genesis block")
var helloMsgErrCt = metrics.NewInt64Counter("hello_message_error", "Number of errors encountered in hello protocol due to malformed message")

// Message is the data structure of a single message in the hello protocol.
type Message struct {
	HeaviestTipSetCids   block.TipSetKey
	HeaviestTipSetHeight uint64
	GenesisHash          cid.Cid
}

// Handler implements the 'Hello' protocol handler. Upon connecting to a new
// node, we send them a message containing some information about the state of
// our chain, and receive the same information from them. This is used to
// initiate a chainsync and detect connections to forks.
type Handler struct {
	host host.Host

	genesis cid.Cid

	// callBack is called when new peers tell us about their chain
	callBack helloCallback

	//  is used to retrieve the current heaviest tipset
	// for filling out our hello messages.
	getHeaviestTipSet getTipSetFunc

	networkName string
}

type helloCallback func(ci *block.ChainInfo)

type getTipSetFunc func() (block.TipSet, error)

// protocol is the libp2p protocol identifier for the hello protocol.
func helloProtocolID(networkName string) protocol.ID {
	return protocol.ID(fmt.Sprintf("/filecoin/hello/%s", networkName))
}

// New creates a new instance of the hello protocol `Handler` and registers it to
// the given `host.Host`.
func New(h host.Host, gen cid.Cid, helloCallback helloCallback, getHeaviestTipSet getTipSetFunc, networkName string) *Handler {
	hello := &Handler{
		host:              h,
		genesis:           gen,
		callBack:          helloCallback,
		getHeaviestTipSet: getHeaviestTipSet,
		networkName:       networkName,
	}
	h.SetStreamHandler(helloProtocolID(networkName), hello.handleNewStream)

	// register for connection notifications
	h.Network().Notify((*helloProtocolNotifiee)(hello))

	return hello
}

func (h *Handler) handleNewStream(s net.Stream) {
	defer s.Close() // nolint: errcheck
	if err := h.sendHello(s); err != nil {
		log.Debugf("failed to send hello message:%s", err)
	}
	return
}

// ErrBadGenesis is the error returned when a mismatch in genesis blocks happens.
var ErrBadGenesis = fmt.Errorf("bad genesis block")

func (h *Handler) processHelloMessage(from peer.ID, msg *Message) (*block.ChainInfo, error) {
	if !msg.GenesisHash.Equals(h.genesis) {
		return nil, ErrBadGenesis
	}

	return block.NewChainInfo(from, msg.HeaviestTipSetCids, msg.HeaviestTipSetHeight), nil
}

func (h *Handler) getOurHelloMessage() (*Message, error) {
	heaviest, err := h.getHeaviestTipSet()
	if err != nil {
		return nil, err
	}
	height, err := heaviest.Height()
	if err != nil {
		return nil, err
	}

	return &Message{
		GenesisHash:          h.genesis,
		HeaviestTipSetCids:   heaviest.Key(),
		HeaviestTipSetHeight: height,
	}, nil
}

func (h *Handler) receiveHello(ctx context.Context, p peer.ID) (*Message, error) {
	s, err := h.host.NewStream(ctx, p, helloProtocolID(h.networkName))
	if err != nil {
		return nil, err
	}
	defer func() { _ = s.Close() }()

	var hello Message
	if err := cbu.NewMsgReader(s).ReadMsg(&hello); err != nil {
		helloMsgErrCt.Inc(ctx, 1)
		return nil, err
	}
	return &hello, nil
}

// sendHello send a hello message on stream `s`.
func (h *Handler) sendHello(s net.Stream) error {
	msg, err := h.getOurHelloMessage()
	if err != nil {
		return err
	}
	return cbu.NewMsgWriter(s).WriteMsg(&msg)
}

// Note: hide `net.Notifyee` impl using a new-type
type helloProtocolNotifiee Handler

const helloTimeout = time.Second * 10

func (hn *helloProtocolNotifiee) asHandler() *Handler {
	return (*Handler)(hn)
}

//
// `net.Notifyee` impl for `helloNotify`
//

func (hn *helloProtocolNotifiee) Connected(n net.Network, c net.Conn) {
	// Connected is invoked when a connection is made to a libp2p node.
	//
	// - read `hello.Message` from connection `c` on a new thread.
	// - process the `hello.Message`.
	// - notify the local `Node` of the new `block.ChainInfo`.
	//
	// Terminate the connection if it fails to:
	//	   * validate, or
	//	   * pass the message information to its handlers callback function.
	go func() {
		// add timeout
		ctx, cancel := context.WithTimeout(context.Background(), helloTimeout)
		defer cancel()

		// receive the hello message
		from := c.RemotePeer()
		hello, err := hn.asHandler().receiveHello(ctx, from)
		if err != nil {
			log.Debugf("failed to receive hello handshake from peer %s: %s", from, err)
			_ = c.Close()
			return
		}

		// process the hello message
		ci, err := hn.asHandler().processHelloMessage(from, hello)
		switch {
		// no error
		case err == nil:
			// notify the local node of the new `block.ChainInfo`
			hn.asHandler().callBack(ci)
		// processing errors
		case err == ErrBadGenesis:
			log.Debugf("genesis cid: %s does not match: %s, disconnecting from peer: %s", &hello.GenesisHash, hn.asHandler().genesis, from)
			genesisErrCt.Inc(context.Background(), 1)
			_ = c.Close()
			return
		default:
			// Note: we do not know why it failed, but we do not wish to shut down all protocols because of it
			log.Error(err)
		}
	}()
}

func (hn *helloProtocolNotifiee) Listen(n net.Network, a ma.Multiaddr)      { /* empty */ }
func (hn *helloProtocolNotifiee) ListenClose(n net.Network, a ma.Multiaddr) { /* empty */ }
func (hn *helloProtocolNotifiee) Disconnected(n net.Network, c net.Conn)    { /* empty */ }
func (hn *helloProtocolNotifiee) OpenedStream(n net.Network, s net.Stream)  { /* empty */ }
func (hn *helloProtocolNotifiee) ClosedStream(n net.Network, s net.Stream)  { /* empty */ }

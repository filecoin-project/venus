package hello

import (
	"context"
	"fmt"
	"time"

	cid "github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"
	host "github.com/libp2p/go-libp2p-core/host"
	net "github.com/libp2p/go-libp2p-core/network"
	peer "github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"

	cbu "github.com/filecoin-project/go-filecoin/cborutil"
	"github.com/filecoin-project/go-filecoin/metrics"
	"github.com/filecoin-project/go-filecoin/types"
)

var versionErrCt = metrics.NewInt64Counter("hello_version_error", "Number of errors encountered in hello protocol due to incorrect version")
var genesisErrCt = metrics.NewInt64Counter("hello_genesis_error", "Number of errors encountered in hello protocol due to incorrect genesis block")
var helloMsgErrCt = metrics.NewInt64Counter("hello_message_error", "Number of errors encountered in hello protocol due to malformed message")

func init() {
	cbor.RegisterCborType(Message{})
}

// Protocol is the libp2p protocol identifier for the hello protocol.
const protocol = "/fil/hello/1.0.0"

var log = logging.Logger("/fil/hello")

// Message is the data structure of a single message in the hello protocol.
type Message struct {
	HeaviestTipSetCids   types.TipSetKey
	HeaviestTipSetHeight uint64
	GenesisHash          cid.Cid
	CommitSha            string
}

type helloCallback func(ci *types.ChainInfo)

type getTipSetFunc func() (types.TipSet, error)

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

	net       string
	commitSha string
}

// New creates a new instance of the hello protocol and registers it to
// the given host, with the provided callbacks.
func New(h host.Host, gen cid.Cid, helloCallback helloCallback, getHeaviestTipSet getTipSetFunc, net string, commitSha string) *Handler {
	hello := &Handler{
		host:              h,
		genesis:           gen,
		callBack:          helloCallback,
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
	if err := h.sendHello(s); err != nil {
		log.Errorf("failed to send hello message:%s", err)
	}
	return
}

// ErrBadGenesis is the error returned when a mismatch in genesis blocks happens.
var ErrBadGenesis = fmt.Errorf("bad genesis block")

// ErrWrongVersion is the error returned when a mismatch in the code version happens.
var ErrWrongVersion = fmt.Errorf("code version mismatch")

func (h *Handler) processHelloMessage(from peer.ID, msg *Message) (*types.ChainInfo, error) {
	if !msg.GenesisHash.Equals(h.genesis) {
		return nil, ErrBadGenesis
	}
	if (h.net == "devnet-staging" || h.net == "devnet-user") && msg.CommitSha != h.commitSha {
		return nil, ErrWrongVersion
	}

	return types.NewChainInfo(from, msg.HeaviestTipSetCids, msg.HeaviestTipSetHeight), nil
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
		CommitSha:            h.commitSha,
	}, nil
}

// ReceiveHello receives a hello message from peer `p` and returns it.
func (h *Handler) ReceiveHello(ctx context.Context, p peer.ID) (*Message, error) {
	s, err := h.host.NewStream(ctx, p, protocol)
	if err != nil {
		return nil, err
	}
	defer s.Close() // nolint: errcheck

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

// New peer connection notifications

type helloNotify Handler

func (hn *helloNotify) hello() *Handler {
	return (*Handler)(hn)
}

const helloTimeout = time.Second * 10

// Connect is the callback triggered when a connection is made to a libp2p node.
// Connect will read a hello message from connection `c`, terminate the connection if it fails to
// validate or pass the message information to its handlers callback function.
func (hn *helloNotify) Connected(n net.Network, c net.Conn) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), helloTimeout)
		defer cancel()

		// receive the hello message
		from := c.RemotePeer()
		hello, err := hn.hello().ReceiveHello(ctx, from)
		if err != nil {
			log.Warningf("failed to receive hello handshake from peer %s: %s", from, err)
			_ = c.Close()
			return
		}

		ci, err := hn.hello().processHelloMessage(from, hello)
		switch {
		case err == ErrBadGenesis:
			log.Debugf("genesis cid: %s does not match: %s, disconnecting from peer: %s", &hello.GenesisHash, hn.hello().genesis, from)
			genesisErrCt.Inc(context.TODO(), 1)
			return
		case err == ErrWrongVersion:
			log.Debugf("code not at same version: peer has version %s, daemon has version %s, disconnecting from peer: %s", hello.CommitSha, hn.hello().commitSha, from)
			versionErrCt.Inc(context.TODO(), 1)
			_ = c.Close()
			return
		case err == nil:
			hn.hello().callBack(ci)
		default:
			log.Error(err)
		}
	}()
}

func (hn *helloNotify) Listen(n net.Network, a ma.Multiaddr)      {}
func (hn *helloNotify) ListenClose(n net.Network, a ma.Multiaddr) {}
func (hn *helloNotify) Disconnected(n net.Network, c net.Conn)    {}
func (hn *helloNotify) OpenedStream(n net.Network, s net.Stream)  {}
func (hn *helloNotify) ClosedStream(n net.Network, s net.Stream)  {}

package pubsub

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/net/pubsub/ratelimit"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	message2 "github.com/ipni/go-libipni/announce/message"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

var log = logging.Logger("pubsub-index-message")

type peerMsgInfo struct {
	peerID    peer.ID
	lastCid   cid.Cid
	lastSeqno uint64
	rateLimit *ratelimit.Window
	mutex     sync.Mutex
}

type IndexerMessageValidator struct {
	self peer.ID

	peerCache  *lru.TwoQueueCache[address.Address, *peerMsgInfo]
	chainStore *chain.Store
}

func NewIndexerMessageValidator(self peer.ID, chainStore *chain.Store) *IndexerMessageValidator {
	peerCache, _ := lru.New2Q[address.Address, *peerMsgInfo](8192)

	return &IndexerMessageValidator{
		self:       self,
		peerCache:  peerCache,
		chainStore: chainStore,
	}
}

func (v *IndexerMessageValidator) Validate(ctx context.Context, pid peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
	// This chain-node should not be publishing its own messages.  These are
	// relayed from market-nodes.  If a node appears to be local, reject it.
	if pid == v.self {
		log.Debug("ignoring indexer message from self")
		return pubsub.ValidationIgnore
	}
	originPeer := msg.GetFrom()
	if originPeer == v.self {
		log.Debug("ignoring indexer message originating from self")
		return pubsub.ValidationIgnore
	}

	idxrMsg := message2.Message{}
	err := idxrMsg.UnmarshalCBOR(bytes.NewBuffer(msg.Data))
	if err != nil {
		log.Errorw("Could not decode indexer pubsub message", "err", err)
		return pubsub.ValidationReject
	}
	if len(idxrMsg.ExtraData) == 0 {
		log.Debugw("ignoring message missing miner id", "peer", originPeer)
		return pubsub.ValidationIgnore
	}

	// Get miner info from lotus
	minerAddr, err := address.NewFromBytes(idxrMsg.ExtraData)
	if err != nil {
		log.Warnw("cannot parse extra data as miner address", "err", err, "extraData", idxrMsg.ExtraData)
		return pubsub.ValidationReject
	}

	msgCid := idxrMsg.Cid

	msgInfo, cached := v.peerCache.Get(minerAddr)
	if !cached {
		msgInfo = &peerMsgInfo{}
	}

	// Lock this peer's message info.
	msgInfo.mutex.Lock()
	defer msgInfo.mutex.Unlock()

	var seqno uint64
	if cached {
		// Reject replayed messages.
		seqno = binary.BigEndian.Uint64(msg.Message.GetSeqno())
		if seqno <= msgInfo.lastSeqno {
			log.Debugf("ignoring replayed indexer message")
			return pubsub.ValidationIgnore
		}
	}

	if !cached || originPeer != msgInfo.peerID {
		// Check that the miner ID maps to the peer that sent the message.
		err = v.authenticateMessage(ctx, minerAddr, originPeer)
		if err != nil {
			log.Warnw("cannot authenticate message", "err", err, "peer", originPeer, "minerID", minerAddr)

			return pubsub.ValidationReject
		}
		msgInfo.peerID = originPeer
		if !cached {
			// Add msgInfo to cache only after being authenticated.  If two
			// messages from the same peer are handled concurrently, there is a
			// small chance that one msgInfo could replace the other here when
			// the info is first cached.  This is OK, so no need to prevent it.
			v.peerCache.Add(minerAddr, msgInfo)
		}
	}

	// Update message info cache with the latest message's sequence number.
	msgInfo.lastSeqno = seqno

	// See if message needs to be ignored due to rate limiting.
	if v.rateLimitPeer(msgInfo, msgCid) {
		return pubsub.ValidationIgnore
	}

	return pubsub.ValidationAccept
}

func (v *IndexerMessageValidator) rateLimitPeer(msgInfo *peerMsgInfo, msgCid cid.Cid) bool {
	const (
		msgLimit        = 5
		msgTimeLimit    = 10 * time.Second
		repeatTimeLimit = 2 * time.Hour
	)

	timeWindow := msgInfo.rateLimit

	// Check overall message rate.
	if timeWindow == nil {
		timeWindow = ratelimit.NewWindow(msgLimit, msgTimeLimit)
		msgInfo.rateLimit = timeWindow
	} else if msgInfo.lastCid == msgCid {
		// Check if this is a repeat of the previous message data.
		if time.Since(timeWindow.Newest()) < repeatTimeLimit {
			log.Warnw("ignoring repeated indexer message", "sender", msgInfo.peerID)
			return true
		}
	}

	err := timeWindow.Add()
	if err != nil {
		log.Warnw("ignoring indexer message", "sender", msgInfo.peerID, "err", err)
		return true
	}

	msgInfo.lastCid = msgCid

	return false
}

func (v *IndexerMessageValidator) authenticateMessage(ctx context.Context, minerAddress address.Address, peerID peer.ID) error {
	minerPeerID, err := getMinerPeer(ctx, minerAddress, v.chainStore)
	if err != nil {
		return err
	}

	if minerPeerID != peerID {
		return fmt.Errorf("miner id does not map to peer that sent message")
	}

	return nil
}

func getMinerPeer(ctx context.Context, maddr address.Address, chainStore *chain.Store) (peer.ID, error) {
	view := state.NewView(chainStore.Store(ctx), chainStore.GetHead().ParentState())
	act, err := view.LoadActor(ctx, maddr)
	if err != nil {
		return peer.ID(""), fmt.Errorf("failed to load miner actor: %v", err)
	}
	mas, err := miner.Load(chainStore.Store(ctx), act)
	if err != nil {
		return peer.ID(""), fmt.Errorf("failed to load miner actor state: %v", err)
	}
	info, err := mas.Info()
	if err != nil {
		return peer.ID(""), fmt.Errorf("failed to load miner info: %v", err)
	}

	peerID, err := peer.IDFromBytes(info.PeerId)
	if err != nil {
		return peer.ID(""), fmt.Errorf("miner not set peer id")
	}

	return peerID, nil
}

package lookup

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	logging "gx/ipfs/QmRb5jh8z2E8hMGN2tkvs1yHynUanqnZ3UeKwgN1i9P1F8/go-log"
	floodsub "gx/ipfs/QmSFihvoND3eDaAYRCeLgLPt62yCPgMZs1NSZmKFEtJQQw/go-libp2p-floodsub"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	pubsub "gx/ipfs/QmdbxjQWogRCHRaxhhGnYdT1oQJzL9GdqSKzCdqWr85AP2/pubsub"

	types "github.com/filecoin-project/go-filecoin/types"
	wallet "github.com/filecoin-project/go-filecoin/wallet"
)

var FilLookupTopic = "/fil/lookup/" // nolint: golint

var log = logging.Logger("lookup")

// LookupEngine can be used to find map address's -> peerId's
type LookupEngine struct { // nolint: golint
	lk    sync.Mutex
	cache map[types.Address]peer.ID

	ourPeerID peer.ID

	reqPubsub *pubsub.PubSub

	ps *floodsub.PubSub

	Wallet *wallet.Wallet
}

// NewLookupEngine returns an engine for looking up peerId's
func NewLookupEngine(ps *floodsub.PubSub, wallet *wallet.Wallet, self peer.ID) (*LookupEngine, error) {
	sub, err := ps.Subscribe(FilLookupTopic) // nolint: errcheck
	if err != nil {
		return nil, err
	}

	le := &LookupEngine{
		ps:        ps,
		cache:     make(map[types.Address]peer.ID),
		ourPeerID: self,
		reqPubsub: pubsub.New(128),
		Wallet:    wallet,
	}

	go le.HandleMessages(sub)
	return le, nil
}

type message struct {
	Address types.Address
	Peer    string
	Request bool
}

// HandleMessages manages sending and receieveing messages
func (le *LookupEngine) HandleMessages(s *floodsub.Subscription) {
	defer s.Cancel()
	ctx := context.TODO()
	for {
		msg, err := s.Next(ctx)
		if err != nil {
			log.Error("from subscription.Next(): ", err)
			return
		}

		if msg.GetFrom() == le.ourPeerID {
			continue
		}

		var m message
		// TODO: Replace with cbor
		if err := json.Unmarshal(msg.GetData(), &m); err != nil {
			log.Error("malformed message: ", err)
			continue
		}

		le.lk.Lock()
		if m.Request {
			if le.Wallet.HasAddress(m.Address) {
				go le.SendMessage(&message{
					Address: m.Address,
					Peer:    le.ourPeerID.Pretty(),
				})
			}
		} else {
			pid, err := peer.IDB58Decode(m.Peer)
			if err != nil {
				log.Error("bad peer ID: ", err)
				continue
			}
			le.cache[m.Address] = pid
			le.reqPubsub.Pub(pid, m.Address.String())
		}
		le.lk.Unlock()
	}
}

// SendMessage publishes message m on FilLookupTopic
func (le *LookupEngine) SendMessage(m *message) {
	// TODO: Replace with cbor
	d, err := json.Marshal(m)
	if err != nil {
		log.Error("failed to marshal message: ", err)
		return
	}

	if err := le.ps.Publish(FilLookupTopic, d); err != nil { // nolint: errcheck
		log.Error("publish failed: ", err)
	}
}

// Lookup returns the peerId associated with address a
func (le *LookupEngine) Lookup(ctx context.Context, a types.Address) (peer.ID, error) {
	le.lk.Lock()
	v, ok := le.cache[a]
	le.lk.Unlock()
	if ok {
		return v, nil
	}

	ch := le.reqPubsub.SubOnce(a.String())

	le.SendMessage(&message{
		Address: a,
		Request: true,
	})

	select {
	case out := <-ch:
		return out.(peer.ID), nil
	case <-time.After(time.Second * 10):
		return "", fmt.Errorf("timed out waiting for response")
	case <-ctx.Done():
		return "", fmt.Errorf("context cancled")
	}
}

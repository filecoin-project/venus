package bazaar

import "github.com/libp2p/go-libp2p-core/peer"

// developers can define the types except Ping
const EventPing EventType = 0

type EventType uint64

type Event struct {
	Type EventType
	Data []byte
	Time int64
}

type EventHandler func(peer.ID, *Event) error

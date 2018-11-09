package event

import (
	"time"

	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"

	jsoniter "github.com/json-iterator/go"

	fcmetrics "github.com/filecoin-project/go-filecoin/metrics"
)

// HeartbeatEvent contains a heartbeat, the time it was received and who it was from
type HeartbeatEvent struct {
	// FromPeer is who created the event
	FromPeer peer.ID `json:"fromPeer"`
	// ReceivedTimestamp represents when the event was received
	ReceivedTimestamp time.Time `json:"receivedTimestamp"`
	// Heartbeat data sent by `FromPeer`
	Heartbeat fcmetrics.Heartbeat `json:"heartbeat"`
}

// MarshalJSON marshals a HeartbeatEvent to json
func (t HeartbeatEvent) MarshalJSON() (data []byte, err error) {
	event := t.getJSONMap()
	return jsoniter.Marshal(event)
}

func (t HeartbeatEvent) getJSONMap() map[string]interface{} {
	event := map[string]interface{}{
		"receivedTimestamp": t.ReceivedTimestamp.UTC().Format(`2000-01-01T15:04:05.999999999Z`),
		"fromPeer":          t.FromPeer.Pretty(),
		"heartbeat":         t.Heartbeat,
	}
	return event
}

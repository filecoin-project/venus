package event

import (
	"time"

	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"

	jsoniter "github.com/json-iterator/go"

	fcmetrics "github.com/filecoin-project/go-filecoin/metrics"
)

// LogEvent contains a heartbeat, the time it was received and who it was from
type LogEvent struct {
	// FromPeer is who created the event
	FromPeer peer.ID `json:"peer"`
	// ReceivedTimestamp represents when the event was received
	ReceivedTimestamp time.Time `json:"timestamp"`
	// Heartbeat data sent with event
	Heartbeat fcmetrics.Heartbeat `json:"heartbeat"`
}

// MarshalJSON marshals a LogEvent to json
func (t LogEvent) MarshalJSON() (data []byte, err error) {
	event := t.getJSONMap()
	return jsoniter.Marshal(event)
}

func (t LogEvent) getJSONMap() map[string]interface{} {
	event := map[string]interface{}{
		"receivedTimestamp": t.ReceivedTimestamp.UTC().Format(`2000-01-01T15:04:05.999999999Z`),
		"fromPeer":          t.FromPeer.Pretty(),
		"heartbeat":         t.Heartbeat,
	}
	return event
}

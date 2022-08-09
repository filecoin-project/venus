package net

import (
	"fmt"

	"github.com/libp2p/go-libp2p-core/protocol"
)

// FilecoinDHT is creates a protocol for the filecoin DHT.
func FilecoinDHT(network string) protocol.ID {
	return protocol.ID(fmt.Sprintf("/fil/kad/%s", network))
}

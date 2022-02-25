package hello

import (
	"fmt"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
)

var ErrBadGenesis = fmt.Errorf("bad genesis block")

const ProtocolID = "/fil/hello/1.0.0"

// GreetingMessage is the data structure of a single message in the hello protocol.
type GreetingMessage struct {
	HeaviestTipSet       []cid.Cid
	HeaviestTipSetHeight abi.ChainEpoch
	HeaviestTipSetWeight big.Int
	GenesisHash          cid.Cid
}

// LatencyMessage is written in response to a hello message for measuring peer
// latency.
type LatencyMessage struct {
	TArrival int64
	TSent    int64
}

package blocksub

import (
	"fmt"
)

// BlockTopic returns the network pubsub topic identifier on which new blocks are announced.
func Topic(networkName string) string {
	return fmt.Sprintf("/fil/blocks/%s", networkName)
}

// Coming soon:
//type Message struct {
//	_           struct{} `cbor:",toarray"`
//	Header      block.Block
//	BLSMsgCids  []e.Cid
//	SECPMsgCids []e.Cid
//}

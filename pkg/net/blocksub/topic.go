package blocksub

import (
	"fmt"
)

// BlockTopic returns the network pubsub topic identifier on which new blocks are announced.
func Topic(networkName string) string {
	return fmt.Sprintf("/fil/blocks/%s", networkName)
}

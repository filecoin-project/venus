package net

import "fmt"

// BlockTopic returns the network pubsub topic identifier on which new blocks are announced.
func BlockTopic(networkName string) string {
	return fmt.Sprintf("/fil/blocks/%s", networkName)
}

// MessageTopic returns the network pubsub topic identifier on which new messages are announced.
func MessageTopic(networkName string) string {
	return fmt.Sprintf("/fil/msgs/%s", networkName)
}

package msgsub

import "fmt"

// MessageTopic returns the network pubsub topic identifier on which new messages are announced.
// The message payload is just a SignedMessage.
func Topic(networkName string) string {
	return fmt.Sprintf("/fil/msgs/%s", networkName)
}

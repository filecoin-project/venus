package node

import (
	"fmt"
)

// Node represents a full Filecoin node.
type Node struct{}

// New creates a new node.
func New() *Node {
	return &Node{}
}

// Start boots up the node.
func (node *Node) Start() error {
	fmt.Println("booting to filecoin :)")
	return nil
}

// Stop initiates the shutdown of the node.
func (node *Node) Stop() {
	fmt.Println("stopping filecoin :(")
}

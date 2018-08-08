package api_impl

import (
	"context"
)

// NodeDaemon is an implementation of the api.Daemon interface for node.
type NodeDaemon struct {
	api *API
}

// NewNodeDaemon creates an instance of the NodeDaemon struct.
func NewNodeDaemon(api *API) *NodeDaemon {
	return &NodeDaemon{api: api}
}

// Start, starts a new daemon process.
func (api *NodeDaemon) Start(ctx context.Context) error {
	return api.api.node.Start()
}

// Stop, shuts down the daemon and cleans up any resources.
func (api *NodeDaemon) Stop(ctx context.Context) error {
	api.api.node.Stop()

	return nil
}

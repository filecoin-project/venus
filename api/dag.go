package api

import "context"

// Dag is the interface that defines methods to interact with IPLD DAG objects.
type Dag interface {
	// Get returns the associated DAG node for the passed in CID.
	Get(ctx context.Context, ref string) (interface{}, error)
}

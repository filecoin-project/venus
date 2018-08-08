package api

import "context"

type Dag interface {
	// Get returns the associated DAG node for the passed in CID.
	Get(ctx context.Context, ref string) (interface{}, error)
}

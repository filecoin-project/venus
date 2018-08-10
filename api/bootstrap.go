package api

import (
	"context"
)

// Bootstrap is the interface that defines methods to show, edit and list bootstrap peers.
type Bootstrap interface {
	Ls(ctx context.Context) ([]string, error)
}

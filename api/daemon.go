package api

import (
	"context"
)

type Daemon interface {
	// Start, starts a new daemon process.
	Start(ctx context.Context) error
	// Stop, shuts down the daemon and cleans up any resources.
	Stop(ctx context.Context) error
}

package api

import (
	"context"
	"io"
)

// Log is the interface that defines methods to interact with the event log output of the daemon.
type Log interface {
	Tail(ctx context.Context) io.Reader
}

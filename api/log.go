package api

import (
	"context"
	"io"

	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
)

// Log is the interface that defines methods to interact with the event log output of the daemon.
type Log interface {
	Tail(ctx context.Context) io.Reader
	StreamTo(ctx context.Context, maddr ma.Multiaddr) error
}

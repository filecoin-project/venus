package api

import (
	"context"
	"time"

	peer "gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"
)

type PingResult struct {
	Time    time.Duration
	Text    string
	Success bool
}

type Ping interface {
	Ping(ctx context.Context, pid peer.ID, count uint, delay time.Duration) (<-chan *PingResult, error)
}

package porcelain

import (
	"context"
	"fmt"
	"time"

	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
)

// PingResult is the data that gets emitted on the Ping channel.
type PingResult struct {
	Time    time.Duration
	Text    string
	Success bool
}

// pingTimeout is the maximum timeout that is waited for when pinging another peer.
const pingTimeout = time.Second * 10

type pPlumbing interface {
	NetworkGetPeerID() peer.ID
	NetworkPing(ctx context.Context, pid peer.ID) (<-chan time.Duration, error)
}

// NetworkPingWithCount pings a peer repeatedly returning delay information each time
func NetworkPingWithCount(ctx context.Context, plumbing pPlumbing, pid peer.ID, count uint, delay time.Duration) (<-chan *PingResult, error) {
	if pid == plumbing.NetworkGetPeerID() {
		return nil, errors.New("cannot ping self")
	}

	ctx, cancel := context.WithCancel(ctx)

	times, err := plumbing.NetworkPing(ctx, pid)
	if err != nil {
		cancel()
		return nil, err
	}

	ch := make(chan *PingResult)

	go func() {
		defer close(ch)
		defer cancel()

		ch <- &PingResult{Text: fmt.Sprintf("PING %s", pid)}
		for i := 0; i < int(count); i++ {
			select {
			case dur := <-times:
				ch <- &PingResult{Time: dur, Success: true}
			case <-time.After(pingTimeout):
				ch <- &PingResult{Text: "error: timeout"}
			case <-ctx.Done():
				return
			}

			time.Sleep(delay)
		}
	}()

	return ch, nil
}

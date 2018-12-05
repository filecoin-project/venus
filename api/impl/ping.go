package impl

import (
	"context"
	"fmt"
	"time"

	peer "gx/ipfs/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/api"
)

type nodePing struct {
	api *nodeAPI
}

func newNodePing(api *nodeAPI) *nodePing {
	return &nodePing{api: api}
}

// pingTimeout is the maximum timeout that is waited for when pinging another peer.
const pingTimeout = time.Second * 10

func (np *nodePing) Ping(ctx context.Context, pid peer.ID, count uint, delay time.Duration) (<-chan *api.PingResult, error) {
	nd := np.api.node

	if pid == nd.Host().ID() {
		return nil, ErrCannotPingSelf
	}

	ctx, cancel := context.WithCancel(ctx)

	times, err := nd.Ping.Ping(ctx, pid)
	if err != nil {
		cancel()
		return nil, err
	}

	ch := make(chan *api.PingResult)

	go func() {
		defer close(ch)
		defer cancel()

		ch <- &api.PingResult{Text: fmt.Sprintf("PING %s", pid)}
		for i := 0; i < int(count); i++ {
			select {
			case dur := <-times:
				ch <- &api.PingResult{Time: dur, Success: true}
			case <-time.After(pingTimeout):
				ch <- &api.PingResult{Text: "error: timeout"}
			case <-ctx.Done():
				return
			}

			time.Sleep(delay)
		}
	}()

	return ch, nil
}

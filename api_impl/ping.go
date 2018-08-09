package api_impl

import (
	"context"
	"fmt"
	"time"

	peer "gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/api"
)

type NodePing struct {
	api *API
}

func NewNodePing(api *API) *NodePing {
	return &NodePing{api: api}
}

const PingTimeout = time.Second * 10

func (np *NodePing) Ping(ctx context.Context, pid peer.ID, count uint, delay time.Duration) (<-chan *api.PingResult, error) {
	nd := np.api.node

	if pid == nd.Host.ID() {
		return nil, ErrCannotPingSelf
	}

	ctx, cancel := context.WithCancel(ctx)

	times, err := nd.Ping.Ping(ctx, pid)
	if err != nil {
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
			case <-time.After(PingTimeout):
				ch <- &api.PingResult{Text: "error: timeout"}
			case <-ctx.Done():
				return
			}

			time.Sleep(delay)
		}
	}()

	return ch, nil
}

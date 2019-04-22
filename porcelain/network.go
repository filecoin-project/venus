package porcelain

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-peer"
	"github.com/pkg/errors"
)

type netPlumbing interface {
	NetworkPing(ctx context.Context, pid peer.ID) (<-chan time.Duration, error)
}

// PingMinerWithTimeout pings a storage or retrieval miner, waiting the given
// timeout and returning descriptive errors.
func PingMinerWithTimeout(ctx context.Context, minerPID peer.ID, timeout time.Duration, plumbing netPlumbing) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	res, err := netPlumbing.NetworkPing(plumbing, ctx, minerPID)
	if err != nil {
		return err
	}

	select {
	case _, ok := <-res:
		if !ok {
			return errors.New("couldn't establish connection to miner: ping channel closed")
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("couldn't establish connection to miner: %s, timed out after %s", ctx.Err(), timeout.String())
	}
}

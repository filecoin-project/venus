package events

import (
	"context"
	"sync"

	"github.com/hashicorp/golang-lru/arc/v2"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/venus-shared/types"
)

type messageCache struct {
	api IEvent

	blockMsgLk    sync.Mutex
	blockMsgCache *arc.ARCCache[cid.Cid, *types.BlockMessages]
}

func newMessageCache(api IEvent) *messageCache {
	blsMsgCache, _ := arc.NewARC[cid.Cid, *types.BlockMessages](500)

	return &messageCache{
		api:           api,
		blockMsgCache: blsMsgCache,
	}
}

func (c *messageCache) ChainGetBlockMessages(ctx context.Context, blkCid cid.Cid) (*types.BlockMessages, error) {
	c.blockMsgLk.Lock()
	defer c.blockMsgLk.Unlock()

	msgs, ok := c.blockMsgCache.Get(blkCid)
	var err error
	if !ok {
		msgs, err = c.api.ChainGetBlockMessages(ctx, blkCid)
		if err != nil {
			return nil, err
		}
		c.blockMsgCache.Add(blkCid, msgs)
	}
	return msgs, nil
}

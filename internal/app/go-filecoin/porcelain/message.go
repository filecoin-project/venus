package porcelain

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/util/moresync"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
)

type waitPlumbing interface {
	MessageWait(context.Context, cid.Cid, func(*block.Block, *types.SignedMessage, *vm.MessageReceipt) error) error
}

// MessageWaitDone blocks until the given message cid appears on chain
func MessageWaitDone(ctx context.Context, plumbing waitPlumbing, msgCid cid.Cid) (*vm.MessageReceipt, error) {
	l := moresync.NewLatch(1)
	var ret *vm.MessageReceipt
	err := plumbing.MessageWait(ctx, msgCid, func(_ *block.Block, _ *types.SignedMessage, rcpt *vm.MessageReceipt) error {
		ret = rcpt
		l.Done()
		return nil
	})
	if err != nil {
		return nil, err
	}
	l.Wait()
	return ret, nil
}

package fast

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
)

// ShowHeader runs the `show header` command against the filecoin process
func (f *Filecoin) ShowHeader(ctx context.Context, ref cid.Cid) (*block.Block, error) {
	var out block.Block

	sRef := ref.String()

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "show", "header", sRef); err != nil {
		return nil, err
	}

	return &out, nil
}

// ShowMessages runs the `show messages` command against the filecoin process
func (f *Filecoin) ShowMessages(ctx context.Context, ref cid.Cid) ([]*types.SignedMessage, error) {
	var out []*types.SignedMessage

	sRef := ref.String()

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "show", "messages", sRef); err != nil {
		return nil, err
	}

	return out, nil
}

// ShowReceipts runs the `show receipts` command against the filecoin process
func (f *Filecoin) ShowReceipts(ctx context.Context, ref cid.Cid) ([]vm.MessageReceipt, error) {
	var out []vm.MessageReceipt

	sRef := ref.String()

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "show", "receipts", sRef); err != nil {
		return nil, err
	}

	return out, nil
}

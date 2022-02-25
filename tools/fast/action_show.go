package fast

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
)

// ShowHeader runs the `show header` command against the filecoin process
func (f *Filecoin) ShowHeader(ctx context.Context, ref cid.Cid) (*types.BlockHeader, error) {
	var out types.BlockHeader

	sRef := ref.String()

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "venus", "show", "header", sRef); err != nil {
		return nil, err
	}

	return &out, nil
}

// ShowMessages runs the `show messages` command against the filecoin process
func (f *Filecoin) ShowMessages(ctx context.Context, ref cid.Cid) ([]*types.SignedMessage, error) {
	var out []*types.SignedMessage

	sRef := ref.String()

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "venus", "show", "messages", sRef); err != nil {
		return nil, err
	}

	return out, nil
}

// ShowReceipts runs the `show receipts` command against the filecoin process
func (f *Filecoin) ShowReceipts(ctx context.Context, ref cid.Cid) ([]types.MessageReceipt, error) {
	var out []types.MessageReceipt

	sRef := ref.String()

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "venus", "show", "receipts", sRef); err != nil {
		return nil, err
	}

	return out, nil
}

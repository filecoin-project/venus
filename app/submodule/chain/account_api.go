package chain

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/block"
	xerrors "github.com/pkg/errors"
)

func (chainAPI *ChainAPI) StateAccountKey(ctx context.Context, addr address.Address, tsk block.TipSetKey) (address.Address, error) {
	view, err := chainAPI.chain.State.StateView(tsk)
	if err != nil {
		return address.Undef, xerrors.Errorf("loading tipset %s: %w", tsk, err)
	}

	return view.ResolveToKeyAddr(ctx, addr)
}

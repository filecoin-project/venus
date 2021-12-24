package chain

import (
	"context"

	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	types "github.com/filecoin-project/venus/venus-shared/chain"

	"github.com/filecoin-project/go-address"
	"golang.org/x/xerrors"
)

var _ v1api.IAccount = &accountAPI{}

type accountAPI struct {
	chain *ChainSubmodule
}

//NewAccountAPI create a new account api
func NewAccountAPI(chain *ChainSubmodule) v1api.IAccount {
	return &accountAPI{chain: chain}
}

// StateAccountKey returns the public key address of the given ID address
func (accountAPI *accountAPI) StateAccountKey(ctx context.Context, addr address.Address, tsk types.TipSetKey) (address.Address, error) {
	ts, err := accountAPI.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return address.Undef, xerrors.Errorf("loading tipset %s: %w", tsk, err)
	}
	return accountAPI.chain.Stmgr.ResolveToKeyAddress(ctx, addr, ts)
}

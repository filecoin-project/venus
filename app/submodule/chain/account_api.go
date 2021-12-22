package chain

import (
	"context"

	"github.com/filecoin-project/venus/app/client/apiface"
	types "github.com/filecoin-project/venus/venus-shared/chain"

	"github.com/filecoin-project/go-address"
	"golang.org/x/xerrors"
)

var _ apiface.IAccount = &accountAPI{}

type accountAPI struct {
	chain *ChainSubmodule
}

//NewAccountAPI create a new account api
func NewAccountAPI(chain *ChainSubmodule) apiface.IAccount {
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

package paych

import (
	"context"

	"github.com/ipfs/go-datastore"

	v0api2 "github.com/filecoin-project/venus/app/submodule/paych/v0api"
	"github.com/filecoin-project/venus/pkg/paychmgr"
	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

//PaychSubmodule support paych related functions, including paych construction, extraction, query and other functions
type PaychSubmodule struct { //nolint
	pmgr *paychmgr.Manager
}

// PaychSubmodule enhances the `Node` with paych capabilities.
func NewPaychSubmodule(ctx context.Context, ds datastore.Batching, params *paychmgr.ManagerParams) (*PaychSubmodule, error) {
	mgr, err := paychmgr.NewManager(ctx, ds, params)
	return &PaychSubmodule{mgr}, err
}

func (ps *PaychSubmodule) Start(ctx context.Context) error {
	return ps.pmgr.Start(ctx)
}

func (ps *PaychSubmodule) Stop() {
	ps.pmgr.Stop()
}

//API create a new paych implement
func (ps *PaychSubmodule) API() v1api.IPaychan {
	return NewPaychAPI(ps.pmgr)
}

func (ps *PaychSubmodule) V0API() v0api.IPaychan {
	return &v0api2.WrapperV1IPaych{IPaychan: ps.API()}
}

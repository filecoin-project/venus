package paych

import (
	"context"

	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/filecoin-project/venus/app/client/apiface/v0api"
	"github.com/filecoin-project/venus/pkg/paychmgr"
	"github.com/ipfs/go-datastore"
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

func (ps *PaychSubmodule) Start() error {
	return ps.pmgr.Start()
}

func (ps *PaychSubmodule) Stop() {
	ps.pmgr.Stop()
}

//API create a new paych implement
func (ps *PaychSubmodule) API() apiface.IPaychan {
	return NewPaychAPI(ps.pmgr)
}

func (ps *PaychSubmodule) V0API() v0api.IPaychan {
	return NewPaychAPI(ps.pmgr)
}

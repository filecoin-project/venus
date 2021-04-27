package paych

import (
	"context"
	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/pkg/paychmgr"
)

//PaychSubmodule support paych related functions, including paych construction, extraction, query and other functions
type PaychSubmodule struct { //nolint
	pmgr *paychmgr.Manager
}

// PaychSubmodule enhances the `Node` with paych capabilities.
func NewPaychSubmodule(ctx context.Context, params *paychmgr.ManagerParams) *PaychSubmodule {
	mgr := paychmgr.NewManager(ctx, params)
	return &PaychSubmodule{mgr}
}

func (ps *PaychSubmodule) Start() error {
	return ps.pmgr.Start()
}

func (ps *PaychSubmodule) Stop() {
	ps.pmgr.Stop()
}

//API create a new paych implement
func (ps *PaychSubmodule) API() apiface.IPaychan {
	return newPaychAPI(ps.pmgr)
}

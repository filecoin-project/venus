package paych

import (
	"context"
	"github.com/filecoin-project/venus/pkg/paychmgr"
)

type PaychSubmodule struct {//nolint
	pmgr *paychmgr.Manager
}

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

func (ps *PaychSubmodule) API() IPaychan {
	return newPaychAPI(ps.pmgr)
}

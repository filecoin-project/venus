package f3

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-f3/certs"
	"github.com/filecoin-project/go-f3/gpbft"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var _ v1.IF3 = &f3API{}

type f3API struct {
	f3module *F3Submodule
}

var ErrF3Disabled = errors.New("f3 is disabled")

func (f3api *f3API) F3Participate(ctx context.Context,
	minerAddr address.Address,
	newLeaseExpiration time.Time,
	oldLeaseExpiration time.Time,
) (bool, error) {
	if f3api.f3module.F3 == nil {
		log.Infof("F3Participate called for %v, F3 is disabled", minerAddr)
		return false, ErrF3Disabled
	}

	if leaseDuration := time.Until(newLeaseExpiration); leaseDuration > 5*time.Minute {
		return false, fmt.Errorf("F3 participation lease too long: %v > 5 min", leaseDuration)
	} else if leaseDuration < 0 {
		return false, fmt.Errorf("F3 participation lease is in the past: %d < 0", leaseDuration)
	}

	minerID, err := address.IDFromAddress(minerAddr)
	if err != nil {
		return false, fmt.Errorf("miner address is not of ID type: %v: %w", minerID, err)
	}

	return f3api.f3module.F3.Participate(ctx, minerID, newLeaseExpiration, oldLeaseExpiration), nil
}

func (f3api *f3API) F3GetCertificate(ctx context.Context, instance uint64) (*certs.FinalityCertificate, error) {
	if f3api.f3module.F3 == nil {
		return nil, ErrF3Disabled
	}
	return f3api.f3module.F3.GetCert(ctx, instance)
}

func (f3api *f3API) F3GetLatestCertificate(ctx context.Context) (*certs.FinalityCertificate, error) {
	if f3api.f3module.F3 == nil {
		return nil, ErrF3Disabled
	}
	return f3api.f3module.F3.GetLatestCert(ctx)
}

func (f3api *f3API) F3GetECPowerTable(ctx context.Context, tsk types.TipSetKey) (gpbft.PowerEntries, error) {
	if f3api.f3module.F3 == nil {
		return nil, ErrF3Disabled
	}
	return f3api.f3module.F3.GetPowerTable(ctx, tsk)
}

func (f3api *f3API) F3GetF3PowerTable(ctx context.Context, tsk types.TipSetKey) (gpbft.PowerEntries, error) {
	if f3api.f3module.F3 == nil {
		return nil, ErrF3Disabled
	}
	return f3api.f3module.F3.GetF3PowerTable(ctx, tsk)
}

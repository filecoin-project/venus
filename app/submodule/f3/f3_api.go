package f3

import (
	"context"
	"errors"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-f3/certs"
	"github.com/filecoin-project/go-f3/gpbft"
	"github.com/filecoin-project/go-f3/manifest"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var _ v1.IF3 = &f3API{}

type f3API struct {
	f3module *F3Submodule
}

var ErrF3Disabled = errors.New("f3 is disabled")

func (f3api *f3API) F3GetOrRenewParticipationTicket(ctx context.Context, miner address.Address, previous types.F3ParticipationTicket, instances uint64) (types.F3ParticipationTicket, error) {
	if f3api.f3module.F3 == nil {
		log.Infof("F3GetParticipationTicket called for %v, F3 is disabled", miner)
		return nil, types.ErrF3Disabled
	}
	minerID, err := address.IDFromAddress(miner)
	if err != nil {
		return nil, fmt.Errorf("miner address is not of ID type: %v: %w", miner, err)
	}
	return f3api.f3module.F3.GetOrRenewParticipationTicket(ctx, minerID, previous, instances)
}

func (f3api *f3API) F3Participate(ctx context.Context, ticket types.F3ParticipationTicket) (types.F3ParticipationLease, error) {
	if f3api.f3module.F3 == nil {
		log.Infof("F3Participate called, F3 is disabled")
		return types.F3ParticipationLease{}, types.ErrF3Disabled
	}
	return f3api.f3module.F3.Participate(ctx, ticket)
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

func (f3api *f3API) F3GetManifest(ctx context.Context) (*manifest.Manifest, error) {
	if f3api.f3module.F3 == nil {
		return nil, ErrF3Disabled
	}
	return f3api.f3module.F3.GetManifest(ctx)
}

func (f3api *f3API) F3IsRunning(_ctx context.Context) (bool, error) {
	if f3api.f3module.F3 == nil {
		return false, ErrF3Disabled
	}
	return f3api.f3module.F3.IsRunning(), nil
}

func (f3api *f3API) F3GetProgress(context.Context) (gpbft.InstanceProgress, error) {
	if f3api.f3module.F3 == nil {
		return gpbft.InstanceProgress{}, types.ErrF3Disabled
	}
	return f3api.f3module.F3.Progress(), nil
}

func (f3api *f3API) F3ListParticipants(ctx context.Context) ([]types.F3Participant, error) {
	if f3api.f3module.F3 == nil {
		return nil, types.ErrF3Disabled
	}
	return f3api.f3module.F3.ListParticipants(), nil
}

package f3

import (
	"context"
	"errors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-f3/certs"
	"github.com/filecoin-project/venus/venus-shared/api/f3"
	"golang.org/x/xerrors"
)

var _ f3.F3 = &f3API{}

type f3API struct {
	f3module *F3Submodule
}

var ErrF3Disabled = errors.New("f3 is disabled")

func (f3api *f3API) F3Participate(ctx context.Context, miner address.Address) (<-chan string, error) {
	if f3api.f3module.F3 == nil {
		log.Infof("F3Participate called for %v, F3 is disabled", miner)
		return nil, ErrF3Disabled
	}

	// Make channel with some buffer to avoid blocking under higher load.
	errCh := make(chan string, 4)
	log.Infof("starting F3 participation for %v", miner)

	actorID, err := address.IDFromAddress(miner)
	if err != nil {
		return nil, xerrors.Errorf("miner address in F3Participate not of ID type: %w", err)
	}

	// Participate takes control of closing the channel
	go f3api.f3module.F3.Participate(ctx, actorID, errCh)
	return errCh, nil
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

package drand

import (
	"context"
	"github.com/filecoin-project/go-filecoin/internal/pkg/drand"
)

type API struct {
	drand drand.IFace
}

// New creates a new API
func New(drand drand.IFace) *API {
	return &API{
		drand: drand,
	}
}

// GetEntry retrieves an entry from the drand server
func (api *API) GetEntry(ctx context.Context, round drand.Round) (*drand.Entry, error) {
	return api.drand.ReadEntry(ctx, round)
}

// VerifyEntry verifies that child is a valid entry if its parent is.
func (api *API) VerifyEntry(parent, child *drand.Entry) (bool, error) {
	return api.drand.VerifyEntry(parent, child)
}

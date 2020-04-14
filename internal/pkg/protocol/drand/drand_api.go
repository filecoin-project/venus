package drand

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/drand"
)

type Config interface {
	ConfigSet(dottedPath string, paramJSON string) error
}

type API struct {
	drand  drand.IFace
	config Config
}

// New creates a new API
func New(drand drand.IFace, config Config) *API {
	return &API{
		drand:  drand,
		config: config,
	}
}

// Configure fetches group configuration from a drand server.
// It runs through the list of addrs trying each one to fetch the group config.
// Once the group is retrieved, the node's group key will be set in config.
// If overrideGroupAddrs is true, the given set of addresses will be set as the drand nodes.
// Otherwise drand address config will be set from the retrieved group info. The
// override is useful when the the drand server is behind NAT.
// This method assumes all drand nodes are secure or that all of them are not. This
// mis-models the drand config, but is unlikely to be false in practice.
func (api *API) Configure(addrs []string, secure bool, overrideGroupAddrs bool) error {
	groupAddrs, keyCoeffs, err := api.drand.FetchGroupConfig(addrs, secure, overrideGroupAddrs)
	if err != nil {
		return errors.Wrapf(err, "Could not retrieve drand group from %+v", addrs)
	}

	jsonCoeffs, err := json.Marshal(keyCoeffs)
	if err != nil {
		return errors.New("Could not convert coefficients to json")
	}

	err = api.config.ConfigSet("drand.distKey", string(jsonCoeffs))
	if err != nil {
		return errors.Wrap(err, "Could not set dist key in config")
	}

	if overrideGroupAddrs {
		groupAddrs = addrs
	}

	jsonAddrs, err := json.Marshal(groupAddrs)
	if err != nil {
		return errors.New("Could not convert addresses to json")
	}

	err = api.config.ConfigSet("drand.addresses", string(jsonAddrs))
	if err != nil {
		return errors.Wrap(err, "Could not set drand addresses in config")
	}

	jsonSecure, err := json.Marshal(secure)
	if err != nil {
		return errors.New("Could not convert secure to json")
	}

	err = api.config.ConfigSet("drand.secure", string(jsonSecure))
	if err != nil {
		return errors.Wrap(err, "Could not set drand secure in config")
	}

	return nil
}

// GetEntry retrieves an entry from the drand server
func (api *API) GetEntry(ctx context.Context, round drand.Round) (*drand.Entry, error) {
	return api.drand.ReadEntry(ctx, round)
}

// GetEntry retrieves a entry from the drand server
func (api *API) VerifyEntry(parent, child *drand.Entry) (bool, error) {
	return api.drand.VerifyEntry(parent, child)
}

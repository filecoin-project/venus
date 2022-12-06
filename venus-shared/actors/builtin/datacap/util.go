// FETCHED FROM LOTUS: builtin/datacap/util.go

package datacap

import (
	"fmt"

	"github.com/multiformats/go-varint"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/builtin/v9/verifreg"

	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
)

// taking this as a function instead of asking the caller to call it helps reduce some of the error
// checking boilerplate.
//
// "go made me do it"
type rootFunc func() (adt.Map, error)

func getDataCap(store adt.Store, ver actors.Version, root rootFunc, addr address.Address) (bool, abi.StoragePower, error) {
	if addr.Protocol() != address.ID {
		return false, big.Zero(), fmt.Errorf("can only look up ID addresses")
	}
	vh, err := root()
	if err != nil {
		return false, big.Zero(), fmt.Errorf("loading datacap actor: %w", err)
	}

	var dcap abi.StoragePower
	if found, err := vh.Get(abi.IdAddrKey(addr), &dcap); err != nil {
		return false, big.Zero(), fmt.Errorf("looking up addr: %w", err)
	} else if !found {
		return false, big.Zero(), nil
	}

	return true, big.Div(dcap, verifreg.DataCapGranularity), nil
}

func forEachClient(store adt.Store, ver actors.Version, root rootFunc, cb func(addr address.Address, dcap abi.StoragePower) error) error {
	vh, err := root()
	if err != nil {
		return fmt.Errorf("loading verified clients: %w", err)
	}
	var dcap abi.StoragePower
	return vh.ForEach(&dcap, func(key string) error {
		id, n, err := varint.FromUvarint([]byte(key))
		if n != len([]byte(key)) {
			return fmt.Errorf("could not get varint from address string")
		}
		if err != nil {
			return err
		}

		a, err := address.NewIDAddress(id)
		if err != nil {
			return fmt.Errorf("creating ID address from actor ID: %w", err)
		}

		return cb(a, big.Div(dcap, verifreg.DataCapGranularity))
	})
}

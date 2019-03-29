package commands

import (
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
)

func optionalAddr(o interface{}) (ret address.Address, err error) {
	if o != nil {
		ret, err = address.NewFromString(o.(string))
		if err != nil {
			err = errors.Wrap(err, "invalid from address")
		}
	}
	return
}

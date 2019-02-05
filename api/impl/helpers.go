package impl

import (
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/porcelain"
)

func setDefaultFromAddr(fromAddr *address.Address, nd *node.Node) error {
	if *fromAddr == (address.Address{}) {
		ret, err := nd.PorcelainAPI.GetAndMaybeSetDefaultSenderAddress()
		if (err != nil && err == porcelain.ErrNoDefaultFromAddress) || ret == (address.Address{}) {
			return ErrCouldNotDefaultFromAddress
		}
		*fromAddr = ret
	}

	return nil
}

package impl

import (
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/node"
)

func setDefaultFromAddr(fromAddr *address.Address, nd *node.Node) error {
	if *fromAddr == (address.Address{}) {
		ret, err := nd.DefaultSenderAddress()
		if (err != nil && err == node.ErrNoDefaultMessageFromAddress) || ret == (address.Address{}) {
			return ErrCouldNotDefaultFromAddress
		}
		*fromAddr = ret
	}

	return nil
}

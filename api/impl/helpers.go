package impl

import (
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"
)

func setDefaultFromAddr(fromAddr *types.Address, nd *node.Node) error {
	if *fromAddr == (types.Address{}) {
		ret, err := nd.DefaultSenderAddress()
		if (err != nil && err == node.ErrNoDefaultMessageFromAddress) || ret == (types.Address{}) {
			return ErrCouldNotDefaultFromAddress
		}
		*fromAddr = ret
	}

	return nil
}

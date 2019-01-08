package impl

import (
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/plumbing/msg"
)

func setDefaultFromAddr(fromAddr *address.Address, nd *node.Node) error {
	if *fromAddr == (address.Address{}) {
		ret, err := msg.GetAndMaybeSetDefaultSenderAddress(nd.Repo, nd.Wallet)
		if (err != nil && err == msg.ErrNoDefaultFromAddress) || ret == (address.Address{}) {
			return ErrCouldNotDefaultFromAddress
		}
		*fromAddr = ret
	}

	return nil
}

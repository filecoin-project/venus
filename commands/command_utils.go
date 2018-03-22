package commands

import (
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"
)

func addressWithDefault(rawAddr interface{}, n *node.Node) (types.Address, error) {
	stringAddr, _ := rawAddr.(string)
	var addr types.Address
	var err error
	if stringAddr != "" {
		addr, err = types.NewAddressFromString(stringAddr)
		if err != nil {
			return types.Address{}, err
		}
	} else {
		addr, err = n.Wallet.GetDefaultAddress()
		if err != nil {
			return types.Address{}, err
		}
	}

	return addr, nil
}

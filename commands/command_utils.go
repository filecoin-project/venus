package commands

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"
)

func addressWithDefault(rawAddr interface{}, n *node.Node) (types.Address, error) {
	stringAddr, ok := rawAddr.(string)
	if !ok {
		stringAddr = ""
	}
	var addr types.Address
	var err error
	if stringAddr != "" {
		addr, err = types.ParseAddress(stringAddr)
		if err != nil {
			return "", err
		}
	} else {
		addrs := n.Wallet.GetAddresses()
		if len(addrs) == 0 {
			return "", fmt.Errorf("no addresses in local wallet")
		}
		addr = addrs[0]
	}

	return addr, nil
}

func toBigInt(rawString string) (*big.Int, error) {
	rawInt, err := strconv.ParseInt(rawString, 10, 64)
	if err != nil {
		return nil, err
	}
	return big.NewInt(rawInt), nil
}

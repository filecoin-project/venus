package cmd

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/pkg/constants"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

func reqConfidence(req *cmds.Request) uint64 {
	confidence, ok := req.Options["confidence"]
	if ok {
		return confidence.(uint64)
	}
	return 0
}

func reqFromWithDefault(req *cmds.Request, env cmds.Environment) (address.Address, error) {
	f, ok := req.Options["from"]
	if ok {
		from, err := address.NewFromString(f.(string))
		if err != nil {
			return address.Undef, err
		}
		return from, nil
	}
	defaddr, err := env.(*node.Env).WalletAPI.WalletDefaultAddress(req.Context)
	if err != nil {
		return address.Undef, err
	}
	return defaddr, nil
}

func reqBoolOption(req *cmds.Request, cmd string) bool {
	tmp, ok := req.Options[cmd]
	if ok {
		return tmp.(bool)
	}
	return false
}
func reqUint64Option(req *cmds.Request, cmd string) uint64 {
	tmp, ok := req.Options[cmd]
	if ok {
		return tmp.(uint64)
	}
	return 0
}
func reqStringOption(req *cmds.Request, cmd string) string {
	tmp, ok := req.Options[cmd]
	if ok {
		return tmp.(string)
	}
	return constants.StringEmpty
}
func reqChainEpochOption(req *cmds.Request, cmd string) abi.ChainEpoch {
	v, ok := req.Options[cmd]
	if ok {
		return abi.ChainEpoch(v.(int64))
	}
	return 0
}

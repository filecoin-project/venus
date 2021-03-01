package cmd

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/app/node"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

func reqConfidence(req *cmds.Request) abi.ChainEpoch {
	confidence, ok := req.Options["confidence"]
	if ok {
		return confidence.(abi.ChainEpoch)
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
	defaddr, err := env.(*node.Env).WalletAPI.WalletDefaultAddress()
	if err != nil {
		return address.Undef, err
	}
	return defaddr, nil
}

func reqBoolOption(req *cmds.Request, cmd string) bool {
	bl, ok := req.Options[cmd]
	if ok {
		return bl.(bool)
	}
	return false
}
func reqChainEpochOption(req *cmds.Request, cmd string) abi.ChainEpoch {
	v, ok := req.Options[cmd]
	if ok {
		return v.(abi.ChainEpoch)
	}
	return 0
}

req.Arguments[
env.(*node.Env)
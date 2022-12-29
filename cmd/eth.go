package cmd

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"

	cmds "github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/venus/app/node"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var ethCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Query eth contract state",
	},
	Subcommands: map[string]*cmds.Command{
		"stat":             ethGetInfoCmd,
		"call":             ethCallSimulateCmd,
		"contract-address": ethGetContractAddressCmd,
	},
}

var ethGetInfoCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Print eth/filecoin addrs and code cid",
	},
	Options: []cmds.Option{
		cmds.StringOption("filAddr", "Filecoin address"),
		cmds.StringOption("ethAddr", "Ethereum address"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		filAddr, _ := req.Options["filAddr"].(string)
		ethAddr, _ := req.Options["ethAddr"].(string)

		var faddr address.Address
		var eaddr types.EthAddress

		ctx := req.Context
		chainAPI := env.(*node.Env).ChainAPI

		if filAddr != "" {
			addr, err := address.NewFromString(filAddr)
			if err != nil {
				return err
			}
			eaddr, faddr, err = ethAddrFromFilecoinAddress(ctx, addr, chainAPI)
			if err != nil {
				return err
			}
		} else if ethAddr != "" {
			eaddr, err := types.EthAddressFromHex(ethAddr)
			if err != nil {
				return err
			}
			faddr, err = eaddr.ToFilecoinAddress()
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("neither filAddr nor ethAddr specified")
		}

		actor, err := chainAPI.StateGetActor(ctx, faddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)

		writer.Println("Filecoin address: ", faddr)
		writer.Println("Eth address: ", eaddr)
		writer.Println("Code cid: ", actor.Code.String())

		return re.Emit(buf)
	},
}

var ethCallSimulateCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Simulate an eth contract call",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("from", true, false, ""),
		cmds.StringArg("to", true, false, ""),
		cmds.StringArg("params", true, false, ""),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 3 {
			return fmt.Errorf("incorrect number of arguments, got %d", len(req.Arguments))
		}

		fromEthAddr, err := types.EthAddressFromHex(req.Arguments[0])
		if err != nil {
			return err
		}

		toEthAddr, err := types.EthAddressFromHex(req.Arguments[1])
		if err != nil {
			return err
		}

		params, err := hex.DecodeString(req.Arguments[2])
		if err != nil {
			return err
		}

		ctx := req.Context

		res, err := env.(*node.Env).EthAPI.EthCall(ctx, types.EthCall{
			From: &fromEthAddr,
			To:   &toEthAddr,
			Data: params,
		}, "")
		if err != nil {
			_ = re.Emit(fmt.Sprintln("Eth call fails, return val: ", res))
			return err
		}

		return re.Emit(fmt.Sprintln("Result: ", res))
	},
}

var ethGetContractAddressCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Generate contract address from smart contract code",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("senderEthAddr", true, false, ""),
		cmds.StringArg("salt", true, false, ""),
		cmds.StringArg("contractHexPath", true, false, ""),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 3 {
			return fmt.Errorf("incorrect number of arguments, got %d", len(req.Arguments))
		}

		sender, err := types.EthAddressFromHex(req.Arguments[0])
		if err != nil {
			return err
		}

		salt, err := hex.DecodeString(req.Arguments[1])
		if err != nil {
			return fmt.Errorf("could not decode salt: %v", err)
		}
		if len(salt) > 32 {
			return fmt.Errorf("len of salt bytes greater than 32")
		}
		var fsalt [32]byte
		copy(fsalt[:], salt[:])

		contractBin := req.Arguments[2]
		if err != nil {
			return err
		}
		contractHex, err := os.ReadFile(contractBin)
		if err != nil {

			return err
		}
		contract, err := hex.DecodeString(string(contractHex))
		if err != nil {
			return fmt.Errorf("could not decode contract file: %v", err)
		}

		contractAddr, err := types.GetContractEthAddressFromCode(sender, fsalt, contract)
		if err != nil {
			return err
		}

		return printOneString(re, fmt.Sprint("contract Eth address: ", contractAddr))
	},
}

func ethAddrFromFilecoinAddress(ctx context.Context, addr address.Address, fnapi v1api.IChain) (types.EthAddress, address.Address, error) {
	var faddr address.Address
	var err error

	switch addr.Protocol() {
	case address.BLS, address.SECP256K1:
		faddr, err = fnapi.StateLookupID(ctx, addr, types.EmptyTSK)
		if err != nil {
			return types.EthAddress{}, addr, err
		}
	case address.Actor, address.ID:
		faddr, err = fnapi.StateLookupID(ctx, addr, types.EmptyTSK)
		if err != nil {
			return types.EthAddress{}, addr, err
		}
		fAct, err := fnapi.StateGetActor(ctx, faddr, types.EmptyTSK)
		if err != nil {
			return types.EthAddress{}, addr, err
		}
		if fAct.Address != nil && (*fAct.Address).Protocol() == address.Delegated {
			faddr = *fAct.Address
		}
	case address.Delegated:
		faddr = addr
	default:
		return types.EthAddress{}, addr, fmt.Errorf("filecoin address doesn't match known protocols")
	}

	ethAddr, err := types.EthAddressFromFilecoinAddress(faddr)
	if err != nil {
		return types.EthAddress{}, addr, err
	}

	return ethAddr, faddr, nil
}

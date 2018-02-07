package commands

import (
	"fmt"
	"io"

	cmds "gx/ipfs/QmWGgKRz5S24SqaAapF5PPCfYfLT7MexJZewN5M82CQTzs/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/types"
)

var walletCmd = &cmds.Command{
	Subcommands: map[string]*cmds.Command{
		"addrs": addrsCmd,
	},
}

var addrsCmd = &cmds.Command{
	Subcommands: map[string]*cmds.Command{
		"new":  addrsNewCmd,
		"list": addrsListCmd,
	},
}

var addrsNewCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fcn := GetNode(env)
		re.Emit(fcn.Wallet.NewAddress())
	},
	Type: types.Address(""),
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, a *types.Address) error {
			_, err := fmt.Fprintln(w, a.String())
			return err
		}),
	},
}

var addrsListCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fcn := GetNode(env)
		re.Emit(fcn.Wallet.GetAddresses())
	},
	Type: []types.Address{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, addrs *[]types.Address) error {
			for _, a := range *addrs {
				_, err := fmt.Fprintln(w, a.String())
				if err != nil {
					return err
				}
			}
			return nil
		}),
	},
}

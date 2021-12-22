package cmd

import (
	"github.com/filecoin-project/venus/app/node"
	apitypes "github.com/filecoin-project/venus/venus-shared/api/chain"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var protocolCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show protocol parameter details",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		params, err := env.(*node.Env).ChainAPI.ProtocolParameters(req.Context)
		if err != nil {
			return err
		}
		return re.Emit(params)
	},
	Type: apitypes.ProtocolParams{},
}

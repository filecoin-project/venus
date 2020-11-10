package commands

import (
	cmds "github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/venus/internal/app/go-filecoin/porcelain"
)

var protocolCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show protocol parameter details",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		params, err := GetPorcelainAPI(env).ProtocolParameters(req.Context)
		if err != nil {
			return err
		}
		return re.Emit(params)
	},
	Type: porcelain.ProtocolParams{},
}

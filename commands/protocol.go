package commands

import (
	"fmt"
	"io"

	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/porcelain"
)

var protocolCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show protocol parameter details",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		params, err := GetPorcelainAPI(env).ProtocolParameters()
		if err != nil {
			return err
		}
		return re.Emit(params)
	},
	Type: porcelain.ProtocolParams{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, pp *porcelain.ProtocolParams) error {
			_, err := fmt.Fprintf(w, "Auto-Seal Interval: %d seconds\n", pp.AutoSealInterval)
			return err
		}),
	},
}

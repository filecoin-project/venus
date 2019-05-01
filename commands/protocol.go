package commands

import (
	"fmt"
	"io"

	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/types"
)

var protocolCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show protocol parameter details",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		params, err := GetPorcelainAPI(env).ProtocolParameters(env.Context())
		if err != nil {
			return err
		}
		return re.Emit(params)
	},
	Type: porcelain.ProtocolParams{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, pp *porcelain.ProtocolParams) error {
			_, err := fmt.Fprintf(w, "Auto-Seal Interval: %d seconds\nSector Sizes:\n", pp.AutoSealInterval)
			if err != nil {
				return err
			}

			sectorSize := types.OneKiBSectorSize
			if pp.ProofsMode == types.LiveProofsMode {
				sectorSize = types.TwoHundredFiftySixMiBSectorSize
			}

			_, err = fmt.Fprintf(w, "\t%d bytes\n", sectorSize.Uint64())
			if err != nil {
				return err
			}

			return nil
		}),
	},
}

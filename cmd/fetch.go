package cmd

import (
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/pkg/errors"

	paramfetch "github.com/filecoin-project/go-paramfetch"

	"github.com/filecoin-project/venus/fixtures/assets"
)

var fetchCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "fetch paramsters",
	},
	Options: []cmds.Option{
		cmds.Uint64Option(Size, "size to fetch"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		// highest precedence is cmd line flag.
		if size, ok := req.Options[Size].(uint64); ok {
			ps, err := assets.GetProofParams()
			if err != nil {
				return err
			}

			srs, err := assets.GetSrs()
			if err != nil {
				return err
			}

			if err := paramfetch.GetParams(req.Context, ps, srs, size); err != nil {
				return errors.Wrapf(err, "fetching proof parameters: %v", err)
			}
			return nil
		}
		return errors.New("uncorrect parameters")
	},
}

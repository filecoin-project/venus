package commands

import (
	paramfetch "github.com/filecoin-project/go-paramfetch"
	"github.com/filecoin-project/venus/fixtures"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/pkg/errors"
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
			if err := paramfetch.GetParams(req.Context, fixtures.ParametersJSON(), size); err != nil {
				return errors.Wrapf(err, "fetching proof parameters: %v", err)
			}
			return nil
		}
		return errors.New("uncorrect parameters")
	},
}

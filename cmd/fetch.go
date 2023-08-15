package cmd

import (
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/pkg/errors"

	"github.com/docker/go-units"
	paramfetch "github.com/filecoin-project/go-paramfetch"
	"github.com/filecoin-project/venus/fixtures/assets"
)

var fetchCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "fetch paramsters",
	},
	Options: []cmds.Option{
		cmds.StringOption(Size, "size to fetch, eg. 2Kib,512Mib,32Gib,64Gib"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		// highest precedence is cmd line flag.
		if sizeStr, ok := req.Options[Size].(string); ok {
			ps, err := assets.GetProofParams()
			if err != nil {
				return err
			}

			srs, err := assets.GetSrs()
			if err != nil {
				return err
			}

			size, err := units.RAMInBytes(sizeStr)
			if err != nil {
				return err
			}

			if err := paramfetch.GetParams(req.Context, ps, srs, uint64(size)); err != nil {
				return errors.Wrapf(err, "fetching proof parameters: %v", err)
			}
			return nil
		}
		return errors.New("uncorrect parameters")
	},
}

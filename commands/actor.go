package commands

import (
	"encoding/json"
	"io"

	"github.com/filecoin-project/go-filecoin/api"

	"gx/ipfs/QmVTmXZC2yE38SDKRihn96LXX6KwBWgzAg8aCDZaMirCHm/go-ipfs-cmds"
	"gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"
)

var actorCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with actors",
	},
	Subcommands: map[string]*cmds.Command{
		"ls": actorLsCmd,
	},
}

var actorLsCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		actors, err := GetAPI(env).Actor().Ls(req.Context)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal) // nolint: errcheck
			return
		}

		for _, actor := range actors {
			re.Emit(actor) // nolint: errcheck
		}
	},
	Type: &api.ActorView{},
	Encoders: cmds.EncoderMap{
		cmds.JSON: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, a *api.ActorView) error {
			marshaled, err := json.Marshal(a)
			if err != nil {
				return err
			}
			_, err = w.Write(marshaled)
			if err != nil {
				return err
			}
			_, err = w.Write([]byte("\n"))
			return err
		}),
	},
}

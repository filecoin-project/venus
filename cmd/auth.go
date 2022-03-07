package cmd

import (
	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/venus-shared/api/permission"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"golang.org/x/xerrors"
)

var authCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Usage: "auth commands",
	},
	Subcommands: map[string]*cmds.Command{
		"create-token": authCreateAdminToken,
	},
}

var authCreateAdminToken = &cmds.Command{
	Helptext: cmds.HelpText{
		Usage: "create token",
	},
	Options: []cmds.Option{
		cmds.StringOption("perm"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context

		perm, ok := req.Options["perm"].(string)
		if !ok {
			return xerrors.New("--perm flag not set")
		}

		idx := 0
		for i, p := range permission.AllPermissions {
			if perm == p {
				idx = i + 1
			}
		}

		if idx == 0 {
			return xerrors.Errorf("--perm flag has to be one of: %v", permission.AllPermissions)
		}

		// slice on [:idx] so for example: 'sign' gives you [read, write, sign]
		token, err := env.(*node.Env).AuthAPI.AuthNew(ctx, permission.AllPermissions[:idx])
		if err != nil {
			return err
		}

		return re.Emit(string(token))
	},
}

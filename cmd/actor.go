package cmd

import (
	"encoding/json"
	"github.com/filecoin-project/venus/app/node"
	"io"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/venus/pkg/types"
)

// ActorView represents a generic way to represent details about any actor to the user.
type ActorView struct {
	Address string        `json:"address"`
	Code    cid.Cid       `json:"code,omitempty"`
	Nonce   uint64        `json:"nonce"`
	Balance types.AttoFIL `json:"balance"`
	Head    cid.Cid       `json:"head,omitempty"`
}

var actorCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with actors. Actors are built-in smart contracts.",
	},
	Subcommands: map[string]*cmds.Command{
		"ls": actorLsCmd,
	},
}

var actorLsCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		results, err := env.(*node.Env).ChainAPI.ListActor(req.Context)
		if err != nil {
			return err
		}

		for addr, actor := range results {
			output := makeActorView(actor, addr)
			if err := re.Emit(output); err != nil {
				return err
			}
		}
		return nil
	},
	Type: &ActorView{},
	Encoders: cmds.EncoderMap{
		cmds.JSON: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, a *ActorView) error {
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

func makeActorView(act *types.Actor, addr address.Address) *ActorView {
	return &ActorView{
		Address: addr.String(),
		Code:    act.Code.Cid,
		Nonce:   act.Nonce,
		Balance: act.Balance,
		Head:    act.Head.Cid,
	}
}

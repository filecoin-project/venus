// Package commands implements the command to print the blockchain.
package commands

import (
	"encoding/json"
	"io"

	cmds "gx/ipfs/Qmc5paX4ECBARnAKkcAmUYHBGor228Tkfxeya3Nu2KRL46/go-ipfs-cmds"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/types"
)

var chainCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "dump full block chain",
	},
	Run:  chainRun,
	Type: types.Block{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, val *types.Block) error {
			marshaled, err := json.MarshalIndent(val, "", "\t")
			if err != nil {
				return err
			}
			marshaled = append(marshaled, byte('\n'))
			_, err = w.Write(marshaled)
			return err
		}),
	},
}

func chainRun(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
	n := GetNode(env)
	if n != nil && n.Block != nil {
		if err := re.Emit(n.Block); err != nil {
			panic(err)
		}
		// TODO Actually walk the chain. The actual thing that should happen could be
		// that Block implements https://golang.org/pkg/fmt/#Formatter or perhaps
		// StateManager/Blockchain provides this service.
	}
}

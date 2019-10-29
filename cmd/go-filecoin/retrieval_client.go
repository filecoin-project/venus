package commands

import (
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

var retrievalClientCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage retrieval client operations",
	},
	Subcommands: map[string]*cmds.Command{
		"retrieve-piece": clientRetrievePieceCmd,
	},
}

var clientRetrievePieceCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Read out piece data stored by a miner on the network",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "Retrieval miner actor address"),
		cmdkit.StringArg("cid", true, false, "Content identifier of piece to read"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		minerAddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		pieceCID, err := cid.Decode(req.Arguments[1])
		if err != nil {
			return err
		}

		mpid, err := GetPorcelainAPI(env).MinerGetPeerID(req.Context, minerAddr)
		if err != nil {
			return err
		}

		readCloser, err := GetRetrievalAPI(env).RetrievePiece(req.Context, pieceCID, mpid, minerAddr)
		if err != nil {
			return err
		}

		return re.Emit(readCloser)
	},
}

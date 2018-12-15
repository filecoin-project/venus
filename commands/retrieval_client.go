package commands

import (
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/address"
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

		readCloser, err := GetAPI(env).RetrievalClient().RetrievePiece(req.Context, pieceCID, minerAddr)
		if err != nil {
			return err
		}

		return re.Emit(readCloser)
	},
}

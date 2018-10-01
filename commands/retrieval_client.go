package commands

import (
	"gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds"
	"gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	"gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
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
		cmdkit.StringArg("pid", true, false, "libp2p peer id of storage miner"),
		cmdkit.StringArg("cid", true, false, "content identifier of piece to read"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		minerPeerID, err := peer.IDB58Decode(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		pieceCID, err := cid.Decode(req.Arguments[1])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		readCloser, err := GetAPI(env).RetrievalClient().RetrievePiece(req.Context, minerPeerID, pieceCID)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(readCloser) // nolint: errcheck
	},
}

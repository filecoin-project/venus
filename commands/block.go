package commands

import (
	"fmt"
	"io"
	"strconv"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/types"
)

var showCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Get human-readable representations of filecoin objects",
	},
	Subcommands: map[string]*cmds.Command{
		"block": showBlockCmd,
	},
}

var showBlockCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show a Filecoin block by its CID",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ref", true, false, "CID of block to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		block, err := GetAPI(env).Block().Get(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(block)
	},
	Type: types.Block{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, block *types.Block) error {
			output := fmt.Sprintf(`Block Details
Miner:       %s
Ticket:      %s
Numerator:   %s
Denominator: %s
Height:      %s
Nonce:       %s`,
				block.Miner,
				block.Ticket,
				strconv.FormatUint(uint64(block.ParentWeightNum), 10),
				strconv.FormatUint(uint64(block.ParentWeightDenom), 10),
				strconv.FormatUint(uint64(block.Height), 10),
				strconv.FormatUint(uint64(block.Nonce), 10),
			)

			_, err := fmt.Fprintln(w, output)
			return err
		}),
	},
}

package commands

import (
	"fmt"
	"io"
	"strconv"

	"gx/ipfs/QmQtQrtNioesAWtrx8csBvfY37gTe94d6wQ3VikZUjxD39/go-ipfs-cmds"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
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
		Tagline: "Show a filecoin block by its CID",
		ShortDescription: `Prints the miner, parent weight, height,
and nonce of a given block. If JSON encoding is specified with the --enc flag,
all other block properties will be included as well.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ref", true, false, "CID of block to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		block, err := GetPorcelainAPI(env).BlockGet(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(block)
	},
	Type: types.Block{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, block *types.Block) error {
			wStr, err := types.FixedStr(uint64(block.ParentWeight))
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(w, `Block Details
Miner:  %s
Weight: %s
Height: %s
Nonce:  %s
`,
				block.Miner,
				wStr,
				strconv.FormatUint(uint64(block.Height), 10),
				strconv.FormatUint(uint64(block.Nonce), 10),
			)
			return err
		}),
	},
}

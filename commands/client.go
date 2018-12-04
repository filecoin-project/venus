package commands

import (
	"fmt"
	"io"
	"strconv"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	cmds "gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
)

var clientCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Make deals, store data, retrieve data",
	},
	Subcommands: map[string]*cmds.Command{
		"cat":                  clientCatCmd,
		"import":               clientImportDataCmd,
		"propose-storage-deal": clientProposeStorageDealCmd,
		"query-storage-deal":   clientQueryStorageDealCmd,
		"list-asks":            clientListAsksCmd,
	},
}

var clientCatCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Read out data stored on the network",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("cid", true, false, "cid of data to read"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		c, err := cid.Decode(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		dr, err := GetAPI(env).Client().Cat(req.Context, c)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(dr) // nolint: errcheck
	},
}

var clientImportDataCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "import data into the local node",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.FileArg("file", true, false, "path to file to import").EnableStdin(),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fi, err := req.Files.NextFile()
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		out, err := GetAPI(env).Client().ImportData(req.Context, fi)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(out.Cid()) // nolint: errcheck
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c *cid.Cid) error {
			return PrintString(w, c)
		}),
	},
}

var clientProposeStorageDealCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "propose a storage deal with a storage miner",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "address of miner to propose to"),
		cmdkit.StringArg("data", true, false, "cid of the data to be stored"),
		cmdkit.StringArg("ask", true, false, "ID of ask to propose a deal for"),
		cmdkit.StringArg("duration", true, false, "number of blocks to store the data for"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		miner, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		data, err := cid.Decode(req.Arguments[1])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		askid, err := strconv.ParseUint(req.Arguments[2], 10, 64)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		duration, err := strconv.ParseUint(req.Arguments[3], 10, 64)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		resp, err := GetAPI(env).Client().ProposeStorageDeal(req.Context, data, miner, askid, duration)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(resp) // nolint: errcheck
	},
	Type: storage.DealResponse{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, resp *storage.DealResponse) error {
			fmt.Fprintf(w, "State:   %s\n", resp.State.String())    // nolint: errcheck
			fmt.Fprintf(w, "Message: %s\n", resp.Message)           // nolint: errcheck
			fmt.Fprintf(w, "DealID:  %s\n", resp.Proposal.String()) // nolint: errcheck
			return nil
		}),
	},
}

var clientQueryStorageDealCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "query a storage deals status",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("id", true, false, "cid of deal to query"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		propcid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		resp, err := GetAPI(env).Client().QueryStorageDeal(req.Context, propcid)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(resp) // nolint: errcheck
	},
	Type: storage.DealResponse{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, resp *storage.DealResponse) error {
			fmt.Fprintf(w, "Status: %s\n", resp.State.String()) // nolint: errcheck
			fmt.Fprintf(w, "Message: %s\n", resp.Message)       // nolint: errcheck
			return nil
		}),
	},
}

var clientListAsksCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "list all asks in the storage market",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		asksCh, err := GetAPI(env).Client().ListAsks(req.Context)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		for a := range asksCh {
			if a.Error != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}
			re.Emit(a) // nolint: errcheck
		}
	},
	Type: api.Ask{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, ask *api.Ask) error {
			fmt.Fprintf(w, "%s %.3d %s %s\n", ask.Miner, ask.ID, ask.Price, ask.Expiry) // nolint: errcheck
			return nil
		}),
	},
}

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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		c, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		dr, err := GetAPI(env).Client().Cat(req.Context, c)
		if err != nil {
			return err
		}

		return re.Emit(dr)
	},
}

var clientImportDataCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "import data into the local node",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.FileArg("file", true, false, "path to file to import").EnableStdin(),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fi, err := req.Files.NextFile()
		if err != nil {
			return err
		}

		out, err := GetAPI(env).Client().ImportData(req.Context, fi)
		if err != nil {
			return err
		}

		return re.Emit(out.Cid())
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c cid.Cid) error {
			return PrintString(w, c)
		}),
	},
}

var clientProposeStorageDealCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "propose a storage deal with a storage miner",
		ShortDescription: `Sends a storage deal proposal to a miner`,
		LongDescription: `
Send a storage deal proposal to a miner. The first and second arguments to this
subcommand should be the address of the miner to send the deal and the CID of
the data to be stored respectively.

The third argument should be an ask ID representing how much to exchange for the
storage deal. Existing asks can be listed with the following command:

$ go-filecoin client list-asks

See the miner command help text for more information on asks.

The last argument should be the number of blocks for which to store the data.
New blocks are generated about every 30 seconds, so the time given should be
represented as a count of 30 second intervals. For example, 1 minute would be 2,
1 hour would be 120, and 1 day would be 2880.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "address of miner to send storage proposal"),
		cmdkit.StringArg("data", true, false, "CID of the data to be stored"),
		cmdkit.StringArg("ask", true, false, "ID of ask for which to propose a deal"),
		cmdkit.StringArg("duration", true, false, "time in blocks (about 30 seconds) to store data"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		miner, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		data, err := cid.Decode(req.Arguments[1])
		if err != nil {
			return err
		}

		askid, err := strconv.ParseUint(req.Arguments[2], 10, 64)
		if err != nil {
			return err
		}

		duration, err := strconv.ParseUint(req.Arguments[3], 10, 64)
		if err != nil {
			return err
		}

		resp, err := GetAPI(env).Client().ProposeStorageDeal(req.Context, data, miner, askid, duration)
		if err != nil {
			return err
		}

		return re.Emit(resp)
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		propcid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		resp, err := GetAPI(env).Client().QueryStorageDeal(req.Context, propcid)
		if err != nil {
			return err
		}

		return re.Emit(resp)
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		asksCh, err := GetAPI(env).Client().ListAsks(req.Context)
		if err != nil {
			return err
		}

		for a := range asksCh {
			if a.Error != nil {
				return err
			}
			if err := re.Emit(a); err != nil {
				return err
			}
		}
		return nil
	},
	Type: api.Ask{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, ask *api.Ask) error {
			fmt.Fprintf(w, "%s %.3d %s %s\n", ask.Miner, ask.ID, ask.Price, ask.Expiry) // nolint: errcheck
			return nil
		}),
	},
}

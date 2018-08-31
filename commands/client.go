package commands

import (
	"encoding/hex"
	"fmt"
	"io"
	"strconv"

	cmds "gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds"
	cmdkit "gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"
)

var clientCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage client operations",
	},
	Subcommands: map[string]*cmds.Command{
		"add-bid":              clientAddBidCmd,
		"cat":                  clientCatCmd,
		"import":               clientImportDataCmd,
		"propose-deal":         clientProposeDealCmd,
		"query-deal":           clientQueryDealCmd,
		"propose-storage-deal": clientProposeStorageDealCmd,
		"query-storage-deal":   clientQueryStorageDealCmd,
	},
}

var clientAddBidCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Add a bid to the storage market",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("size", true, false, "size in bytes of the bid"),
		cmdkit.StringArg("price", true, false, "the price of the bid"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address to send the bid from"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		o := req.Options["from"]
		var fromAddr address.Address
		if o != nil {
			var err error
			fromAddr, err = address.NewFromString(o.(string))
			if err != nil {
				re.SetError(errors.Wrap(err, "invalid from address"), cmdkit.ErrNormal)
				return
			}
		}

		size, ok := types.NewBytesAmountFromString(req.Arguments[0], 10)
		if !ok {
			re.SetError(ErrInvalidSize, cmdkit.ErrNormal)
			return
		}

		price, ok := types.NewAttoFILFromFILString(req.Arguments[1])
		if !ok {
			re.SetError(ErrInvalidPrice, cmdkit.ErrNormal)
			return
		}

		smsgCid, err := GetAPI(env).Client().AddBid(req.Context, fromAddr, size, price)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(smsgCid) // nolint: errcheck
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c *cid.Cid) error {
			return PrintString(w, c)
		}),
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

var clientProposeDealCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "propose a deal",
	},
	Options: []cmdkit.Option{
		// TODO: use UintOption once its fixed, ref go-ipfs-cmdkit#15
		cmdkit.UintOption("ask", "ID of ask to propose a deal for"),
		cmdkit.UintOption("bid", "ID of bid to propose a deal for"),
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("data", true, false, "cid of data to be referenced in deal"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		askID, ok := req.Options["ask"].(uint)
		if !ok {
			re.SetError("must specify an ask", cmdkit.ErrNormal)
			return
		}

		bidID, ok := req.Options["bid"].(uint)
		if !ok {
			re.SetError("must specify a bid", cmdkit.ErrNormal)
			return
		}

		// ensure arg is a valid cid
		c, err := cid.Decode(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		resp, err := GetAPI(env).Client().ProposeDeal(req.Context, askID, bidID, c)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(resp) // nolint: errcheck
	},
	Type: node.DealResponse{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, resp *node.DealResponse) error {
			fmt.Fprintf(w, "Status: %s\n", resp.State.String()) // nolint: errcheck
			fmt.Fprintf(w, "ID: %x\n", resp.ID)                 // nolint: errcheck
			return nil
		}),
	},
}

var clientQueryDealCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "query a deals status",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("id", true, false, "hex ID of deal to query"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		id, err := hex.DecodeString(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		resp, err := GetAPI(env).Client().QueryDeal(req.Context, id)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(resp) // nolint: errcheck
	},
	Type: node.DealResponse{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, resp *node.DealResponse) error {
			fmt.Fprintf(w, "Status: %s\n", resp.State.String()) // nolint: errcheck
			fmt.Fprintf(w, "ID: %x\n", resp.ID)                 // nolint: errcheck
			fmt.Fprintf(w, "Message: %s\n", resp.Message)       // nolint: errcheck
			if resp.MsgCid != nil {
				fmt.Fprintf(w, "MsgCid: %s\n", resp.MsgCid) // nolint: errcheck
			}
			return nil
		}),
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
		cmdkit.StringArg("duration", true, false, "number of blocks to store the data for"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("price", "price per byte per block"),
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

		duration, err := strconv.ParseUint(req.Arguments[2], 10, 64)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		price, ok := req.Options["price"].(string)
		if !ok {
			re.SetError("failed to get price option", cmdkit.ErrNormal)
			return
		}

		af, ok := types.NewAttoFILFromString(price, 10)
		if !ok {
			re.SetError("failed to parse price", cmdkit.ErrNormal)
			return
		}

		resp, err := GetAPI(env).Client().ProposeStorageDeal(req.Context, data, miner, af, duration)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(resp) // nolint: errcheck
	},
	Type: node.DealResponse{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, resp *node.DealResponse) error {
			fmt.Fprintf(w, "Status: %s\n", resp.State.String()) // nolint: errcheck
			fmt.Fprintf(w, "ID: %x\n", resp.ID)                 // nolint: errcheck
			fmt.Fprintf(w, "Message: %s\n", resp.Message)       // nolint: errcheck
			if resp.MsgCid != nil {
				fmt.Fprintf(w, "MsgCid: %s\n", resp.MsgCid) // nolint: errcheck
			}
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
	Type: node.StorageDealResponse{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, resp *node.StorageDealResponse) error {
			fmt.Fprintf(w, "Status: %s\n", resp.State.String()) // nolint: errcheck
			fmt.Fprintf(w, "Message: %s\n", resp.Message)       // nolint: errcheck
			return nil
		}),
	},
}

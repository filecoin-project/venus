package commands

import (
	"encoding/hex"
	"fmt"
	"io"

	chunk "gx/ipfs/QmVDjhUMtkRskBFAVNwyXuLSKbeAya7JKPnzAxMKDaK4x4/go-ipfs-chunker"
	cmds "gx/ipfs/QmVTmXZC2yE38SDKRihn96LXX6KwBWgzAg8aCDZaMirCHm/go-ipfs-cmds"
	imp "gx/ipfs/QmXBooHftCHoCUmwuxSibWCgLzmRw2gd2FBTJowsWKy9vE/go-unixfs/importer"
	uio "gx/ipfs/QmXBooHftCHoCUmwuxSibWCgLzmRw2gd2FBTJowsWKy9vE/go-unixfs/io"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	cmdkit "gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"
	dag "gx/ipfs/QmeCaeBmCCEJrZahwXY4G2G8zRaNBWskrfKWoQ6Xv6c1DR/go-merkledag"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"
)

var clientCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage client operations",
	},
	Subcommands: map[string]*cmds.Command{
		"add-bid":      clientAddBidCmd,
		"cat":          clientCatCmd,
		"import":       clientImportDataCmd,
		"propose-deal": clientProposeDealCmd,
		"query-deal":   clientQueryDealCmd,
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
		n := GetNode(env)

		fromAddr, err := fromAddress(req.Options, n)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		size, ok := types.NewBytesAmountFromString(req.Arguments[0], 10)
		if !ok {
			re.SetError(ErrInvalidSize, cmdkit.ErrNormal)
			return
		}

		price, ok := types.NewAttoFILFromFILString(req.Arguments[1], 10)
		if !ok {
			re.SetError(ErrInvalidPrice, cmdkit.ErrNormal)
			return
		}

		funds := price.CalculatePrice(size)

		params, err := abi.ToEncodedValues(price, size)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		msg, err := node.NewMessageWithNextNonce(req.Context, n, fromAddr, address.StorageMarketAddress, funds, "addBid", params)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		smsg, err := types.NewSignedMessage(*msg, n.Wallet)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		err = n.AddNewMessage(req.Context, smsg)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		smsgCid, err := smsg.Cid()
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
		nd := GetNode(env)

		// TODO: this goes back to 'how is data stored and referenced'
		// For now, lets just do things the ipfs way.

		c, err := cid.Decode(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		ds := dag.NewDAGService(nd.Blockservice)

		data, err := ds.Get(req.Context, c)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		dr, err := uio.NewDagReader(req.Context, data, ds)
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
		cmdkit.IntOption("ask", "ID of ask to propose a deal for"),
		cmdkit.IntOption("bid", "ID of bid to propose a deal for"),
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("data", true, false, "cid of data to be referenced in deal"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		nd := GetNode(env)

		askID, ok := req.Options["ask"].(int)
		if !ok {
			re.SetError("must specify an ask", cmdkit.ErrNormal)
			return
		}

		bidID, ok := req.Options["bid"].(int)
		if !ok {
			re.SetError("must specify a bid", cmdkit.ErrNormal)
			return
		}

		// ensure arg is a valid cid
		_, err := cid.Decode(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		defaddr := nd.RewardAddress()

		propose := &node.DealProposal{
			ClientSig: string(defaddr[:]), // TODO: actual crypto
			Deal: &storagemarket.Deal{
				Ask:     uint64(askID),
				Bid:     uint64(bidID),
				DataRef: req.Arguments[0],
			},
		}

		resp, err := nd.StorageClient.ProposeDeal(req.Context, propose)
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
		nd := GetNode(env)

		idslice, err := hex.DecodeString(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		if len(idslice) != 32 {
			re.SetError("id must be 32 bytes long", cmdkit.ErrNormal)
			return
		}

		var id [32]byte
		copy(id[:], idslice)

		resp, err := nd.StorageClient.QueryDeal(req.Context, id)
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
		nd := GetNode(env)

		ds := dag.NewDAGService(nd.Blockservice)

		fi, err := req.Files.NextFile()
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		spl := chunk.DefaultSplitter(fi)

		out, err := imp.BuildDagFromReader(ds, spl)
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

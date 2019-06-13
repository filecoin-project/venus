package commands

import (
	"fmt"
	"io"
	"strconv"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs-files"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/types"
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
		"payments":             paymentsCmd,
	},
}

var clientCatCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Read out data stored on the network",
		ShortDescription: `
Prints data from the storage market specified with a given CID to stdout. The
only argument should be the CID to return. The data will be returned in whatever
format was provided with the data initially.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("cid", true, false, "CID of data to read"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		c, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		dr, err := GetPorcelainAPI(env).DAGCat(req.Context, c)
		if err != nil {
			return err
		}

		return re.Emit(dr)
	},
}

var clientImportDataCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Import data into the local node",
		ShortDescription: `
Imports data previously exported with the client cat command into the storage
market. This command takes only one argument, the path of the file to import.
See the go-filecoin client cat command for more details.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.FileArg("file", true, false, "Path to file to import").EnableStdin(),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		iter := req.Files.Entries()
		if !iter.Next() {
			return fmt.Errorf("no file given: %s", iter.Err())
		}

		fi, ok := iter.Node().(files.File)
		if !ok {
			return fmt.Errorf("given file was not a files.File")
		}

		out, err := GetPorcelainAPI(env).DAGImportData(req.Context, fi)
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
		Tagline:          "Propose a storage deal with a storage miner",
		ShortDescription: `Sends a storage deal proposal to a miner`,
		LongDescription: `
Send a storage deal proposal to a miner. IDs provided to this command should
represent valid asks. Existing asks can be listed with the following command:

$ go-filecoin client list-asks

See the miner command help text for more information on asks.

Duration should be specified with the number of blocks for which to store the
data. New blocks are generated about every 30 seconds, so the time given should
be represented as a count of 30 second intervals. For example, 1 minute would
be 2, 1 hour would be 120, and 1 day would be 2880.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "Address of miner to send storage proposal"),
		cmdkit.StringArg("data", true, false, "CID of the data to be stored"),
		cmdkit.StringArg("ask", true, false, "ID of ask for which to propose a deal"),
		cmdkit.StringArg("duration", true, false, "Time in blocks (about 30 seconds per block) to store data"),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("allow-duplicates", "Allows duplicate proposals to be created. Unless this flag is set, you will not be able to make more than one deal per piece per miner. This protection exists to prevent erroneous duplicate deals."),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		allowDuplicates, _ := req.Options["allow-duplicates"].(bool)

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

		resp, err := GetStorageAPI(env).ProposeStorageDeal(req.Context, data, miner, askid, duration, allowDuplicates)
		if err != nil {
			return err
		}

		return re.Emit(resp)
	},
	Type: storagedeal.Response{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, resp *storagedeal.Response) error {
			fmt.Fprintf(w, "State:   %s\n", resp.State.String())       // nolint: errcheck
			fmt.Fprintf(w, "Message: %s\n", resp.Message)              // nolint: errcheck
			fmt.Fprintf(w, "DealID:  %s\n", resp.ProposalCid.String()) // nolint: errcheck
			return nil
		}),
	},
}

var clientQueryStorageDealCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Query a storage deal's status",
		ShortDescription: `
Checks the status of the storage deal proposal specified by the id. The deal
status and deal message will be returned as a formatted string unless another
format is specified with the --enc flag.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("id", true, false, "CID of deal to query"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		propcid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		resp, err := GetStorageAPI(env).QueryStorageDeal(req.Context, propcid)
		if err != nil {
			return err
		}

		return re.Emit(resp)
	},
	Type: storagedeal.Response{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, resp *storagedeal.Response) error {
			fmt.Fprintf(w, "Status: %s\n", resp.State.String()) // nolint: errcheck
			fmt.Fprintf(w, "Message: %s\n", resp.Message)       // nolint: errcheck
			return nil
		}),
	},
}

var clientListAsksCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List all asks in the storage market",
		ShortDescription: `
Lists all asks in the storage market. This command takes no arguments. Results
will be returned as a space separated table with miner, id, price and expiration
respectively.
`,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		asksCh := GetPorcelainAPI(env).ClientListAsks(req.Context)

		for a := range asksCh {
			if a.Error != nil {
				return a.Error
			}
			if err := re.Emit(a); err != nil {
				return err
			}
		}
		return nil
	},
	Type: porcelain.Ask{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, ask *porcelain.Ask) error {
			fmt.Fprintf(w, "%s %.3d %s %s\n", ask.Miner, ask.ID, ask.Price, ask.Expiry) // nolint: errcheck
			return nil
		}),
	},
}

var paymentsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "List payments for a given deal",
		ShortDescription: "List payments for a given deal",
	},
	Options: []cmdkit.Option{},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("dealCid", true, false, "Channel id from which to list vouchers"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		dealCid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return fmt.Errorf("invalid channel id")
		}

		vouchers, err := GetStorageAPI(env).Payments(req.Context, dealCid)
		if err != nil {
			return err
		}

		return re.Emit(vouchers)
	},
	Type: []*types.PaymentVoucher{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, vouchers []*types.PaymentVoucher) error {
			if _, err := fmt.Println("Channel\tAmount\tValidAt\tEncoded Voucher"); err != nil {
				return err
			}
			for _, voucher := range vouchers {
				encodedVoucher, err := voucher.Encode()
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", voucher.Channel.String(), voucher.Amount.String(), voucher.ValidAt.String(), encodedVoucher)
				if err != nil {
					return err
				}
			}
			return nil
		}),
	},
}

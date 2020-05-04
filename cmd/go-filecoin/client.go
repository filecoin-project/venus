package commands

import (
	"fmt"
	"strconv"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/pkg/errors"

	"github.com/ipfs/go-cid"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
	files "github.com/ipfs/go-ipfs-files"
	p2pcore "github.com/libp2p/go-libp2p-core/peer"
)

var clientCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Make deals, store data, retrieve data",
	},
	Subcommands: map[string]*cmds.Command{
		"cat":                  clientCatCmd,
		"import":               clientImportDataCmd,
		"propose-storage-deal": ClientProposeStorageDealCmd,
		"query-storage-deal":   ClientQueryStorageDealCmd,
		"verify-storage-deal":  clientVerifyStorageDealCmd,
		"list-asks":            clientListAsksCmd,
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
}

var ClientProposeStorageDealCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Propose a storage deal with a storage miner",
		ShortDescription: `Sends a storage deal proposal to a miner`,
		LongDescription: `
Send a storage deal proposal to a miner.

Start and end should be specified with the number of blocks for which to store the
data. New blocks are generated about every 30 seconds, so the time given should
be represented as a count of 30 second intervals. For example, 1 minute would
be 2, 1 hour would be 120, and 1 day would be 2880.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "Address of miner to send storage proposal"),
		cmdkit.StringArg("data", true, false, "CID of the data to be stored"),
		cmdkit.StringArg("start", true, false, "Chain epoch at which deal should start"),
		cmdkit.StringArg("end", true, false, "Chain epoch at which deal should end"),
		cmdkit.StringArg("price", true, false, "Storage price per epoch of all data in FIL (e.g. 0.01)"),
		cmdkit.StringArg("collateral", true, false, "Collateral of deal in FIL (e.g. 0.01)"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("peerid", "Override miner's peer id stored on chain"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		addr, err := GetPorcelainAPI(env).WalletDefaultAddress()
		if err != nil {
			return err
		}

		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		chainHead := GetPorcelainAPI(env).ChainHeadKey()

		dataCID, err := cid.Decode(req.Arguments[1])
		if err != nil {
			return errors.Wrap(err, "could not decode data cid")
		}

		data := &storagemarket.DataRef{
			TransferType: "graphsync",
			Root:         dataCID,
		}

		status, err := GetPorcelainAPI(env).MinerGetStatus(req.Context, maddr, chainHead)
		if err != nil {
			return err
		}

		peerID := status.PeerID
		peerIDStr, _ := req.Options["peerid"].(string)
		if peerIDStr != "" {
			peerID, err = p2pcore.Decode(peerIDStr)
			if err != nil {
				return err
			}
		}

		providerInfo := &storagemarket.StorageProviderInfo{
			Address:    maddr,
			Owner:      status.OwnerAddress,
			Worker:     status.WorkerAddress,
			SectorSize: uint64(status.SectorConfiguration.SectorSize),
			PeerID:     peerID,
		}

		start, err := strconv.ParseUint(req.Arguments[2], 10, 64)
		if err != nil {
			return errors.Wrap(err, "could not parse deal start")
		}

		end, err := strconv.ParseUint(req.Arguments[3], 10, 64)
		if err != nil {
			return errors.Wrap(err, "could not parse deal end")
		}

		price, valid := types.NewAttoFILFromFILString(req.Arguments[4])
		if !valid {
			return errors.Errorf("could not parse price %s", req.Arguments[5])
		}

		collateral, valid := types.NewAttoFILFromFILString(req.Arguments[5])
		if !valid {
			return errors.Errorf("could not parse collateral %s", req.Arguments[6])
		}

		resp, err := GetStorageAPI(env).ProposeStorageDeal(
			req.Context,
			addr,
			providerInfo,
			data,
			abi.ChainEpoch(start),
			abi.ChainEpoch(end),
			price,
			collateral,
			// proof version (not circuit or size) should be all that is important here
			constants.DevRegisteredWindowPoStProof,
		)
		if err != nil {
			return err
		}

		return re.Emit(resp)
	},
	Type: storagemarket.ProposeStorageDealResult{},
}

var ClientQueryStorageDealCmd = &cmds.Command{
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
		dealCID, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return errors.Wrap(err, "could not decode deal cid")
		}

		deal, err := GetStorageAPI(env).GetStorageDeal(req.Context, dealCID)
		if err != nil {
			return err
		}

		return re.Emit(deal)
	},
	Type: storagemarket.ClientDeal{},
}

// VerifyStorageDealResult wraps the success in an interface type
type VerifyStorageDealResult struct {
	validPip bool // nolint: structcheck
}

var clientVerifyStorageDealCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Verify a storage deal",
		ShortDescription: `
Returns an error if the deal is not in the Complete state or the Piece Inclusion Proof
is invalid.  Returns nil otherwise.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("id", true, false, "CID of deal to query"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		panic("not implemented pending full storage market integration")

		//proposalCid, err := cid.Decode(req.Arguments[0])
		//if err != nil {
		//	return err
		//}
		//
		//resp, err := GetStorageAPI(env).QueryStorageDeal(req.Context, proposalCid)
		//if err != nil {
		//	return err
		//}
		//
		//if resp.State != storagedeal.Complete {
		//	return errors.New("storage deal not in Complete state")
		//}
		//
		//validateError := GetPorcelainAPI(env).ClientValidateDeal(req.Context, proposalCid, resp.ProofInfo)
		//
		//return re.Emit(VerifyStorageDealResult{validateError == nil})
	},
	Type: &VerifyStorageDealResult{},
}

var clientListAsksCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List all asks in the storage market",
		ShortDescription: `
Lists all asks in the storage market. This command takes no arguments.
`,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		minerAddr, err := GetBlockAPI(env).MinerAddress()
		if err != nil {
			return err
		}

		asks, err := GetStorageAPI(env).ListAsks(minerAddr)
		if err != nil {
			return err
		}

		return re.Emit(asks)
	},
	Type: []*storagemarket.SignedStorageAsk{},
}

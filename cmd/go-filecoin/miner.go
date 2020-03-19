package commands

import (
	"fmt"
	"io"
	"math/big"
	"strconv"

	address "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	cid "github.com/ipfs/go-cid"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

var minerCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage a single miner actor",
	},
	Subcommands: map[string]*cmds.Command{
		"create":        minerCreateCmd,
		"status":        minerStatusCommand,
		"set-price":     minerSetPriceCmd,
		"update-peerid": minerUpdatePeerIDCmd,
		"set-worker":    minerSetWorkerAddressCmd,
	},
}

// MinerCreateResult is the type returned when creating a miner.
type MinerCreateResult struct {
	Address address.Address
	GasUsed gas.Unit
	Preview bool
}

var minerCreateCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Create a new file miner with <collateral> FIL",
		ShortDescription: `Issues a new message to the network to create the miner, then waits for the
message to be mined as this is required to return the address of the new miner.
Collateral will be committed at the rate of 0.001FIL per sector. When the
miner's collateral drops below 0.001FIL, the miner will not be able to commit
additional sectors.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("collateral", true, false, "The amount of collateral, in FIL."),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("sectorsize", "size of the sectors which this miner will commit, in bytes"),
		cmdkit.StringOption("from", "address to send from"),
		cmdkit.StringOption("peerid", "Base58-encoded libp2p peer ID that the miner will operate"),
		priceOption,
		limitOption,
		previewOption,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		var err error

		sectorSize, err := optionalSectorSizeWithDefault(req.Options["sectorsize"], constants.DevSectorSize)
		if err != nil {
			return err
		}

		fromAddr, err := fromAddrOrDefault(req, env)
		if err != nil {
			return err
		}

		var pid peer.ID
		peerid := req.Options["peerid"]
		if peerid != nil {
			pid, err = peer.Decode(peerid.(string))
			if err != nil {
				return errors.Wrap(err, "invalid peer id")
			}
		}
		if pid == "" {
			pid = GetPorcelainAPI(env).NetworkGetPeerID()
		}

		collateral, ok := types.NewAttoFILFromFILString(req.Arguments[0])
		if !ok {
			return ErrInvalidCollateral
		}

		gasPrice, gasLimit, preview, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		if preview {
			usedGas, err := GetPorcelainAPI(env).MinerPreviewCreate(
				req.Context,
				fromAddr,
				sectorSize,
				pid,
			)
			if err != nil {
				return err
			}
			return re.Emit(&MinerCreateResult{
				Address: address.Undef,
				GasUsed: usedGas,
				Preview: true,
			})
		}

		addr, err := GetPorcelainAPI(env).MinerCreate(
			req.Context,
			fromAddr,
			gasPrice,
			gasLimit,
			sectorSize,
			pid,
			collateral,
		)
		if err != nil {
			return errors.Wrap(err, "Could not create miner. Please consult the documentation to setup your wallet and genesis block correctly")
		}

		return re.Emit(&MinerCreateResult{
			Address: *addr,
			GasUsed: gas.NewGas(0),
			Preview: false,
		})
	},
	Type: &MinerCreateResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *MinerCreateResult) error {
			if res.Preview {
				output := strconv.FormatUint(uint64(res.GasUsed), 10)
				_, err := w.Write([]byte(output))
				return err
			}
			return PrintString(w, res.Address)
		}),
	},
}

// MinerSetPriceResult is the return type for miner set-price command
type MinerSetPriceResult struct {
	MinerAddress address.Address
	Price        types.AttoFIL
}

var minerSetPriceCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Set the minimum price for storage",
		ShortDescription: `Sets the mining.minimumPrice in config and creates a new ask for the given price.
This command waits for the ask to be mined.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("storageprice", true, false, "The new price of storage in FIL per byte per block"),
		cmdkit.StringArg("expiry", true, false, "How long this ask is valid for in blocks"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "Address to send from"),
		cmdkit.StringOption("miner", "The address of the miner owning the ask"),
		priceOption,
		limitOption,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		price, ok := types.NewAttoFILFromFILString(req.Arguments[0])
		if !ok {
			return ErrInvalidPrice
		}

		fromAddr, err := fromAddrOrDefault(req, env)
		if err != nil {
			return err
		}

		var minerAddr address.Address
		if req.Options["miner"] != nil {
			minerAddr, err = address.NewFromString(req.Options["miner"].(string))
			if err != nil {
				return errors.Wrap(err, "miner must be an address")
			}
		}

		expiry, ok := big.NewInt(0).SetString(req.Arguments[1], 10)
		if !ok {
			return fmt.Errorf("expiry must be a valid integer")
		}

		gasPrice, gasLimit, _, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		res, err := GetPorcelainAPI(env).MinerSetPrice(
			req.Context,
			fromAddr,
			minerAddr,
			gasPrice,
			gasLimit,
			price,
			expiry)
		if err != nil {
			return err
		}

		return re.Emit(&MinerSetPriceResult{minerAddr, res.Price})
	},
	Type: &MinerSetPriceResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *MinerSetPriceResult) error {
			_, err := fmt.Fprintf(w, "%s", res)
			return err
		}),
	},
}

// MinerUpdatePeerIDResult is the return type for miner update-peerid command
type MinerUpdatePeerIDResult struct {
	Cid     cid.Cid
	GasUsed gas.Unit
	Preview bool
}

var minerUpdatePeerIDCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Change the libp2p identity that a miner is operating",
		ShortDescription: `Issues a new message to the network to update the miner's libp2p identity.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("address", true, false, "Miner address to update peer ID for"),
		cmdkit.StringArg("peerid", true, false, "Base58-encoded libp2p peer ID that the miner will operate"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "Address to send from"),
		priceOption,
		limitOption,
		previewOption,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		minerAddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		fromAddr, err := fromAddrOrDefault(req, env)
		if err != nil {
			return err
		}

		newPid, err := peer.Decode(req.Arguments[1])
		if err != nil {
			return err
		}

		gasPrice, gasLimit, preview, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		if preview {
			usedGas, err := GetPorcelainAPI(env).MessagePreview(
				req.Context,
				fromAddr,
				minerAddr,
				builtin.MethodsMiner.ChangePeerID,
				newPid,
			)
			if err != nil {
				return err
			}

			return re.Emit(&MinerUpdatePeerIDResult{
				Cid:     cid.Cid{},
				GasUsed: usedGas,
				Preview: true,
			})
		}

		params := miner.ChangePeerIDParams{NewID: newPid}

		c, _, err := GetPorcelainAPI(env).MessageSend(
			req.Context,
			fromAddr,
			minerAddr,
			types.ZeroAttoFIL,
			gasPrice,
			gasLimit,
			builtin.MethodsMiner.ChangePeerID,
			&params,
		)
		if err != nil {
			return err
		}

		return re.Emit(&MinerUpdatePeerIDResult{
			Cid:     c,
			GasUsed: gas.NewGas(0),
			Preview: false,
		})
	},
	Type: &MinerUpdatePeerIDResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *MinerUpdatePeerIDResult) error {
			if res.Preview {
				output := strconv.FormatUint(uint64(res.GasUsed), 10)
				_, err := w.Write([]byte(output))
				return err
			}
			return PrintString(w, res.Cid)
		}),
	},
}

var minerStatusCommand = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Get the status of a miner",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		minerAddr, err := optionalAddr(req.Arguments[0])
		if err != nil {
			return err
		}

		porcelainAPI := GetPorcelainAPI(env)
		status, err := porcelainAPI.MinerGetStatus(req.Context, minerAddr, porcelainAPI.ChainHeadKey())
		if err != nil {
			return err
		}
		return re.Emit(status)
	},
	Type: porcelain.MinerStatus{},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "A miner actor address"),
	},
}

var minerSetWorkerAddressCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Set the address of the miner worker. Returns a message CID",
		ShortDescription: "Set the address of the miner worker to the provided address. When a miner is created, this address defaults to the miner owner. Use this command to change the default. Returns a message CID to wait for the message to appear on chain.",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("new-address", true, false, "The address of the new miner worker."),
	},
	Options: []cmdkit.Option{
		priceOption,
		limitOption,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		newWorker, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		gasPrice, gasLimit, _, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		msgCid, err := GetPorcelainAPI(env).MinerSetWorkerAddress(req.Context, newWorker, gasPrice, gasLimit)
		if err != nil {
			return err
		}

		return re.Emit(msgCid)
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c cid.Cid) error {
			fmt.Fprintln(w, c) // nolint: errcheck
			return nil
		}),
	},
}

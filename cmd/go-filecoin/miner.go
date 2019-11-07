package commands

import (
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

var minerCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage a single miner actor",
	},
	Subcommands: map[string]*cmds.Command{
		"create":         minerCreateCmd,
		"owner":          minerOwnerCmd,
		"power":          minerPowerCmd,
		"set-price":      minerSetPriceCmd,
		"update-peerid":  minerUpdatePeerIDCmd,
		"collateral":     minerCollateralCmd,
		"proving-window": minerProvingWindowCmd,
		"set-worker":     minerSetWorkerAddressCmd,
		"worker":         minerWorkerAddressCmd,
	},
}

// MinerCreateResult is the type returned when creating a miner.
type MinerCreateResult struct {
	Address address.Address
	GasUsed types.GasUnits
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

		pp, err := GetPorcelainAPI(env).ProtocolParameters(req.Context)
		if err != nil {
			return err
		}

		sectorSize, err := optionalSectorSizeWithDefault(req.Options["sectorsize"], pp.SupportedSectors[0].Size)
		if err != nil {
			return err
		}

		// TODO: It may become the case that the protocol does not specify an
		// enumeration of supported sector sizes, but rather that any sector
		// size for which a miner has Groth parameters and a verifying key is
		// supported.
		// https://github.com/filecoin-project/specs/pull/318
		if !pp.IsSupportedSectorSize(sectorSize) {
			supportedStrs := make([]string, len(pp.SupportedSectors))
			for i, si := range pp.SupportedSectors {
				supportedStrs[i] = si.Size.String()
			}
			return fmt.Errorf("unsupported sector size: %s (supported sizes: %s)", sectorSize, strings.Join(supportedStrs, ", "))
		}

		fromAddr, err := fromAddrOrDefault(req, env)
		if err != nil {
			return err
		}

		var pid peer.ID
		peerid := req.Options["peerid"]
		if peerid != nil {
			pid, err = peer.IDB58Decode(peerid.(string))
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
			GasUsed: types.NewGasUnits(0),
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
	GasUsed               types.GasUnits
	MinerSetPriceResponse porcelain.MinerSetPriceResponse
	Preview               bool
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
		previewOption,
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

		gasPrice, gasLimit, preview, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		if preview {
			usedGas, err := GetPorcelainAPI(env).MinerPreviewSetPrice(
				req.Context,
				fromAddr,
				minerAddr,
				price,
				expiry)
			if err != nil {
				return err
			}
			return re.Emit(&MinerSetPriceResult{
				GasUsed:               usedGas,
				Preview:               true,
				MinerSetPriceResponse: porcelain.MinerSetPriceResponse{},
			})
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

		return re.Emit(&MinerSetPriceResult{
			GasUsed:               types.NewGasUnits(0),
			Preview:               false,
			MinerSetPriceResponse: res,
		})
	},
	Type: &MinerSetPriceResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *MinerSetPriceResult) error {
			if res.Preview {
				output := strconv.FormatUint(uint64(res.GasUsed), 10)
				_, err := w.Write([]byte(output))
				return err
			}
			_, err := fmt.Fprintf(w, `Set price for miner %s to %s.
	Published ask, cid: %s.
	Ask confirmed on chain in block: %s.
`,
				res.MinerSetPriceResponse.MinerAddr.String(),
				res.MinerSetPriceResponse.Price.String(),
				res.MinerSetPriceResponse.AddAskCid.String(),
				res.MinerSetPriceResponse.BlockCid.String(),
			)
			return err
		}),
	},
}

// MinerUpdatePeerIDResult is the return type for miner update-peerid command
type MinerUpdatePeerIDResult struct {
	Cid     cid.Cid
	GasUsed types.GasUnits
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

		newPid, err := peer.IDB58Decode(req.Arguments[1])
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
				miner.UpdatePeerID,
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

		c, _, err := GetPorcelainAPI(env).MessageSend(
			req.Context,
			fromAddr,
			minerAddr,
			types.ZeroAttoFIL,
			gasPrice,
			gasLimit,
			miner.UpdatePeerID,
			newPid,
		)
		if err != nil {
			return err
		}

		return re.Emit(&MinerUpdatePeerIDResult{
			Cid:     c,
			GasUsed: types.NewGasUnits(0),
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

var minerOwnerCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Show the actor address of <miner>",
		ShortDescription: `Given <miner> miner address, output the address of the actor that owns the miner.`,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		minerAddr, err := optionalAddr(req.Arguments[0])
		if err != nil {
			return err
		}
		ownerAddr, err := GetPorcelainAPI(env).MinerGetOwnerAddress(req.Context, minerAddr)
		if err != nil {
			return err
		}

		return re.Emit(&ownerAddr)
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "The address of the miner"),
	},
	Type: address.Address{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, a *address.Address) error {
			return PrintString(w, a)
		}),
	},
}

var minerPowerCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Get the power of a miner versus the total storage market power",
		ShortDescription: `Check the current power of a given miner and total power of the storage market.
Values will be output as a ratio where the first number is the miner power and second is the total market power.`,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		minerAddr, err := optionalAddr(req.Arguments[0])
		if err != nil {
			return err
		}

		minerPower, err := GetPorcelainAPI(env).MinerGetPower(req.Context, minerAddr)
		if err != nil {
			return err
		}
		return re.Emit(minerPower)
	},
	Type: porcelain.MinerPower{},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "The address of the miner"),
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *porcelain.MinerPower) error {
			outStr := fmt.Sprintf("%s / %s", out.Power.String(), out.Total.String())
			_, err := fmt.Fprintln(w, outStr)
			return err
		}),
	},
}

var minerCollateralCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Get the active collateral of a miner",
		ShortDescription: `Check the actively staked collateral of a given miner. Values reported in attoFIL`,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		minerAddr, err := optionalAddr(req.Arguments[0])
		if err != nil {
			return err
		}
		collateral, err := GetPorcelainAPI(env).MinerGetCollateral(req.Context, minerAddr)
		if err != nil {
			return err
		}
		return re.Emit(collateral)
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "The address of the miner"),
	},
	Type: types.AttoFIL{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, af types.AttoFIL) error {
			return PrintString(w, af)
		}),
	},
}

var minerProvingWindowCmd = &cmds.Command{
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "Miner address to get proving window for"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		// Get the Miner Address
		minerAddress, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		mpp, err := GetPorcelainAPI(env).MinerGetProvingWindow(req.Context, minerAddress)
		if err != nil {
			return err
		}
		return re.Emit(mpp)
	},
	Type: porcelain.MinerProvingWindow{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *porcelain.MinerProvingWindow) error {
			var pSet []string
			for p := range res.ProvingSet {
				pSet = append(pSet, p)
			}
			_, err := fmt.Fprintf(w, `Proving Window
Start:      %s
End:        %s
ProvingSet: %s
`,
				res.Start.String(),
				res.End.String(),
				pSet,
			)
			return err
		}),
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

// MinerWorkerResult is a struct containing the result of a MinerWorker or MinerSetWorker command.
type MinerWorkerResult struct {
	WorkerAddress address.Address `json:"workerAddress"`
}

var minerWorkerAddressCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Show the address of the miner worker",
		ShortDescription: "Show the address of the miner worker",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ret, err := GetPorcelainAPI(env).ConfigGet("mining.minerAddress")
		if err != nil {
			return errors.Wrap(err, "problem getting miner address")
		}
		minerAddr, ok := ret.(address.Address)
		if !ok {
			return errors.New("problem converting miner address")
		}
		workerAddr, err := GetPorcelainAPI(env).MinerGetWorkerAddress(req.Context, minerAddr, GetPorcelainAPI(env).ChainHeadKey())
		if err != nil {
			return errors.Wrap(err, "problem getting worker address")
		}

		res := MinerWorkerResult{WorkerAddress: workerAddr}
		fmt.Printf("workerAddr: %s", res.WorkerAddress.String())
		return re.Emit(&res)
	},
	Type: &MinerWorkerResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, result *MinerWorkerResult) error {
			fmt.Fprintln(w, result.WorkerAddress.String()) // nolint: errcheck
			return nil
		}),
	},
}

package commands

import (
	"io"
	"strconv"

	"gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds"
	"gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	"gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

var minerCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage miner operations",
	},
	Subcommands: map[string]*cmds.Command{
		"create":        minerCreateCmd,
		"owner":         minerOwnerCmd,
		"update-peerid": minerUpdatePeerIDCmd,
	},
}

var minerCreateCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Create a new file miner",
		ShortDescription: `Issues a new message to the network to create the miner. Then waits for the
message to be mined as this is required to return the address of the new miner.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("pledge", true, false, "the size of the pledge (in sector count) for the miner"),
		cmdkit.StringArg("collateral", true, false, "the amount of collateral to be sent"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address to send from"),
		cmdkit.StringOption("peerid", "b58-encoded libp2p peer ID that the miner will operate"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		var err error

		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		var pid peer.ID
		peerid := req.Options["peerid"]
		if peerid != nil {
			pid, err = peer.IDB58Decode(peerid.(string))
			if err != nil {
				re.SetError(errors.Wrap(err, "invalid peer id"), cmdkit.ErrNormal)
				return
			}
		}

		pledge, err := strconv.ParseUint(req.Arguments[0], 10, 64)
		if err != nil {
			re.SetError(ErrInvalidPledge, cmdkit.ErrNormal)
			return
		}

		collateral, ok := types.NewAttoFILFromFILString(req.Arguments[1])
		if !ok {
			re.SetError(ErrInvalidCollateral, cmdkit.ErrNormal)
			return
		}

		addr, err := GetAPI(env).Miner().Create(req.Context, fromAddr, pledge, pid, collateral)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(&addr) // nolint: errcheck
	},
	Type: address.Address{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, a *address.Address) error {
			return PrintString(w, a)
		}),
	},
}

var minerUpdatePeerIDCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Change the libp2p identity that a miner is operating",
		ShortDescription: `Issues a new message to the network to update the miner's libp2p identity.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("address", true, false, "miner address to update peer ID for"),
		cmdkit.StringArg("peerid", true, false, "b58-encoded libp2p peer ID that the miner will operate"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address to send from"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		minerAddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		newPid, err := peer.IDB58Decode(req.Arguments[1])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		c, err := GetAPI(env).Miner().UpdatePeerID(req.Context, fromAddr, minerAddr, newPid)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(c) // nolint: errcheck
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c *cid.Cid) error {
			return PrintString(w, c)
		}),
	},
}

var minerOwnerCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		minerAddr, err := optionalAddr(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
		ownerAddr, err := GetAPI(env).Miner().GetOwner(req.Context, minerAddr)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal) // nolint: errcheck
			return
		}

		re.Emit(&ownerAddr) // nolint: errcheck
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "the address of the miner"),
	},
	Type: address.Address{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, a *address.Address) error {
			return PrintString(w, a)
		}),
	},
}

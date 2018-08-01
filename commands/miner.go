package commands

import (
	"io"

	"gx/ipfs/QmVTmXZC2yE38SDKRihn96LXX6KwBWgzAg8aCDZaMirCHm/go-ipfs-cmds"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	"gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"
	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/core/node"
	"github.com/filecoin-project/go-filecoin/types"
)

var minerCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage miner operations",
	},
	Subcommands: map[string]*cmds.Command{
		"create":        minerCreateCmd,
		"add-ask":       minerAddAskCmd,
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
		cmdkit.StringArg("pledge", true, false, "the size of the pledge for the miner"),
		cmdkit.StringArg("collateral", true, false, "the amount of collateral to be sent"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address to send from"),
		cmdkit.StringOption("peerid", "b58-encoded libp2p peer ID that the miner will operate"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		n := GetNode(env)

		fromAddr, err := fromAddress(req.Options, n)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		pid, err := peerID(req.Options, n)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		pledge, ok := types.NewBytesAmountFromString(req.Arguments[0], 10)
		if !ok {
			re.SetError(ErrInvalidPledge, cmdkit.ErrNormal)
			return
		}

		collateral, ok := types.NewAttoFILFromFILString(req.Arguments[1], 10)
		if !ok {
			re.SetError(ErrInvalidCollateral, cmdkit.ErrNormal)
			return
		}

		addr, err := n.CreateMiner(req.Context, fromAddr, *pledge, pid, *collateral)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(addr) // nolint: errcheck
	},
	Type: types.Address{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, a *types.Address) error {
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
		n := GetNode(env)

		minerAddr, err := types.NewAddressFromString(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		fromAddr, err := fromAddress(req.Options, n)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		newPid, err := peer.IDB58Decode(req.Arguments[1])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		args, err := abi.ToEncodedValues(newPid)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		msg, err := node.NewMessageWithNextNonce(req.Context, n, fromAddr, minerAddr, nil, "updatePeerID", args)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		smsg, err := types.NewSignedMessage(*msg, n.Wallet)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		if err := n.AddNewMessage(req.Context, smsg); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		c, err := smsg.Cid()
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

var minerAddAskCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Add an ask to the storage market",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "the address of the miner owning the ask"),
		cmdkit.StringArg("size", true, false, "size in bytes of the ask"),
		cmdkit.StringArg("price", true, false, "the price of the ask"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address to send the ask from"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		n := GetNode(env)

		fromAddr, err := fromAddress(req.Options, n)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		minerAddr, err := types.NewAddressFromString(req.Arguments[0])
		if err != nil {
			err = errors.Wrap(err, "invalid miner address")
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		size, ok := types.NewBytesAmountFromString(req.Arguments[1], 10)
		if !ok {
			re.SetError(ErrInvalidSize, cmdkit.ErrNormal)
			return
		}

		price, ok := types.NewAttoFILFromFILString(req.Arguments[2], 10)
		if !ok {
			re.SetError(ErrInvalidPrice, cmdkit.ErrNormal)
			return
		}

		params, err := abi.ToEncodedValues(price, size)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		msg, err := node.NewMessageWithNextNonce(req.Context, n, fromAddr, minerAddr, nil, "addAsk", params)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		smsg, err := types.NewSignedMessage(*msg, n.Wallet)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		if err := n.AddNewMessage(req.Context, smsg); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		c, err := smsg.Cid()
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

func peerID(opts cmdkit.OptMap, node *node.Node) (ret peer.ID, err error) {
	o := opts["peerid"]
	if o != nil {
		ret, err = peer.IDB58Decode(o.(string))
		if err != nil {
			err = errors.Wrap(err, "invalid peer id")
		}
	} else {
		ret = node.Host.ID()
	}
	return
}

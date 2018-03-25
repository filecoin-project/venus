package commands

import (
	"io"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cmds "gx/ipfs/QmYMj156vnPY7pYvtkvQiMDAzqWDDHkfiW5bYbMpYoHxhB/go-ipfs-cmds"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

var clientCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage client operations",
	},
	Subcommands: map[string]*cmds.Command{
		"add-bid": clientAddBidCmd,
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		n := GetNode(env)

		fromAddr, err := addressWithDefault(req.Options["from"], n)
		if err != nil {
			return errors.Wrap(err, "invalid from address")
		}

		size, ok := types.NewBytesAmountFromString(req.Arguments[0], 10)
		if !ok {
			return ErrInvalidSize
		}

		price, ok := types.NewTokenAmountFromString(req.Arguments[1], 10)
		if !ok {
			return ErrInvalidPrice
		}

		funds := price.CalculatePrice(size)

		params, err := abi.ToEncodedValues(price, size)
		if err != nil {
			return err
		}

		msg := types.NewMessage(fromAddr, core.StorageMarketAddress, funds, "addBid", params)
		err = n.AddNewMessage(req.Context, msg)
		if err != nil {
			return err
		}

		msgCid, err := msg.Cid()
		if err != nil {
			return err
		}

		re.Emit(msgCid) // nolint: errcheck

		return nil
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c *cid.Cid) error {
			return PrintString(w, c)
		}),
	},
}

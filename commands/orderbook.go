package commands

import (
	"encoding/json"
	"io"

	"github.com/filecoin-project/go-filecoin/core"
	cmds "gx/ipfs/QmRv6ddf7gkiEgBs1LADv3vC1mkVGPZEfByoiiVybjE9Mc/go-ipfs-cmds"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
)

var orderbookCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with the order book",
	},
	Subcommands: map[string]*cmds.Command{
		"asks":  askCmd,
		"bids":  bidCmd,
		"deals": dealCmd,
	},
}

var askCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		n := GetNode(env)
		askSet, err := n.StorageMarket.GetMarketPeeker().GetAskSet()
		if err != nil {
			re.SetError(err, cmdkit.ErrNotFound)
			return
		}
		for _, ask := range *askSet {
			re.Emit(ask) // nolint errcheck
		}
	},
	Type: &core.Ask{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, ask *core.Ask) error {
			b, err := json.Marshal(ask)
			if err != nil {
				return err
			}
			_, err = w.Write(b)
			return err
		}),
	},
}

var bidCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		n := GetNode(env)
		bidSet, err := n.StorageMarket.GetMarketPeeker().GetBidSet()
		if err != nil {
			re.SetError(err, cmdkit.ErrNotFound)
			return
		}
		for _, bid := range *bidSet {
			re.Emit(bid) // nolint errcheck
		}
	},
	Type: &core.Bid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, bid *core.Bid) error {
			b, err := json.Marshal(bid)
			if err != nil {
				return err
			}
			_, err = w.Write(b)
			return err
		}),
	},
}

var dealCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		n := GetNode(env)
		dealList, err := n.StorageMarket.GetMarketPeeker().GetDealList()
		if err != nil {
			re.SetError(err, cmdkit.ErrNotFound)
			return
		}
		for deal := range dealList {
			re.Emit(deal) // nolint errcheck
		}
	},
	Type: &core.Deal{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, deal *core.Deal) error {
			b, err := json.Marshal(deal)
			if err != nil {
				return err
			}
			_, err = w.Write(b)
			return err
		}),
	},
}

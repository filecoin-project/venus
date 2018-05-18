package commands

import (
	"encoding/json"
	"io"

	cmds "gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		n := GetNode(env)
		askSet, err := n.StorageMarket.GetMarketPeeker().GetAskSet()
		if err != nil {
			return err
		}
		for _, ask := range askSet {
			re.Emit(ask) // nolint errcheck
		}
		return nil
	},
	Type: &storagemarket.Ask{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, ask *storagemarket.Ask) error {
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		n := GetNode(env)
		bidSet, err := n.StorageMarket.GetMarketPeeker().GetBidSet()
		if err != nil {
			return err
		}
		for _, bid := range bidSet {
			re.Emit(bid) // nolint errcheck
		}
		return nil
	},
	Type: &storagemarket.Bid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, bid *storagemarket.Bid) error {
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		n := GetNode(env)
		dealList, err := n.StorageMarket.GetMarketPeeker().GetDealList()
		if err != nil {
			return err
		}
		for _, deal := range dealList {
			re.Emit(deal) // nolint errcheck
		}
		return nil
	},
	Type: &storagemarket.Deal{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, deal *storagemarket.Deal) error {
			b, err := json.Marshal(deal)
			if err != nil {
				return err
			}
			_, err = w.Write(b)
			return err
		}),
	},
}

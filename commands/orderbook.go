package commands

import (
	"encoding/json"
	"io"

	cmds "gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds"
	cmdkit "gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
)

var orderbookCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with the order book",
	},
	Subcommands: map[string]*cmds.Command{
		"asks": askCmd,
		"bids": bidCmd,
	},
}

var askCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		asks, err := GetAPI(env).Orderbook().Asks()
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		for _, ask := range asks {
			re.Emit(ask) // nolint errcheck
		}
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		bids, err := GetAPI(env).Orderbook().Bids()
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		for _, bid := range bids {
			re.Emit(bid) // nolint errcheck
		}
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

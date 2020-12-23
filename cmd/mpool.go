package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/net"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var mpoolCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manage message pool",
		ShortDescription: `
'Manage message pool.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"pending": mpoolPending,
		"clear":   mpoolClear,
		//"sub":      mpoolSub,
		//"stat":     mpoolStat,
		//"replace":  mpoolReplaceCmd,
		//"find":     mpoolFindCmd,
		//"config":   mpoolConfig,
		//"gas-perf": mpoolGasPerfCmd,
	},
}

var mpoolPending = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get pending messages",
		ShortDescription: `
Get pending messages.
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption("local", "print pending messages for addresses in local wallet only"),
		cmds.BoolOption("cids", "only print cids of messages in output"),
		cmds.BoolOption("to", "return messages to a given address"),
		cmds.BoolOption("from", "return messages from a given address"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		local, _ := req.Options["local"].(bool)
		cids, _ := req.Options["cids"].(bool)
		to, _ := req.Options["to"].(string)
		from, _ := req.Options["from"].(string)

		var toa, froma address.Address
		if to != "" {
			a, err := address.NewFromString(to)
			if err != nil {
				return fmt.Errorf("given 'to' address %q was invalid: %w", to, err)
			}
			toa = a
		}

		if from != "" {
			a, err := address.NewFromString(from)
			if err != nil {
				return fmt.Errorf("given 'to' address %q was invalid: %w", from, err)
			}
			froma = a
		}

		var filter map[address.Address]struct{}
		if local {
			filter = map[address.Address]struct{}{}

			addrss := env.(*node.Env).WalletAPI.WalletAddresses()
			for _, a := range addrss {
				filter[a] = struct{}{}
			}
		}

		msgs, err := env.(*node.Env).MessagePoolAPI.MpoolPending(req.Context, block.TipSetKey{})

		if err != nil {
			return err
		}
		for _, msg := range msgs {
			if filter != nil {
				if _, has := filter[msg.Message.From]; !has {
					continue
				}
			}

			if toa != address.Undef && msg.Message.To != toa {
				continue
			}
			if froma != address.Undef && msg.Message.From != froma {
				continue
			}

			if cids {
				fmt.Println(msg.Cid())
			} else {
				out, err := json.MarshalIndent(msg, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(out))
			}
		}

		return nil
	},
	Type: net.SwarmConnInfos{},
}

var mpoolClear = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "clear",
		ShortDescription: `
"Clear all pending messages from the mpool (USE WITH CARE)"
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption("local", "also clear local messages"),
		cmds.BoolOption("really-do-it", "must be specified for the action to take effect"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		local, _ := req.Options["local"].(bool)
		really, _ := req.Options["really-do-it"].(bool)

		if !really {
			//nolint:golint
			return fmt.Errorf("--really-do-it must be specified for this action to have an effect; you have been warned")
		}

		return env.(*node.Env).MessagePoolAPI.MpoolClear(context.TODO(), local)

		return nil
	},
	Type: net.SwarmConnInfos{},
}

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/messagepool"
	"github.com/filecoin-project/venus/pkg/net"
	"github.com/filecoin-project/venus/pkg/types"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"golang.org/x/xerrors"
	stdbig "math/big"
)

var mpoolCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manage message pool",
	},
	Subcommands: map[string]*cmds.Command{
		"pending": mpoolPending,
		"clear":   mpoolClear,
		"sub":     mpoolSub,
		//"stat":     mpoolStat,
		//"replace":  mpoolReplaceCmd,
		//"find":     mpoolFindCmd,
		"config":   mpoolConfig,
		"gas-perf": mpoolGasPerfCmd,
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
				re.Emit(string(out))
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
Clear all pending messages from the mpool (USE WITH CARE)
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
	},
	Type: net.SwarmConnInfos{},
}

var mpoolSub = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "sub",
		ShortDescription: `
Subscribe to mpool changes
`,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := context.TODO()
		sub, err := env.(*node.Env).MessagePoolAPI.MpoolSub(ctx)
		if err != nil {
			return err
		}

		for {
			select {
			case update := <-sub:
				out, err := json.MarshalIndent(update, "", "  ")
				if err != nil {
					return err
				}
				re.Emit(string(out))
			case <-ctx.Done():
				return nil
			}
		}

		return nil
	},
	Type: net.SwarmConnInfos{},
}

var mpoolConfig = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "config",
		ShortDescription: `
get or set current mpool configuration
`,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := context.TODO()

		if len(req.Arguments) > 1 {
			return xerrors.Errorf("get or set current mpool configuration")
		}

		if len(req.Arguments) == 0 {
			cfg, err := env.(*node.Env).MessagePoolAPI.MpoolGetConfig(ctx)
			if err != nil {
				return err
			}

			bytes, err := json.Marshal(cfg)
			if err != nil {
				return err
			}

			re.Emit(string(bytes))
		} else {
			cfg := new(messagepool.MpoolConfig)
			bytes := []byte(req.Arguments[0])

			err := json.Unmarshal(bytes, cfg)
			if err != nil {
				return err
			}

			return env.(*node.Env).MessagePoolAPI.MpoolSetConfig(ctx, cfg)
		}

		return nil
	},
	Type: net.SwarmConnInfos{},
}

var mpoolGasPerfCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "gas-perf",
		ShortDescription: `
Check gas performance of messages in mempool
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption("all", "print gas performance for all mempool messages (default only prints for local)"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		all, _ := req.Options["all"].(bool)

		ctx := context.TODO()

		msgs, err := env.(*node.Env).MessagePoolAPI.MpoolPending(ctx, block.TipSetKey{})
		if err != nil {
			return err
		}

		var filter map[address.Address]struct{}
		if !all {
			filter = map[address.Address]struct{}{}

			addrss := env.(*node.Env).WalletAPI.WalletAddresses()

			for _, a := range addrss {
				filter[a] = struct{}{}
			}

			var filtered []*types.SignedMessage
			for _, msg := range msgs {
				if _, has := filter[msg.Message.From]; !has {
					continue
				}
				filtered = append(filtered, msg)
			}
			msgs = filtered
		}

		ts, err := env.(*node.Env).ChainAPI.ChainHead(ctx)

		if err != nil {
			return xerrors.Errorf("failed to get chain head: %w", err)
		}

		baseFee := ts.Blocks()[0].ParentBaseFee

		bigBlockGasLimit := big.NewInt(build.BlockGasLimit)

		getGasReward := func(msg *types.SignedMessage) big.Int {
			maxPremium := types.BigSub(msg.Message.GasFeeCap, baseFee)
			if types.BigCmp(maxPremium, msg.Message.GasPremium) < 0 {
				maxPremium = msg.Message.GasPremium
			}
			return types.BigMul(maxPremium, types.NewInt(uint64(msg.Message.GasLimit)))
		}

		getGasPerf := func(gasReward big.Int, gasLimit types.Unit) float64 {
			// gasPerf = gasReward * build.BlockGasLimit / gasLimit
			a := new(stdbig.Rat).SetInt(new(stdbig.Int).Mul(gasReward.Int, bigBlockGasLimit.Int))
			b := stdbig.NewRat(1, int64(gasLimit))
			c := new(stdbig.Rat).Mul(a, b)
			r, _ := c.Float64()
			return r
		}

		for _, m := range msgs {
			gasReward := getGasReward(m)
			gasPerf := getGasPerf(gasReward, m.Message.GasLimit)

			re.Emit(fmt.Sprintf("%s\t%d\t%s\t%f\n", m.Message.From, m.Message.Nonce, gasReward, gasPerf))
		}

		return nil
	},
	Type: net.SwarmConnInfos{},
}

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"golang.org/x/xerrors"
	stdbig "math/big"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/messagepool"
	"github.com/filecoin-project/venus/pkg/types"
)

var mpoolCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manage message pool",
	},
	Subcommands: map[string]*cmds.Command{
		"pending":  mpoolPending,
		"clear":    mpoolClear,
		"sub":      mpoolSub,
		"stat":     mpoolStat,
		"replace":  mpoolReplaceCmd,
		"find":     mpoolFindCmd,
		"config":   mpoolConfig,
		"gas-perf": mpoolGasPerfCmd,
		"publish":  mpoolPublish,
		"delete":   mpoolDeleteAddress,
	},
}

var mpoolDeleteAddress = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "delete",
		ShortDescription: "delete message by address",
	},
	Options: []cmds.Option{
		cmds.StringOption("from", "optionally specify the wallet for publish message"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := context.TODO()

		from, _ := req.Options["from"].(string)
		if from == "" {
			return xerrors.Errorf("address can`t be null")
		}

		addr, err := address.NewFromString(from)
		if err != nil {
			return err
		}

		err = env.(*node.Env).MessagePoolAPI.DeleteByAdress(ctx, addr)
		if err != nil {
			return err
		}

		return nil
	},
}

var mpoolPublish = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "publish",
		ShortDescription: "publish pending messages",
	},
	Options: []cmds.Option{
		cmds.StringOption("from", "optionally specify the wallet for publish message"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		from, _ := req.Options["from"].(string)

		ctx := context.TODO()

		var fromAddr address.Address
		if from == "" {
			defaddr, err := env.(*node.Env).WalletAPI.WalletDefaultAddress()
			if err != nil {
				return err
			}

			fromAddr = defaddr
		} else {
			addr, err := address.NewFromString(from)
			if err != nil {
				return err
			}

			fromAddr = addr
		}

		err := env.(*node.Env).MessagePoolAPI.MpoolPublish(ctx, fromAddr)
		if err != nil {
			return err
		}

		return nil
	},
}

var mpoolFindCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "find",
		ShortDescription: "find a message in the mempool",
	},
	Options: []cmds.Option{
		cmds.StringOption("from", "search for messages with given 'from' address"),
		cmds.StringOption("to", "search for messages with given 'to' address"),
		cmds.Int64Option("method", "search for messages with given method"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		from, _ := req.Options["from"].(string)
		to, _ := req.Options["to"].(string)
		method, _ := req.Options["method"].(int64)

		ctx := context.TODO()
		pending, err := env.(*node.Env).MessagePoolAPI.MpoolPending(ctx, block.TipSetKey{})
		if err != nil {
			return err
		}

		var toFilter, fromFilter address.Address
		if len(to) > 0 {
			a, err := address.NewFromString(to)
			if err != nil {
				return fmt.Errorf("'to' address was invalid: %w", err)
			}

			toFilter = a
		}

		if len(from) > 0 {
			a, err := address.NewFromString(from)
			if err != nil {
				return fmt.Errorf("'from' address was invalid: %w", err)
			}

			fromFilter = a
		}

		var methodFilter *abi.MethodNum
		if method > 0 {
			m := abi.MethodNum(method)
			methodFilter = &m
		}

		var out []*types.SignedMessage
		for _, m := range pending {
			if toFilter != address.Undef && m.Message.To != toFilter {
				continue
			}

			if fromFilter != address.Undef && m.Message.From != fromFilter {
				continue
			}

			if methodFilter != nil && *methodFilter != m.Message.Method {
				continue
			}

			out = append(out, m)
		}

		_ = re.Emit(out)
		return nil
	},
}

var mpoolReplaceCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "replace",
		ShortDescription: "replace a message in the mempool",
	},

	Options: []cmds.Option{
		cmds.BoolOption("auto", "automatically reprice the specified message"),
		cmds.Int64Option("gas-limit", "gas price for new message"),
		cmds.StringOption("gas-premium", "gas price for new message"),
		cmds.StringOption("max-fee", "Spend up to X FIL for this message (applicable for auto mode)"),
		cmds.StringOption("gas-feecap", "gas feecap for new message"),
		cmds.StringOption("message-cid", "message-cid"),
		cmds.StringOption("from", "message sender"),
		cmds.StringOption("nonce", "message nonce"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("from nonce", false, true, "from nonce"),
		cmds.StringArg("message-cid", false, true, "message-cid"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		auto, _ := req.Options["auto"].(bool)
		gasFeecap, _ := req.Options["gas-feecap"].(string)
		gasPremium, _ := req.Options["gas-premium"].(string)
		gasLimit, _ := req.Options["gas-limit"].(int64)
		maxFee, _ := req.Options["max-fee"].(string)
		messageCid, _ := req.Options["message-cid"].(string)
		froms, _ := req.Options["from"].(string)
		nonces, _ := req.Options["nonce"].(string)

		ctx := context.TODO()
		var from address.Address
		var nonce uint64

		if len(messageCid) > 0 {
			mcid, err := cid.Decode(messageCid)
			if err != nil {
				return err
			}

			msg, err := env.(*node.Env).ChainAPI.ChainGetMessage(ctx, mcid)
			if err != nil {
				return fmt.Errorf("could not find referenced message: %w", err)
			}

			from = msg.From
			nonce = msg.Nonce
		} else if len(froms) > 0 && len(nonces) > 0 {
			f, err := address.NewFromString(froms)
			if err != nil {
				return err
			}

			n, err := strconv.ParseUint(nonces, 10, 64)
			if err != nil {
				return err
			}

			from = f
			nonce = n
		} else {
			return xerrors.Errorf("replace a message in the mempool")
		}

		ts, err := env.(*node.Env).ChainAPI.ChainHead(ctx)
		if err != nil {
			return xerrors.Errorf("getting chain head: %w", err)
		}

		pending, err := env.(*node.Env).MessagePoolAPI.MpoolPending(ctx, ts.Key())
		if err != nil {
			return err
		}

		var found *types.SignedMessage
		for _, p := range pending {
			if p.Message.From == from && p.Message.Nonce == nonce {
				found = p
				break
			}
		}

		if found == nil {
			return fmt.Errorf("no pending message found from %s with nonce %d", from, nonce)
		}

		msg := found.Message

		if auto {
			minRBF := messagepool.ComputeMinRBF(msg.GasPremium)

			var mss *types.MessageSendSpec
			if len(maxFee) > 0 {
				maxFee, err := big.FromString(maxFee)
				if err != nil {
					return fmt.Errorf("parsing max-spend: %w", err)
				}
				mss = &types.MessageSendSpec{
					MaxFee: maxFee,
				}
			}

			// msg.GasLimit = 0 // TODO: need to fix the way we estimate gas limits to account for the messages already being in the mempool
			msg.GasFeeCap = abi.NewTokenAmount(0)
			msg.GasPremium = abi.NewTokenAmount(0)
			retm, err := env.(*node.Env).MessagePoolAPI.GasEstimateMessageGas(ctx, &msg, mss, block.TipSetKey{})
			if err != nil {
				return fmt.Errorf("failed to estimate gas values: %w", err)
			}

			msg.GasPremium = big.Max(retm.GasPremium, minRBF)
			msg.GasFeeCap = big.Max(retm.GasFeeCap, msg.GasPremium)

			mff := func() (abi.TokenAmount, error) {
				return constants.DefaultDefaultMaxFee, nil
			}

			messagepool.CapGasFee(mff, &msg, mss.MaxFee)
		} else {
			if gasLimit > 0 {
				msg.GasLimit = gasLimit
			}
			msg.GasPremium, err = big.FromString(gasPremium)
			if err != nil {
				return fmt.Errorf("parsing gas-premium: %w", err)
			}
			// TODO: estimate fee cap here
			msg.GasFeeCap, err = big.FromString(gasFeecap)
			if err != nil {
				return fmt.Errorf("parsing gas-feecap: %w", err)
			}
		}

		smsg, err := env.(*node.Env).WalletAPI.WalletSignMessage(ctx, msg.From, &msg)
		if err != nil {
			return fmt.Errorf("failed to sign message: %w", err)
		}

		cid, err := env.(*node.Env).MessagePoolAPI.MpoolPush(ctx, smsg)
		if err != nil {
			return fmt.Errorf("failed to push new message to mempool: %w", err)
		}

		_ = re.Emit(fmt.Sprintf("new message cid: %s", cid))
		return nil
	},
}

var mpoolStat = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "print mpool state messages",
		ShortDescription: `
Get pending messages.
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption("local", "print stats for addresses in local wallet only"),
		cmds.Int64Option("basefee-lookback", "number of blocks to look back for minimum basefee"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		local, _ := req.Options["local"].(bool)
		basefee, _ := req.Options["basefee-lookback"].(int)

		ctx := context.TODO()
		ts, err := env.(*node.Env).ChainAPI.ChainHead(ctx)
		if err != nil {
			return xerrors.Errorf("getting chain head: %w", err)
		}
		currBF := ts.Blocks()[0].ParentBaseFee
		minBF := currBF
		{
			currTs := ts
			for i := 0; i < basefee; i++ {
				key, err := currTs.Parents()
				if err != nil {
					return xerrors.Errorf("get TipSetKey error: %w", err)
				}
				currTs, err = env.(*node.Env).ChainAPI.ChainGetTipSet(key)
				if err != nil {
					return xerrors.Errorf("walking chain: %w", err)
				}
				if newBF := currTs.Blocks()[0].ParentBaseFee; newBF.LessThan(minBF) {
					minBF = newBF
				}
			}
		}

		var filter map[address.Address]struct{}
		if local {
			filter = map[address.Address]struct{}{}

			addrss := env.(*node.Env).WalletAPI.WalletAddresses()

			for _, a := range addrss {
				filter[a] = struct{}{}
			}
		}

		msgs, err := env.(*node.Env).MessagePoolAPI.MpoolPending(ctx, block.TipSetKey{})
		if err != nil {
			return err
		}

		type statBucket struct {
			msgs map[uint64]*types.SignedMessage
		}
		type mpStat struct {
			addr                 string
			past, cur, future    uint64
			belowCurr, belowPast uint64
			gasLimit             big.Int
		}

		buckets := map[address.Address]*statBucket{}
		for _, v := range msgs {
			if filter != nil {
				if _, has := filter[v.Message.From]; !has {
					continue
				}
			}

			bkt, ok := buckets[v.Message.From]
			if !ok {
				bkt = &statBucket{
					msgs: map[uint64]*types.SignedMessage{},
				}
				buckets[v.Message.From] = bkt
			}

			bkt.msgs[v.Message.Nonce] = v
		}

		var out []mpStat

		for a, bkt := range buckets {
			act, err := env.(*node.Env).ChainAPI.StateGetActor(ctx, a, ts.Key())
			if err != nil {
				fmt.Printf("%s, err: %s\n", a, err)
				continue
			}

			cur := act.Nonce
			for {
				_, ok := bkt.msgs[cur]
				if !ok {
					break
				}
				cur++
			}

			var s mpStat
			s.addr = a.String()
			s.gasLimit = big.Zero()

			for _, m := range bkt.msgs {
				if m.Message.Nonce < act.Nonce {
					s.past++
				} else if m.Message.Nonce > cur {
					s.future++
				} else {
					s.cur++
				}

				if m.Message.GasFeeCap.LessThan(currBF) {
					s.belowCurr++
				}
				if m.Message.GasFeeCap.LessThan(minBF) {
					s.belowPast++
				}

				s.gasLimit = big.Add(s.gasLimit, big.NewInt(m.Message.GasLimit))
			}

			out = append(out, s)
		}

		sort.Slice(out, func(i, j int) bool {
			return out[i].addr < out[j].addr
		})

		var total mpStat
		total.gasLimit = big.Zero()

		for _, stat := range out {
			total.past += stat.past
			total.cur += stat.cur
			total.future += stat.future
			total.belowCurr += stat.belowCurr
			total.belowPast += stat.belowPast
			total.gasLimit = big.Add(total.gasLimit, stat.gasLimit)

			_ = re.Emit(fmt.Sprintf("%s: Nonce past: %d, cur: %d, future: %d; FeeCap cur: %d, min-%d: %d, gasLimit: %s", stat.addr, stat.past, stat.cur, stat.future, stat.belowCurr, basefee, stat.belowPast, stat.gasLimit))
		}

		_ = re.Emit("-----")
		_ = re.Emit(fmt.Sprintf("total: Nonce past: %d, cur: %d, future: %d; FeeCap cur: %d, min-%d: %d, gasLimit: %s", total.past, total.cur, total.future, total.belowCurr, basefee, total.belowPast, total.gasLimit))

		return nil
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
		cmds.StringOption("to", "return messages to a given address"),
		cmds.StringOption("from", "return messages from a given address"),
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
				cid, err := msg.Cid()
				_ = re.Emit(cid)
				_ = re.Emit(err)
			} else {
				_ = re.Emit(msg)
			}
		}

		return nil
	},
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
				_ = re.Emit(update)
			case <-ctx.Done():
				return nil
			}
		}
	},
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

			_ = re.Emit(cfg)
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

		bigBlockGasLimit := big.NewInt(constants.BlockGasLimit)

		getGasReward := func(msg *types.SignedMessage) big.Int {
			maxPremium := big.Sub(msg.Message.GasFeeCap, baseFee)
			if big.Cmp(maxPremium, msg.Message.GasPremium) < 0 {
				maxPremium = msg.Message.GasPremium
			}
			return big.Mul(maxPremium, big.NewInt(msg.Message.GasLimit))
		}

		getGasPerf := func(gasReward big.Int, gasLimit int64) float64 {
			// gasPerf = gasReward * constants.BlockGasLimit / gasLimit
			a := new(stdbig.Rat).SetInt(new(stdbig.Int).Mul(gasReward.Int, bigBlockGasLimit.Int))
			b := stdbig.NewRat(1, gasLimit)
			c := new(stdbig.Rat).Mul(a, b)
			r, _ := c.Float64()
			return r
		}

		for _, m := range msgs {
			gasReward := getGasReward(m)
			gasPerf := getGasPerf(gasReward, m.Message.GasLimit)

			_ = re.Emit(fmt.Sprintf("%s   %d   %s  %f", m.Message.From, m.Message.Nonce, gasReward, gasPerf))
		}

		return nil
	},
}

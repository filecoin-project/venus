package cmd

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"golang.org/x/xerrors"
	"strings"
)

func reqConfidence(req *cmds.Request) abi.ChainEpoch {
	confidence, ok := req.Options["confidence"]
	if ok {
		return confidence.(abi.ChainEpoch)
	}
	return 0
}

func reqFromWithDefault(req *cmds.Request, env cmds.Environment) (address.Address, error) {
	f, ok := req.Options["from"]
	if ok {
		from, err := address.NewFromString(f.(string))
		if err != nil {
			return address.Undef, err
		}
		return from, nil
	}
	defaddr, err := env.(*node.Env).WalletAPI.WalletDefaultAddress()
	if err != nil {
		return address.Undef, err
	}
	return defaddr, nil
}

func reqBoolOption(req *cmds.Request, cmd string) bool {
	tmp, ok := req.Options[cmd]
	if ok {
		return tmp.(bool)
	}
	return false
}
func reqUint64Option(req *cmds.Request, cmd string) uint64 {
	tmp, ok := req.Options[cmd]
	if ok {
		return tmp.(uint64)
	}
	return 0
}
func reqStringOption(req *cmds.Request, cmd string) string {
	tmp, ok := req.Options[cmd]
	if ok {
		return tmp.(string)
	}
	return constants.StringEmpty
}
func reqChainEpochOption(req *cmds.Request, cmd string) abi.ChainEpoch {
	v, ok := req.Options[cmd]
	if ok {
		return abi.ChainEpoch(v.(int64))
	}
	return 0
}

func reqTipSetOption(req *cmds.Request, env cmds.Environment) (*types.TipSet, error) {
	tss, ok := req.Options["tipset"]
	if !ok {
		return nil, nil
	}
	tmp := tss.(string)
	if tmp == "" {
		return nil, nil
	}
	return ParseTipSetRef(req.Context, env, tmp)
}

func ParseTipSetRef(ctx context.Context, env cmds.Environment, tss string) (*types.TipSet, error) {
	if tss[0] == '@' {
		if tss == "@head" {
			return env.(*node.Env).ChainAPI.ChainHead(ctx)
		}

		var h uint64
		if _, err := fmt.Sscanf(tss, "@%d", &h); err != nil {
			return nil, xerrors.Errorf("parsing height tipset ref: %w", err)
		}

		return env.(*node.Env).ChainAPI.ChainGetTipSetByHeight(ctx, abi.ChainEpoch(h), types.EmptyTSK)
	}

	cids, err := ParseTipSetString(tss)
	if err != nil {
		return nil, err
	}

	if len(cids) == 0 {
		return nil, nil
	}

	k := types.NewTipSetKey(cids...)
	ts, err := env.(*node.Env).ChainAPI.ChainGetTipSet(k)
	if err != nil {
		return nil, err
	}

	return ts, nil
}

func ParseTipSetString(ts string) ([]cid.Cid, error) {
	strs := strings.Split(ts, ",")

	var cids []cid.Cid
	for _, s := range strs {
		c, err := cid.Parse(strings.TrimSpace(s))
		if err != nil {
			return nil, err
		}
		cids = append(cids, c)
	}

	return cids, nil
}

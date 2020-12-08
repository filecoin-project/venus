package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"
)

var stateCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with and query filecoin chain state",
	},
	Subcommands: map[string]*cmds.Command{
		"sector": stateSectorCmd,
	},
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

func LoadTipSet(ctx context.Context, tss string, api *node.Env) (*block.TipSet, error) {
	if tss[0] == '@' {
		if tss == "@head" {
			return api.ChainAPI.ChainHead()
		}

		var h uint64
		if _, err := fmt.Sscanf(tss, "@%d", &h); err != nil {
			return nil, xerrors.Errorf("parsing height tipset ref: %w", err)
		}

		return api.ChainAPI.ChainGetTipSetByHeight(ctx, nil, abi.ChainEpoch(h), true)
	}

	cids, err := ParseTipSetString(tss)
	if err != nil {
		return nil, err
	}

	if len(cids) == 0 {
		return nil, nil
	}

	k := block.NewTipSetKey(cids...)
	ts, err := api.ChainAPI.ChainTipSet(k)
	if err != nil {
		return nil, err
	}

	return ts, nil
}

var stateSectorCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Query the sector set of a miner",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("miner address", true, false, "miner address, eg. f01000"),
		cmds.StringArg("tipset", true, false, ""),
	},

	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 2 {
			return xerrors.Errorf("expected 1 params")
		}

		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		ts, err := LoadTipSet(req.Context, req.Arguments[1], env.(*node.Env))
		if err != nil {
			return err
		}

		sectors, err := env.(*node.Env).ChainAPI.StateMinerSectors(req.Context, maddr, nil, ts.Key())
		if err != nil {
			return err
		}

		//for _, s := range sectors {
		//	fmt.Printf("%d: %x\n", s.SectorNumber, s.SealedCID)
		//}

		return re.Emit(sectors)
	},
	Type: []*miner.SectorOnChainInfo{},
}

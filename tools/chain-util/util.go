package main

import (
	"fmt"
	"os"

	"github.com/ipfs/go-cid"
	cli "gopkg.in/urfave/cli.v2"

	"github.com/filecoin-project/go-filecoin/block"
)

type config struct {
	repoPath string
	headKey  block.TipSetKey
	outFile  *os.File
}

func parseFlags(cctx *cli.Context) (*config, error) {
	repoPath := cctx.Path(repoFlag)
	if repoPath == "" {
		return nil, fmt.Errorf("filecoin repo path required")
	}

	c := cctx.StringSlice(headFlag)
	cids, err := cidsFromSlice(c)
	if err != nil {
		return nil, err
	}
	headKey := block.NewTipSetKey(cids...)

	out := cctx.Path(outFlag)
	if out == "" {
		return nil, fmt.Errorf("output file required")
	}

	f, err := os.Create(out)
	if err != nil {
		return nil, err
	}

	return &config{
		repoPath: repoPath,
		headKey:  headKey,
		outFile:  f,
	}, nil

}
func cidsFromSlice(args []string) ([]cid.Cid, error) {
	out := make([]cid.Cid, len(args))
	for i, arg := range args {
		c, err := cid.Decode(arg)
		if err != nil {
			return nil, err
		}
		out[i] = c
	}
	return out, nil
}

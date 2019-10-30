package main

import (
	"fmt"
	"os"

	cli "gopkg.in/urfave/cli.v2"
)

type config struct {
	repoPath string
	outFile  *os.File
}

func parseFlags(cctx *cli.Context) (*config, error) {
	repoPath := cctx.Path(repoFlag)
	if repoPath == "" {
		return nil, fmt.Errorf("filecoin repo path required")
	}

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
		outFile:  f,
	}, nil

}

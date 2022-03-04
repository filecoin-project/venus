package main

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/filecoin-project/venus/venus-devtool/util"
	"github.com/urfave/cli/v2"
)

var mockCmd = &cli.Command{
	Name: "mock",
	Action: func(cctx *cli.Context) error {
		for _, t := range apiTargets {
			if err := mockAPI(t); err != nil {
				return err
			}
		}

		return nil
	},
}

func mockAPI(t util.APIMeta) error {
	opt := t.ParseOpt
	opt.ResolveImports = true
	_, astMeta, err := util.ParseInterfaceMetas(opt)
	if err != nil {
		return err
	}

	dest := filepath.Join(astMeta.Location, "mock/mock_"+strings.ToLower(t.Type.Name())+".go")
	cmd := exec.Command("go", "run", "github.com/golang/mock/mockgen", "-destination", dest,
		"-package", "mock", t.ParseOpt.ImportPath, t.Type.Name())

	return cmd.Run()
}

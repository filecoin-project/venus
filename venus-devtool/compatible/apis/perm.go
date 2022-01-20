package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/venus/venus-devtool/util"
)

var permCmd = &cli.Command{
	Name:  "perm",
	Flags: []cli.Flag{},
	Action: func(cctx *cli.Context) error {
		for _, pair := range util.APIPairs {
			originMetas, err := parsePermMetas(pair.Lotus.ParseOpt)
			if err != nil {
				log.Fatalln("parse lotus api interfaces:", err)
			}

			targetMetas, err := parsePermMetas(pair.Venus.ParseOpt)
			if err != nil {
				log.Fatalln("parse venus chain api interfaces:", err)
			}

			originMap := map[string]permMeta{}
			for _, om := range originMetas {
				if om.perm != "" {
					originMap[om.meth] = om
				}
			}

			fmt.Printf("v%d: %s <> %s\n", pair.Ver, pair.Venus.ParseOpt.ImportPath, pair.Lotus.ParseOpt.ImportPath)
			for _, tm := range targetMetas {
				om, has := originMap[tm.meth]
				if !has {
					fmt.Printf("\t- %s.%s\n", tm.iface, tm.meth)
					continue
				}

				if tm.perm != om.perm {
					fmt.Printf("\t> %s.%s: %s <> %s.%s: %s\n", tm.iface, tm.meth, tm.perm, om.iface, om.meth, om.perm)
				}
			}

			fmt.Println()
		}

		return nil
	},
}

type permMeta struct {
	pkg   string
	iface string
	meth  string
	perm  string
}

func parsePermMetas(opt util.InterfaceParseOption) ([]permMeta, error) {
	ifaceMetas, _, err := util.ParseInterfaceMetas(opt)
	if err != nil {
		return nil, err
	}

	var permMetas []permMeta
	for _, iface := range ifaceMetas {
		for _, ifMeth := range iface.Defined {
			permMetas = append(permMetas, permMeta{
				pkg:   opt.ImportPath,
				iface: iface.Name,
				meth:  ifMeth.Name,
				perm:  getPerms(ifMeth),
			})
		}
	}

	return permMetas, nil
}

func getPerms(m util.InterfaceMethodMeta) string {
	permStr := ""

	if cmtNum := len(m.Comments); cmtNum > 0 {
		if itemNum := len(m.Comments[cmtNum-1].List); itemNum > 0 {
			if strings.HasPrefix(m.Comments[cmtNum-1].List[0].Text, "//") {
				permStr = m.Comments[cmtNum-1].List[0].Text[2:]
			}
		}
	}

	for _, piece := range strings.Split(permStr, " ") {
		trimmed := strings.TrimSpace(piece)
		if strings.HasPrefix(trimmed, "perm:") {
			return trimmed[5:]
		}
	}

	return ""
}

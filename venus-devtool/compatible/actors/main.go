package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:                 "actors",
		Usage:                "devtool for template compatible checks between lotus & venus",
		EnableBashCompletion: true,
		Flags:                []cli.Flag{},
		Commands: []*cli.Command{
			sourcesCmd,
			templatesCmd,
			renderCmd,
			replicaCmd,
		},
	}

	app.Setup()

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "ERR: %v\n", err) // nolint: errcheck
	}
}

var templatesCmd = &cli.Command{
	Name: "templates",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "dst",
			Value: "",
		},
	},
	Action: func(c *cli.Context) error {
		srcDir, err := findActorsPkgDir()
		if err != nil {
			return fmt.Errorf("find chain/actors: %w", err)
		}

		log.Println("listing")
		paths, err := listFilesInDir(srcDir, filterWithSuffix(goTemplateExt))
		if err != nil {
			return fmt.Errorf("list template files: %w", err)
		}

		fmt.Println("TEMPLATES IN chain/actors:")

		for _, p := range paths {
			fmt.Printf("\t%s\n", p)
		}

		dstDir := c.String("dst")
		if dstDir == "" {
			return nil
		}

		log.Println("fetching")

		dstAbs, err := filepath.Abs(dstDir)
		if err != nil {
			return fmt.Errorf("get absolute dst path for %s: %w", dstDir, err)
		}

		return fetch(srcDir, dstAbs, paths)
	},
}

var renderCmd = &cli.Command{
	Name:      "render",
	ArgsUsage: "[dir]",
	Action: func(cctx *cli.Context) error {
		dir := cctx.Args().First()
		if dir == "" {
			return fmt.Errorf("dir is required")
		}

		abs, err := filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("get abs path for %s: %w", dir, err)
		}

		templates, err := listFilesInDir(dir, filterWithSuffix(goTemplateExt))
		if err != nil {
			return fmt.Errorf("list templates in %s: %w", abs, err)
		}

		log.Print("rendering")
		for _, tpath := range templates {
			err = render(filepath.Join(abs, tpath))
			if err != nil {
				return fmt.Errorf("for %s: %w", tpath, err)
			}

			log.Printf("%s done", tpath)
		}

		return nil
	},
}

var sourcesCmd = &cli.Command{
	Name: "sources",
	Action: func(cctx *cli.Context) error {
		srcDir, err := findActorsPkgDir()
		if err != nil {
			return fmt.Errorf("find chain/actors: %w", err)
		}

		files, err := listFilesInDir(srcDir, filterWithSuffix(goSourceCodeExt))
		if err != nil {
			return fmt.Errorf("list source code files: %w", err)
		}

		fmt.Println("SOURCES IN chain/actors:")

		for _, p := range files {
			fmt.Printf("\t%s\n", p)
		}
		return nil
	},
}

// todo: move to the appropriate
var replicaCmd = &cli.Command{
	Name: "replica",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "dst",
			Value:    "",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
		srcDir, err := findActorsPkgDir()
		if err != nil {
			return fmt.Errorf("find chain/actors: %w", err)
		}

		reg := regexp.MustCompile(`v[0-9]+.go`)

		files, err := listFilesInDir(srcDir, func(path string, d fs.DirEntry) bool {
			if d.IsDir() {
				return true
			}

			// exclude like builtin/[dir]/dir.go(replaced by actor.go)
			dir, file := filepath.Split(path)
			if strings.Contains(dir, "builtin") && strings.HasSuffix(file, ".go") {
				pf := file[:strings.LastIndex(file, ".go")]
				if strings.Contains(dir, pf) {
					fmt.Println("path:", path)
					return true
				}
			}

			// need adt.go diff_adt.go
			if strings.Contains(path, "adt.go") {
				return false
			}

			// skip test file
			if strings.HasSuffix(path, "test.go") {
				return true
			}

			if strings.HasSuffix(path, "main.go") || strings.Contains(path, "template") ||
				strings.Contains(path, "message") {
				return true
			}

			dir = filepath.Dir(path)
			arr := strings.Split(dir, "/")
			if strings.HasSuffix(path, fmt.Sprintf("%s.go", arr[len(arr)-1])) {
				return true
			}

			if reg.MatchString(d.Name()) {
				return true
			}

			return false
		})
		if err != nil {
			return fmt.Errorf("list replica files failed: %w", err)
		}

		fmt.Println("replica files IN chain/actors:")

		for _, p := range files {
			fmt.Printf("\t%s\n", p)
		}

		replacers := [][2]string{
			{"github.com/filecoin-project/lotus/chain/actors", "github.com/filecoin-project/venus/venus-shared/actors"},
			{"github.com/filecoin-project/lotus/chain/actors/adt", "github.com/filecoin-project/venus/venus-shared/actors/adt"},
			{"github.com/filecoin-project/lotus/chain/actors/aerrors", "github.com/filecoin-project/venus/venus-shared/actors/aerrors"},
			{"dtypes.NetworkName", "string"},
			{"\"github.com/filecoin-project/lotus/node/modules/dtypes\"", ""},
			{"\"github.com/filecoin-project/lotus/chain/types\"", "types \"github.com/filecoin-project/venus/venus-shared/internal\""},
			{"\"github.com/filecoin-project/lotus/blockstore\"", "blockstore \"github.com/filecoin-project/venus/pkg/util/blockstoreutil\""},
			{"golang.org/x/xerrors", "fmt"},
		}

		for _, file := range files {
			if err := fetchOne(srcDir, cctx.String("dst"), file, replacers); err != nil {
				return fmt.Errorf("fetch for %s: %w", file, err)
			}
		}
		return nil
	},
}

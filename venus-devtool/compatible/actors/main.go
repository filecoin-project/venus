package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

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
		paths, err := listFilesInDir(srcDir, goTemplateExt)
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

		templates, err := listFilesInDir(dir, goTemplateExt)
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

		files, err := listFilesInDir(srcDir, goSourceCodeExt)
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

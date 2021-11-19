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
		srcDir, paths, err := listTemplates()
		if err != nil {
			return err
		}

		log.Println("listing")

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

		templates, err := listTemplateInDir(abs)
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

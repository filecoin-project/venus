package main

import (
	"context"
	"flag"
	"log"
	"os"
)

var length int
var binom bool
var repodir string

func init() {
	flag.IntVar(&length, "length", 5, "length of fake chain to create")

	// Default repodir is different than Filecoin to avoid accidental clobbering of real data.
	flag.StringVar(&repodir, "repodir", "~/.fakecoin", "repo directory to use")

	flag.BoolVar(&binom, "binom", false, "generate multiblock tipsets where the number of blocks per epoch is drawn from a a realistic distribution")
}

func main() {
	ctx := context.Background()

	var cmd string

	if len(os.Args) > 1 {
		cmd = os.Args[1]
		if len(os.Args) > 2 {
			// Remove the cmd argument if there are options, to satisfy flag.Parse() while still allowing a command-first syntax.
			os.Args = append(os.Args[1:], os.Args[0])
		}
	}
	flag.Parse()
	switch cmd {
	default:
		flag.Usage()
	case "fake":
		if err := cmdFake(ctx, repodir); err != nil {
			log.Fatal(err)
		}
	case "actors":
		if err := cmdFakeActors(ctx, repodir); err != nil {
			log.Fatal(err)
		}
	}
}

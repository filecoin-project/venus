package porep

//package main
//
//import (
//	"fmt"
//	"log"
//	"math/rand"
//	"os"
//
//	"github.com/filecoin-project/paper-porep-drg/go-porep/drg/drgraph"
//	"github.com/urfave/cli"
//)
//
//func main() {
//	app := cli.NewApp()
//
//	DRSampleCommand := cli.Command{
//		Name:  "drsample",
//		Usage: "using drsample construction",
//		Flags: []cli.Flag{
//			cli.IntFlag{
//				Name:  "nodes, n",
//				Usage: "number of nodes in the graph",
//				Value: 10,
//			},
//			cli.Int64Flag{
//				Name:  "seed, r",
//				Usage: "number of nodes in the graph",
//				Value: int64(123456),
//			},
//		},
//		Action: func(c *cli.Context) error {
//			rand.Seed(c.Int64("seed"))
//			g := drgraph.NewDRSample(c.Int("nodes"))
//			fmt.Println(g.ToJSON())
//			return nil
//		},
//	}
//
//	BucketSampleCommand := cli.Command{
//		Name:  "bucketsample",
//		Usage: "using bucketsample construction",
//		Flags: []cli.Flag{
//			cli.IntFlag{
//				Name:  "nodes, n",
//				Usage: "number of nodes in the graph",
//				Value: 10,
//			},
//			cli.Int64Flag{
//				Name:  "seed, r",
//				Usage: "number of nodes in the graph",
//				Value: int64(123456),
//			},
//			cli.IntFlag{
//				Name:  "buckets, m",
//				Usage: "size of buckets",
//				Value: 2,
//			},
//			cli.BoolFlag{
//				Name:  "stdin-g-dash, g",
//				Usage: "pass a DRSample graph to bucket sample",
//			},
//		},
//		Action: func(c *cli.Context) error {
//			rand.Seed(c.Int64("seed"))
//			var gDash *drgraph.Graph
//			n := c.Int("nodes")
//			m := c.Int("buckets")
//
//			if gDashJson := c.Bool("stdin-g-dash"); gDashJson {
//				gDash = drgraph.NewFromStdin(os.Stdin)
//			} else {
//				gDash = drgraph.NewDRSample(n * m)
//			}
//			g := drgraph.BucketSampling(gDash, n, m)
//			fmt.Println(g.ToJSON())
//			return nil
//		},
//	}
//
//	app.Commands = []cli.Command{
//		{
//			Name:    "drg",
//			Aliases: []string{"d"},
//			Usage:   "options for task templates",
//			Subcommands: []cli.Command{
//				DRSampleCommand,
//				BucketSampleCommand,
//			},
//		},
//	}
//
//	err := app.Run(os.Args)
//	if err != nil {
//		log.Fatal(err)
//	}
//}

// Package cmd implements the command to print the blockchain.
package cmd

import (
	"bytes"
	"strconv"

	syncTypes "github.com/filecoin-project/venus/pkg/chainsync/types"
	cmds "github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/venus/app/node"
)

var syncCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Inspect the sync",
	},
	Subcommands: map[string]*cmds.Command{
		"status":         storeStatusCmd,
		"history":        historyCmd,
		"concurrent":     getConcurrent,
		"set-concurrent": setConcurrent,
	},
}

var getConcurrent = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "get concurrent of sync thread",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		concurrent := env.(*node.Env).SyncerAPI.Concurrent(req.Context)
		return printOneString(re, strconv.Itoa(int(concurrent)))
	},
}

var setConcurrent = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "set concurrent of sync thread",
	},
	Options: []cmds.Option{
		cmds.Int64Option("concurrent", "coucurrent of sync thread"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("concurrent", true, false, "coucurrent of sync thread"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		concurrent, err := strconv.Atoi(req.Arguments[0])
		if err != nil {
			return cmds.ClientError("invalid number")
		}
		env.(*node.Env).SyncerAPI.SetConcurrent(req.Context, int64(concurrent)) //nolint
		return nil
	},
}

var storeStatusCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show status of chain sync operation.",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		tracker := env.(*node.Env).SyncerAPI.SyncerTracker(req.Context)
		targets := tracker.Buckets()
		w := bytes.NewBufferString("")
		writer := NewSilentWriter(w)
		var inSyncing []*syncTypes.Target
		var waitTarget []*syncTypes.Target

		for _, t := range targets {
			if t.State == syncTypes.StateInSyncing {
				inSyncing = append(inSyncing, t)
			} else {
				waitTarget = append(waitTarget, t)
			}
		}
		count := 1
		writer.Println("Syncing:")
		for _, t := range inSyncing {
			writer.Println("SyncTarget:", strconv.Itoa(count))
			writer.Println("\tBase:", t.Base.Height(), t.Base.Key().String())
			writer.Println("\tTarget:", t.Head.Height(), t.Head.Key().String())

			if t.Current != nil {
				writer.Println("\tCurrent:", t.Current.Height(), t.Current.Key().String())
			} else {
				writer.Println("\tCurrent:")
			}

			writer.Println("\tStatus:", t.State.String())
			writer.Println("\tErr:", t.Err)
			writer.Println()
			count++
		}

		writer.Println("Waiting:")
		for _, t := range waitTarget {
			writer.Println("SyncTarget:", strconv.Itoa(count))
			writer.Println("\tBase:", t.Base.Height(), t.Base.Key().String())
			writer.Println("\tTarget:", t.Head.Height(), t.Head.Key().String())

			if t.Current != nil {
				writer.Println("\tCurrent:", t.Current.Height(), t.Current.Key().String())
			} else {
				writer.Println("\tCurrent:")
			}

			writer.Println("\tStatus:", t.State.String())
			writer.Println("\tErr:", t.Err)
			writer.Println()
			count++
		}

		if err := re.Emit(w); err != nil {
			return err
		}
		return nil
	},
}

var historyCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show history of chain sync.",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		tracker := env.(*node.Env).SyncerAPI.SyncerTracker(req.Context)
		w := bytes.NewBufferString("")
		writer := NewSilentWriter(w)

		writer.Println("History:")
		history := tracker.History()
		count := 1
		for _, t := range history {
			writer.Println("SyncTarget:", strconv.Itoa(count))
			writer.Println("\tBase:", t.Base.Height(), t.Base.Key().String())

			writer.Println("\tTarget:", t.Head.Height(), t.Head.Key().String())

			if t.Current != nil {
				writer.Println("\tCurrent:", t.Current.Height(), t.Current.Key().String())
			} else {
				writer.Println("\tCurrent:")
			}
			writer.Println("\tTime:", t.End.Sub(t.Start).Milliseconds())
			writer.Println("\tStatus:", t.State.String())
			writer.Println("\tErr:", t.Err)
			writer.Println()
			count++
		}

		if err := re.Emit(w); err != nil {
			return err
		}
		return nil
	},
}

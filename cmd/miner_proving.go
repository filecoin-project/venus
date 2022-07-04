package cmd

import (
	"bytes"
	"fmt"
	"strconv"
	"text/tabwriter"

	"github.com/filecoin-project/go-address"
	cmds "github.com/ipfs/go-ipfs-cmds"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var minerProvingCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "View proving information.",
	},
	Subcommands: map[string]*cmds.Command{
		"info":      provingInfoCmd,
		"deadlines": provingDeadlinesCmd,
		"deadline":  provingDeadlineInfoCmd,
		"faults":    provingFaultsCmd,
	},
}

var provingInfoCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "View current state information.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Address of miner to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}
		ctx := req.Context

		blockDelay, err := blockDelay(req)
		if err != nil {
			return err
		}

		chainAPI := env.(*node.Env).ChainAPI
		head, err := chainAPI.ChainHead(ctx)
		if err != nil {
			return fmt.Errorf("getting chain head: %v", err)
		}

		mact, err := chainAPI.StateGetActor(ctx, maddr, head.Key())
		if err != nil {
			return err
		}

		stor := adt.WrapStore(ctx, cbor.NewCborStore(chain.NewAPIBlockstore(env.(*node.Env).BlockStoreAPI)))

		mas, err := miner.Load(stor, mact)
		if err != nil {
			return err
		}

		cd, err := chainAPI.StateMinerProvingDeadline(ctx, maddr, head.Key())
		if err != nil {
			return fmt.Errorf("getting miner info: %v", err)
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)
		writer.Printf("Miner: %s\n", maddr)

		proving := uint64(0)
		faults := uint64(0)
		recovering := uint64(0)
		curDeadlineSectors := uint64(0)

		if err := mas.ForEachDeadline(func(dlIdx uint64, dl miner.Deadline) error {
			return dl.ForEachPartition(func(partIdx uint64, part miner.Partition) error {
				if bf, err := part.LiveSectors(); err != nil {
					return err
				} else if count, err := bf.Count(); err != nil {
					return err
				} else {
					proving += count
					if dlIdx == cd.Index {
						curDeadlineSectors += count
					}
				}

				if bf, err := part.FaultySectors(); err != nil {
					return err
				} else if count, err := bf.Count(); err != nil {
					return err
				} else {
					faults += count
				}

				if bf, err := part.RecoveringSectors(); err != nil {
					return err
				} else if count, err := bf.Count(); err != nil {
					return err
				} else {
					recovering += count
				}

				return nil
			})
		}); err != nil {
			return fmt.Errorf("walking miner deadlines and partitions: %v", err)
		}

		var faultPerc float64
		if proving > 0 {
			faultPerc = float64(faults*10000/proving) / 100
		}

		writer.Printf("Current Epoch:           %d\n", cd.CurrentEpoch)

		writer.Printf("Proving Period Boundary: %d\n", cd.PeriodStart%cd.WPoStProvingPeriod)
		writer.Printf("Proving Period Start:    %s\n", EpochTime(cd.CurrentEpoch, cd.PeriodStart, blockDelay))
		writer.Printf("Next Period Start:       %s\n", EpochTime(cd.CurrentEpoch, cd.PeriodStart+cd.WPoStProvingPeriod, blockDelay))

		writer.Println()
		writer.Printf("Faults:      %d (%.2f%%)\n", faults, faultPerc)
		writer.Printf("Recovering:  %d\n", recovering)

		writer.Printf("Deadline Index:       %d\n", cd.Index)
		writer.Printf("Deadline Sectors:     %d\n", curDeadlineSectors)
		writer.Printf("Deadline Open:        %s\n", EpochTime(cd.CurrentEpoch, cd.Open, blockDelay))
		writer.Printf("Deadline Close:       %s\n", EpochTime(cd.CurrentEpoch, cd.Close, blockDelay))
		writer.Printf("Deadline Challenge:   %s\n", EpochTime(cd.CurrentEpoch, cd.Challenge, blockDelay))
		writer.Printf("Deadline FaultCutoff: %s\n", EpochTime(cd.CurrentEpoch, cd.FaultCutoff, blockDelay))

		return re.Emit(buf)
	},
}

var provingDeadlinesCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "View the current proving period deadlines information.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Address of miner to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}
		ctx := req.Context
		api := env.(*node.Env).ChainAPI

		deadlines, err := api.StateMinerDeadlines(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return fmt.Errorf("getting deadlines: %w", err)
		}

		di, err := api.StateMinerProvingDeadline(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return fmt.Errorf("getting deadlines: %w", err)
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)
		writer.Printf("Miner: %s\n", maddr)
		tw := tabwriter.NewWriter(buf, 2, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "deadline\tpartitions\tsectors (faults)\tproven partitions")

		for dlIdx, deadline := range deadlines {
			partitions, err := api.StateMinerPartitions(ctx, maddr, uint64(dlIdx), types.EmptyTSK)
			if err != nil {
				return fmt.Errorf("getting partitions for deadline %d: %w", dlIdx, err)
			}

			provenPartitions, err := deadline.PostSubmissions.Count()
			if err != nil {
				return err
			}

			sectors := uint64(0)
			faults := uint64(0)

			for _, partition := range partitions {
				sc, err := partition.AllSectors.Count()
				if err != nil {
					return err
				}

				sectors += sc

				fc, err := partition.FaultySectors.Count()
				if err != nil {
					return err
				}

				faults += fc
			}

			var cur string
			if di.Index == uint64(dlIdx) {
				cur += "\t(current)"
			}
			_, _ = fmt.Fprintf(tw, "%d\t%d\t%d (%d)\t%d%s\n", dlIdx, len(partitions), sectors, faults, provenPartitions, cur)
		}
		if err := tw.Flush(); err != nil {
			return err
		}

		return re.Emit(buf)
	},
}

var provingDeadlineInfoCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "View the current proving period deadlines information.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Address of miner to show"),
		cmds.StringArg("index", true, false, "Index of deadline to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 2 {
			return fmt.Errorf("must pass two parameters")
		}
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}
		ctx := req.Context
		api := env.(*node.Env).ChainAPI

		dlIdx, err := strconv.ParseUint(req.Arguments[1], 10, 64)
		if err != nil {
			return fmt.Errorf("could not parse deadline index: %w", err)
		}

		deadlines, err := api.StateMinerDeadlines(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return fmt.Errorf("getting deadlines: %w", err)
		}

		di, err := api.StateMinerProvingDeadline(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return fmt.Errorf("getting deadlines: %w", err)
		}

		partitions, err := api.StateMinerPartitions(ctx, maddr, dlIdx, types.EmptyTSK)
		if err != nil {
			return fmt.Errorf("getting partitions for deadline %d: %w", dlIdx, err)
		}

		provenPartitions, err := deadlines[dlIdx].PostSubmissions.Count()
		if err != nil {
			return err
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)

		writer.Printf("Deadline Index:           %d\n", dlIdx)
		writer.Printf("Partitions:               %d\n", len(partitions))
		writer.Printf("Proven Partitions:        %d\n", provenPartitions)
		writer.Printf("Current:                  %t\n\n", di.Index == dlIdx)

		for pIdx, partition := range partitions {
			sectorCount, err := partition.AllSectors.Count()
			if err != nil {
				return err
			}

			sectorNumbers, err := partition.AllSectors.All(sectorCount)
			if err != nil {
				return err
			}

			faultsCount, err := partition.FaultySectors.Count()
			if err != nil {
				return err
			}

			fn, err := partition.FaultySectors.All(faultsCount)
			if err != nil {
				return err
			}

			writer.Printf("Partition Index:          %d\n", pIdx)
			writer.Printf("Sectors:                  %d\n", sectorCount)
			writer.Printf("Sector Numbers:           %v\n", sectorNumbers)
			writer.Printf("Faults:                   %d\n", faultsCount)
			writer.Printf("Faulty Sectors:           %d\n", fn)
		}
		return re.Emit(buf)
	},
}

var provingFaultsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "View the currently known proving faulty sectors information.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Address of miner to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		ctx := req.Context
		api := env.(*node.Env).ChainAPI
		bstoreAPI := env.(*node.Env).BlockStoreAPI
		stor := adt.WrapStore(ctx, cbor.NewCborStore(chain.NewAPIBlockstore(bstoreAPI)))

		mact, err := api.StateGetActor(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		mas, err := miner.Load(stor, mact)
		if err != nil {
			return err
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)
		writer.Printf("Miner: %s\n", maddr)

		tw := tabwriter.NewWriter(buf, 2, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "deadline\tpartition\tsectors")
		err = mas.ForEachDeadline(func(dlIdx uint64, dl miner.Deadline) error {
			return dl.ForEachPartition(func(partIdx uint64, part miner.Partition) error {
				faults, err := part.FaultySectors()
				if err != nil {
					return err
				}
				return faults.ForEach(func(num uint64) error {
					_, _ = fmt.Fprintf(tw, "%d\t%d\t%d\n", dlIdx, partIdx, num)
					return nil
				})
			})
		})
		if err != nil {
			return err
		}
		if err := tw.Flush(); err != nil {
			return err
		}

		return re.Emit(buf)
	},
}

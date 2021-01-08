package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/filecoin-project/go-address"
	cmds "github.com/ipfs/go-ipfs-cmds"
	cbor "github.com/ipfs/go-ipld-cbor"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/specactors/adt"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"
)

var minerProvingCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with actors. Actors are built-in smart contracts.",
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

		blockDelay, err := blockDelay(env.(*node.Env).ConfigAPI)
		if err != nil {
			return err
		}

		chainAPI := env.(*node.Env).ChainAPI
		head, err := chainAPI.ChainHead(ctx)
		if err != nil {
			return xerrors.Errorf("getting chain head: %w", err)
		}

		mact, err := chainAPI.StateGetActor(ctx, maddr, head.Key())
		if err != nil {
			return err
		}

		stor := adt.WrapStore(ctx, cbor.NewCborStore(chain.NewAPIBlockstore(&env.(*node.Env).ChainAPI.DbAPI)))

		mas, err := miner.Load(stor, mact)
		if err != nil {
			return err
		}

		cd, err := chainAPI.StateMinerProvingDeadline(ctx, maddr, head.Key())
		if err != nil {
			return xerrors.Errorf("getting miner info: %w", err)
		}

		var r []string
		r = append(r, fmt.Sprintf("Miner: %s", maddr))

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
			return xerrors.Errorf("walking miner deadlines and partitions: %w", err)
		}

		var faultPerc float64
		if proving > 0 {
			faultPerc = float64(faults*10000/proving) / 100
		}

		r = append(r, fmt.Sprintf("Current Epoch:           %d", cd.CurrentEpoch))

		r = append(r, fmt.Sprintf("Proving Period Boundary: %d", cd.PeriodStart%cd.WPoStProvingPeriod),
			fmt.Sprintf("Proving Period Start:    %s", EpochTime(cd.CurrentEpoch, cd.PeriodStart, blockDelay)),
			fmt.Sprintf("Next Period Start:       %s", EpochTime(cd.CurrentEpoch, cd.PeriodStart+cd.WPoStProvingPeriod, blockDelay)))

		r = append(r, fmt.Sprintf("Faults:      %d (%.2f%%)", faults, faultPerc),
			fmt.Sprintf("Recovering:  %d", recovering))

		r = append(r, fmt.Sprintf("Deadline Index:       %d", cd.Index),
			fmt.Sprintf("Deadline Sectors:     %d", curDeadlineSectors),
			fmt.Sprintf("Deadline Open:        %s", EpochTime(cd.CurrentEpoch, cd.Open, blockDelay)),
			fmt.Sprintf("Deadline Close:       %s", EpochTime(cd.CurrentEpoch, cd.Close, blockDelay)),
			fmt.Sprintf("Deadline Challenge:   %s", EpochTime(cd.CurrentEpoch, cd.Challenge, blockDelay)),
			fmt.Sprintf("Deadline FaultCutoff: %s", EpochTime(cd.CurrentEpoch, cd.FaultCutoff, blockDelay)))

		return doEmit(re, r)
	},
	Type: nil,
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
		api := env.(node.Env).ChainAPI

		deadlines, err := api.StateMinerDeadlines(ctx, maddr, block.EmptyTSK)
		if err != nil {
			return xerrors.Errorf("getting deadlines: %w", err)
		}

		di, err := api.StateMinerProvingDeadline(ctx, maddr, block.EmptyTSK)
		if err != nil {
			return xerrors.Errorf("getting deadlines: %w", err)
		}

		buf := new(bytes.Buffer)
		buf.WriteString(fmt.Sprintf("Miner: %s\n", maddr))
		tw := tabwriter.NewWriter(buf, 2, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "deadline\tpartitions\tsectors (faults)\tproven partitions")

		for dlIdx, deadline := range deadlines {
			partitions, err := api.StateMinerPartitions(ctx, maddr, uint64(dlIdx), block.EmptyTSK)
			if err != nil {
				return xerrors.Errorf("getting partitions for deadline %d: %w", dlIdx, err)
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
		return re.Emit(buf.String())
	},
	Type: nil,
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
			return xerrors.Errorf("must pass two parameters")
		}
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}
		ctx := req.Context
		api := env.(node.Env).ChainAPI

		dlIdx, err := strconv.ParseUint(req.Arguments[1], 10, 64)
		if err != nil {
			return xerrors.Errorf("could not parse deadline index: %w", err)
		}

		deadlines, err := api.StateMinerDeadlines(ctx, maddr, block.EmptyTSK)
		if err != nil {
			return xerrors.Errorf("getting deadlines: %w", err)
		}

		di, err := api.StateMinerProvingDeadline(ctx, maddr, block.EmptyTSK)
		if err != nil {
			return xerrors.Errorf("getting deadlines: %w", err)
		}

		partitions, err := api.StateMinerPartitions(ctx, maddr, dlIdx, block.EmptyTSK)
		if err != nil {
			return xerrors.Errorf("getting partitions for deadline %d: %w", dlIdx, err)
		}

		provenPartitions, err := deadlines[dlIdx].PostSubmissions.Count()
		if err != nil {
			return err
		}

		var r []string
		r = append(r, fmt.Sprintf("Deadline Index:           %d", dlIdx),
			fmt.Sprintf("Partitions:               %d", len(partitions)),
			fmt.Sprintf("Proven Partitions:        %d", provenPartitions),
			fmt.Sprintf("Current:                  %t", di.Index == dlIdx))

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

			r = append(r, fmt.Sprintf("Partition Index:          %d", pIdx),
				fmt.Sprintf("Sectors:                  %d", sectorCount),
				fmt.Sprintf("Sector Numbers:           %v", sectorNumbers),
				fmt.Sprintf("Faults:                   %d", faultsCount),
				fmt.Sprintf("Faulty Sectors:           %d", fn))
		}
		return re.Emit(r)
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
		stor := adt.WrapStore(ctx, cbor.NewCborStore(chain.NewAPIBlockstore(&env.(*node.Env).ChainAPI.DbAPI)))

		mact, err := api.StateGetActor(ctx, maddr, block.EmptyTSK)
		if err != nil {
			return err
		}

		mas, err := miner.Load(stor, mact)
		if err != nil {
			return err
		}

		buf := new(bytes.Buffer)
		buf.WriteString(fmt.Sprintf("Miner: %s\n", color.BlueString("%s", maddr)))

		tw := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
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
		return re.Emit(buf.String())
	},
}

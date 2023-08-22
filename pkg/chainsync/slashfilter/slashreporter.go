package slashfilter

import (
	"context"
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-project/go-state-types/builtin/v8/miner"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/venus-shared/actors"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	levelds "github.com/ipfs/go-ds-leveldb"
	ldbopts "github.com/syndtr/goleveldb/leveldb/opt"
)

func SlashConsensus(ctx context.Context,
	cfg *config.FaultReporterConfig,
	walletAPI v1.IWallet,
	chainAPI v1.IChain,
	mpoolAPI v1.IMessagePool,
	syncAPI v1.ISyncer,
) error {
	ds, err := levelds.NewDatastore(cfg.ConsensusFaultReporterDataDir, &levelds.Options{
		Compression: ldbopts.NoCompression,
		NoSync:      false,
		Strict:      ldbopts.StrictAll,
		ReadOnly:    false,
	})
	if err != nil {
		return fmt.Errorf("open leveldb: %w", err)
	}
	sf := NewLocalSlashFilter(ds)

	var fromAddr address.Address
	if cfg.ConsensusFaultReporterAddress == "" {
		defaddr, err := walletAPI.WalletDefaultAddress(ctx)
		if err != nil {
			return err
		}
		fromAddr = defaddr
	} else {
		addr, err := address.NewFromString(cfg.ConsensusFaultReporterAddress)
		if err != nil {
			return err
		}

		fromAddr = addr
	}

	blocks, err := syncAPI.SyncIncomingBlocks(ctx)
	if err != nil {
		return fmt.Errorf("sync incoming blocks failed: %w", err)
	}

	log.Infow("consensus fault reporter", "from", fromAddr)
	go func() {
		for block := range blocks {
			otherBlock, extraBlock, fault, err := slashFilterMinedBlock(ctx, sf, chainAPI, block)
			if err != nil {
				log.Errorf("slash detector errored: %s", err)
				continue
			}
			if fault {
				log.Errorf("<!!> SLASH FILTER DETECTED FAULT DUE TO BLOCKS %s and %s", otherBlock.Cid(), block.Cid())
				bh1, err := cborutil.Dump(otherBlock)
				if err != nil {
					log.Errorf("could not dump otherblock:%s, err:%s", otherBlock.Cid(), err)
					continue
				}

				bh2, err := cborutil.Dump(block)
				if err != nil {
					log.Errorf("could not dump block:%s, err:%s", block.Cid(), err)
					continue
				}

				params := miner.ReportConsensusFaultParams{
					BlockHeader1: bh1,
					BlockHeader2: bh2,
				}
				if extraBlock != nil {
					be, err := cborutil.Dump(extraBlock)
					if err != nil {
						log.Errorf("could not dump block:%s, err:%s", block.Cid(), err)
						continue
					}
					params.BlockHeaderExtra = be
				}

				enc, err := actors.SerializeParams(&params)
				if err != nil {
					log.Errorf("could not serialize declare faults parameters: %s", err)
					continue
				}
				for {
					head, err := chainAPI.ChainHead(ctx)
					if err != nil || head.Height() > block.Height {
						break
					}
					time.Sleep(time.Second * 10)
				}
				message, err := mpoolAPI.MpoolPushMessage(ctx, &types.Message{
					To:     block.Miner,
					From:   fromAddr,
					Value:  types.NewInt(0),
					Method: builtin.MethodsMiner.ReportConsensusFault,
					Params: enc,
				}, nil)
				if err != nil {
					log.Errorf("ReportConsensusFault to messagepool error:%s", err)
					continue
				}
				log.Infof("ReportConsensusFault message CID:%s", message.Cid())
			}
		}
	}()

	return err
}

func slashFilterMinedBlock(ctx context.Context, sf ISlashFilter, chainAPI v1.IChain, blockB *types.BlockHeader) (*types.BlockHeader, *types.BlockHeader, bool, error) {
	blockC, err := chainAPI.ChainGetBlock(ctx, blockB.Parents[0])
	if err != nil {
		return nil, nil, false, fmt.Errorf("chain get block error:%s", err)
	}

	blockACid, fault, err := sf.MinedBlock(ctx, blockB, blockC.Height)
	if err != nil {
		return nil, nil, false, fmt.Errorf("slash filter check block error:%s", err)
	}

	if !fault {
		return nil, nil, false, nil
	}

	blockA, err := chainAPI.ChainGetBlock(ctx, blockACid)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get blockA: %w", err)
	}

	// (a) double-fork mining (2 blocks at one epoch)
	if blockA.Height == blockB.Height {
		return blockA, nil, true, nil
	}

	// (b) time-offset mining faults (2 blocks with the same parents)
	if types.CidArrsEqual(blockB.Parents, blockA.Parents) {
		return blockA, nil, true, nil
	}

	// (c) parent-grinding fault
	// Here extra is the "witness", a third block that shows the connection between A and B as
	// A's sibling and B's parent.
	// Specifically, since A is of lower height, it must be that B was mined omitting A from its tipset
	//
	//      B
	//      |
	//  [A, C]
	if types.CidArrsEqual(blockA.Parents, blockC.Parents) && blockA.Height == blockC.Height &&
		types.CidArrsContains(blockB.Parents, blockC.Cid()) && !types.CidArrsContains(blockB.Parents, blockA.Cid()) {
		return blockA, blockC, true, nil
	}

	log.Error("unexpectedly reached end of slashFilterMinedBlock despite fault being reported!")
	return nil, nil, false, nil
}

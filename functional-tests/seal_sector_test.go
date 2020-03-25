package functional

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	gengen "github.com/filecoin-project/go-filecoin/tools/gengen/util"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
)

func TestMiningSealSector(t *testing.T) {
	// tf.FunctionalTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wd, _ := os.Getwd()
	genCfgPath := filepath.Join(wd, "..", "fixtures/setup.json")
	presealPath := filepath.Join(wd, "..", "fixtures/genesis-sectors")
	genTime := int64(1000000000)
	blockTime := 1 * time.Second
	fakeClock := clock.NewFake(time.Unix(genTime, 0))

	// Load genesis config fixture.
	genCfg := loadGenesisConfig(t, genCfgPath)
	genCfg.Miners = append(genCfg.Miners, &gengen.CreateStorageMinerConfig{
		Owner:      1,
		SectorSize: 2048,
	})
	seed := node.MakeChainSeed(t, genCfg)
	chainClock := clock.NewChainClockFromClock(uint64(genTime), blockTime, fakeClock)

	bootstrapMiner := makeNode(ctx, t, seed, chainClock)
	_, _, err := initNodeGenesisMiner(t, bootstrapMiner, seed, genCfg.Miners[0].Owner, presealPath, genCfg.Miners[0].SectorSize)
	require.NoError(t, err)

	newMiner := makeNode(ctx, t, seed, chainClock)
	seed.GiveKey(t, newMiner, 1)
	_, _ = seed.GiveMiner(t, newMiner, 1)

	err = bootstrapMiner.Start(ctx)
	require.NoError(t, err)
	err = newMiner.Start(ctx)
	require.NoError(t, err)
	defer bootstrapMiner.Stop(ctx)
	defer newMiner.Stop(ctx)

	node.ConnectNodes(t, newMiner, bootstrapMiner)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				fakeClock.Advance(blockTime)
				_, err := bootstrapMiner.BlockMining.BlockMiningAPI.MiningOnce(ctx)
				require.NoError(t, err)
			}
			time.Sleep(time.Second)
		}
	}()

	err = newMiner.StorageMining.Start(ctx)
	require.NoError(t, err)

	err = newMiner.PieceManager().PledgeSector(ctx)
	require.NoError(t, err)

	for {
		ts, err := newMiner.PorcelainAPI.ChainHead()
		require.NoError(t, err)

		maddr, err := newMiner.BlockMining.BlockMiningAPI.MinerAddress()
		require.NoError(t, err)

		status, err := newMiner.PorcelainAPI.MinerGetStatus(ctx, maddr, ts.Key())
		fmt.Printf("%+v\n", status)

		time.Sleep(5 * time.Second)
	}
}

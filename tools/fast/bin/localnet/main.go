package main

// localnet
//
// localnet is a FAST binary tool that quickly, and easily, sets up a local network
// on the users computer. The network will stay standing till the program is closed.

import (
	"bytes"
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gx/ipfs/QmQmhotPUzVrMEWNK3x1R5jQ5ZHWyL7tVUrmRPjrBrvyCb/go-ipfs-files"
	bstore "gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
	"gx/ipfs/QmSz8kAe2JCKp2dWSG8gHSWnwSmne8YfRXTeK5HBmc9L7t/go-ipfs-exchange-offline"
	bserv "gx/ipfs/QmZsGVGCqMCNzHLNMB6q4F6yyvomqf1VxwhJwSfgo1NGaF/go-blockservice"
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	lpfc "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/local"
)

var (
	workdir         string
	shell           bool
	blocktime       = 5 * time.Second
	err             error
	fil             = 100000
	balance         big.Int
	smallSectors           = true
	minerCount             = 5
	minerPledge     uint64 = 10
	minerCollateral        = big.NewInt(500)
	minerPrice             = big.NewFloat(0.000000001)
	minerExpiry            = big.NewInt(24 * 60 * 60)

	sectorSize uint64

	exitcode int
)

func init() {

	logging.SetDebugLogging()

	var (
		err                error
		minerCollateralArg = minerCollateral.Text(10)
		minerPriceArg      = minerPrice.Text('f', 10)
		minerExpiryArg     = minerExpiry.Text(10)
	)

	flag.StringVar(&workdir, "workdir", workdir, "set the working directory used to store filecoin repos")
	flag.BoolVar(&shell, "shell", shell, "drop into a shell")
	flag.BoolVar(&smallSectors, "small-sectors", smallSectors, "enables small sectors")
	flag.DurationVar(&blocktime, "blocktime", blocktime, "duration for blocktime")
	flag.IntVar(&minerCount, "miner-count", minerCount, "number of miners")
	flag.Uint64Var(&minerPledge, "miner-pledge", minerPledge, "number of sectors to pledge for each miner")
	flag.StringVar(&minerCollateralArg, "miner-collateral", minerCollateralArg, "amount of fil each miner will use for collateral")
	flag.StringVar(&minerPriceArg, "miner-price", minerPriceArg, "price value used when creating ask for miners")
	flag.StringVar(&minerExpiryArg, "miner-expiry", minerExpiryArg, "expiry value used when creating ask for miners")

	_, ok := minerCollateral.SetString(minerCollateralArg, 10)
	if !ok {
		handleError(fmt.Errorf("could not parse miner-collateral"))
		os.Exit(1)
	}

	_, ok = minerPrice.SetString(minerPriceArg)
	if !ok {
		handleError(fmt.Errorf("could not parse miner-price"))
		os.Exit(1)
	}

	_, ok = minerExpiry.SetString(minerExpiryArg, 10)
	if !ok {
		handleError(fmt.Errorf("could not parse miner-expiry"))
		os.Exit(1)
	}

	flag.Parse()

	// Set the series global sleep delay to our blocktime
	series.GlobalSleepDelay = blocktime

	sectorSize, err = getSectorSize(smallSectors)
	if err != nil {
		handleError(err)
		os.Exit(1)
	}

	// Set the initial balance
	balance.SetInt64(int64(100 * fil))
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	exit := make(chan struct{}, 1)

	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		<-signals
		fmt.Println("Ctrl-C received, starting shutdown")
		cancel()
		exit <- struct{}{}
	}()

	defer func() {
		os.Exit(exitcode)
	}()

	if len(workdir) == 0 {
		workdir, err = ioutil.TempDir("", "localnet")
		if err != nil {
			exitcode = handleError(err)
			return
		}
	}

	if ok, err := isEmpty(workdir); !ok {
		if err == nil {
			err = fmt.Errorf("workdir is not empty: %s", workdir)
		}

		exitcode = handleError(err, "fail when checking workdir;")
		return
	}

	env, err := fast.NewEnvironmentMemoryGenesis(&balance, workdir)
	if err != nil {
		exitcode = handleError(err)
		return
	}

	// Defer the teardown, this will shuteverything down for us
	defer env.Teardown(ctx) // nolint: errcheck

	binpath, err := testhelpers.GetFilecoinBinary()
	if err != nil {
		exitcode = handleError(err, "no binary was found, please build go-filecoin;")
		return
	}

	// Setup localfilecoin plugin options
	options := make(map[string]string)
	options[lpfc.AttrLogJSON] = "0"                                     // Disable JSON logs
	options[lpfc.AttrLogLevel] = "4"                                    // Set log level to Info
	options[lpfc.AttrUseSmallSectors] = fmt.Sprintf("%t", smallSectors) // Enable small sectors
	options[lpfc.AttrFilecoinBinary] = binpath                          // Use the repo binary

	genesisURI := env.GenesisCar()
	genesisMiner, err := env.GenesisMiner()
	if err != nil {
		exitcode = handleError(err, "failed to retrieve miner information from genesis;")
		return
	}

	fastenvOpts := fast.EnvironmentOpts{
		InitOpts:   []fast.ProcessInitOption{fast.POGenesisFile(genesisURI)},
		DaemonOpts: []fast.ProcessDaemonOption{fast.POBlockTime(series.GlobalSleepDelay)},
	}

	// The genesis process is the filecoin node that loads the miner that is
	// define with power in the genesis block, and the prefunnded wallet
	genesis, err := env.NewProcess(ctx, lpfc.PluginName, options, fastenvOpts)
	if err != nil {
		exitcode = handleError(err, "failed to create genesis process;")
		return
	}

	err = series.SetupGenesisNode(ctx, genesis, genesisMiner.Address, files.NewReaderFile(genesisMiner.Owner))
	if err != nil {
		exitcode = handleError(err, "failed series.SetupGenesisNode;")
		return
	}

	// Create the processes that we will use to become miners
	var miners []*fast.Filecoin
	for i := 0; i < minerCount; i++ {
		miner, err := env.NewProcess(ctx, lpfc.PluginName, options, fastenvOpts)
		if err != nil {
			exitcode = handleError(err, "failed to create miner process;")
			return
		}

		miners = append(miners, miner)
	}

	// We will now go through the process of creating miners
	// InitAndStart
	// 1. Initialize node
	// 2. Start daemon
	//
	// Connect
	// 3. Connect to genesis
	//
	// SendFilecoinDefaults
	// 4. Issue FIL to node
	//
	// CreateMinerWithAsk
	// 5. Create a new miner
	// 6. Set the miner price, and get ask
	//
	// ImportAndStore
	// 7. Generated some random data and import it to genesis
	// 8. Genesis propposes a storage deal with miner
	//
	// WaitForDealState
	// 9. Query deal till posted

	var deals []*storagedeal.Response

	for _, miner := range miners {
		err = series.InitAndStart(ctx, miner)
		if err != nil {
			exitcode = handleError(err, "failed series.InitAndStart;")
			return
		}

		err = series.Connect(ctx, genesis, miner)
		if err != nil {
			exitcode = handleError(err, "failed series.Connect;")
			return
		}

		err = series.SendFilecoinDefaults(ctx, genesis, miner, fil)
		if err != nil {
			exitcode = handleError(err, "failed series.SendFilecoinDefaults;")
			return
		}

		ask, err := series.CreateMinerWithAsk(ctx, miner, minerPledge, minerCollateral, minerPrice, minerExpiry)
		if err != nil {
			exitcode = handleError(err, "failed series.CreateMinerWithAsk;")
			return
		}

		var data bytes.Buffer
		dataReader := io.LimitReader(rand.Reader, int64(sectorSize))
		dataReader = io.TeeReader(dataReader, &data)
		_, deal, err := series.ImportAndStore(ctx, genesis, ask, files.NewReaderFile(dataReader))
		if err != nil {
			exitcode = handleError(err, "failed series.ImportAndStore;")
			return
		}

		deals = append(deals, deal)

	}

	for _, deal := range deals {
		err = series.WaitForDealState(ctx, genesis, deal, storagedeal.Posted)
		if err != nil {
			exitcode = handleError(err, "failed series.WaitForDealState;")
			return
		}
	}

	if shell {
		client, err := env.NewProcess(ctx, lpfc.PluginName, options, fastenvOpts)
		if err != nil {
			exitcode = handleError(err, "failed to create client process;")
			return
		}

		err = series.InitAndStart(ctx, client)
		if err != nil {
			exitcode = handleError(err, "failed series.InitAndStart;")
			return
		}

		err = series.Connect(ctx, genesis, client)
		if err != nil {
			exitcode = handleError(err, "failed series.Connect;")
			return
		}

		err = series.SendFilecoinDefaults(ctx, genesis, client, fil)
		if err != nil {
			exitcode = handleError(err, "failed series.SendFilecoinDefaults;")
			return
		}

		interval, err := client.StartLogCapture()
		if err != nil {
			exitcode = handleError(err, "failed to start log capture;")
			return
		}

		if err := client.Shell(); err != nil {
			exitcode = handleError(err, "failed to run client shell;")
			return
		}

		interval.Stop()
		fmt.Println("===================================")
		fmt.Println("===================================")
		io.Copy(os.Stdout, interval) // nolint: errcheck
		fmt.Println("===================================")
		fmt.Println("===================================")
	}

	fmt.Println("Finished!")
	fmt.Println("Ctrl-C to exit")

	<-exit
}

func handleError(err error, msg ...string) int {
	if err == nil {
		return 0
	}

	if len(msg) != 0 {
		fmt.Println(msg[0], err)
	} else {
		fmt.Println(err)
	}

	return 1
}

// https://stackoverflow.com/a/30708914
func isEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close() // nolint: errcheck

	_, err = f.Readdirnames(1) // Or f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err // Either not empty or error, suits both cases
}

func getSectorSize(smallSectors bool) (uint64, error) {
	rp := repo.NewInMemoryRepo()
	blockstore := bstore.NewBlockstore(rp.Datastore())
	blockservice := bserv.New(blockstore, offline.Exchange(blockstore))

	var sectorStoreType proofs.SectorStoreType

	if smallSectors {
		sectorStoreType = proofs.Test
	} else {
		sectorStoreType = proofs.Live
	}

	// Not a typo
	addr := address.NewForTestGetter()()

	cfg := sectorbuilder.RustSectorBuilderConfig{
		BlockService:     blockservice,
		LastUsedSectorID: 0,
		MetadataDir:      "",
		MinerAddr:        addr,
		SealedSectorDir:  "",
		SectorStoreType:  sectorStoreType,
		StagedSectorDir:  "",
	}

	sb, err := sectorbuilder.NewRustSectorBuilder(cfg)
	if err != nil {
		return 0, err
	}

	return sb.GetMaxUserBytesPerStagedSector()
}

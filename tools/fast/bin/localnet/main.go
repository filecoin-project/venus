package main

// localnet
//
// localnet is a FAST binary tool that quickly and easily, sets up a local network
// on the users computer. The network will stay standing till the program is closed.

import (
	"bytes"
	"context"
	"crypto/rand"
	flg "flag"
	"fmt"
	"github.com/filecoin-project/go-filecoin/types"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/ipfs/go-ipfs-files"
	logging "github.com/ipfs/go-log"
	"github.com/mitchellh/go-homedir"

	"github.com/filecoin-project/go-filecoin/proofs/libsectorbuilder"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/environment"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	lpfc "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/local"
)

var (
	workdir         string
	binpath         string
	shell           bool
	blocktime       = 5 * time.Second
	err             error
	fil             = 100000
	balance         big.Int
	smallSectors    = true
	minerCount      = 5
	minerCollateral = big.NewInt(500)
	minerPrice      = big.NewFloat(0.000000001)
	minerExpiry     = big.NewInt(24 * 60 * 60)

	exitcode int

	flag = flg.NewFlagSet(os.Args[0], flg.ExitOnError)
)

func init() {

	logging.SetDebugLogging()

	var (
		err                error
		minerCollateralArg = minerCollateral.Text(10)
		minerPriceArg      = minerPrice.Text('f', 10)
		minerExpiryArg     = minerExpiry.Text(10)
	)

	// We default to the binary built in the project directory, fallback
	// to searching path.
	binpath, err = getFilecoinBinary()
	if err != nil {
		// Look for `go-filecoin` in the path to set `binpath` default
		// If the binary is not found, an error will be returned. If the
		// error is ErrNotFound we ignore it.
		// Error is handled after flag parsing so help can be shown without
		// erroring first
		binpath, err = exec.LookPath("go-filecoin")
		if err != nil {
			xerr, ok := err.(*exec.Error)
			if ok && xerr.Err == exec.ErrNotFound {
				err = nil
			}
		}
	}

	flag.StringVar(&workdir, "workdir", workdir, "set the working directory used to store filecoin repos")
	flag.StringVar(&binpath, "binpath", binpath, "set the binary used when executing `go-filecoin` commands")
	flag.BoolVar(&shell, "shell", shell, "setup a filecoin client node and enter into a shell ready to use")
	flag.BoolVar(&smallSectors, "small-sectors", smallSectors, "enables small sectors")
	flag.DurationVar(&blocktime, "blocktime", blocktime, "duration for blocktime")
	flag.IntVar(&minerCount, "miner-count", minerCount, "number of miners")
	flag.StringVar(&minerCollateralArg, "miner-collateral", minerCollateralArg, "amount of fil each miner will use for collateral")
	flag.StringVar(&minerPriceArg, "miner-price", minerPriceArg, "price value used when creating ask for miners")
	flag.StringVar(&minerExpiryArg, "miner-expiry", minerExpiryArg, "expiry value used when creating ask for miners")

	// ExitOnError is set
	flag.Parse(os.Args[1:]) // nolint: errcheck

	// If we failed to find `go-filecoin` and it was not set, handle the error
	if len(binpath) == 0 {
		msg := "failed when checking for `go-filecoin` binary;"
		if err == nil {
			err = fmt.Errorf("no binary provided or found")
			msg = "please install or build `go-filecoin`;"
		}

		handleError(err, msg)
		os.Exit(1)
	}

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
		if r := recover(); r != nil {
			fmt.Println("recovered from panic", r)
			fmt.Println("stacktrace from panic: \n" + string(debug.Stack()))
			exitcode = 1
		}
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

	env, err := environment.NewMemoryGenesis(&balance, workdir, getProofsMode(smallSectors))
	if err != nil {
		exitcode = handleError(err)
		return
	}

	// Defer the teardown, this will shuteverything down for us
	defer env.Teardown(ctx) // nolint: errcheck

	// Setup localfilecoin plugin options
	options := make(map[string]string)
	options[lpfc.AttrLogJSON] = "0"            // Disable JSON logs
	options[lpfc.AttrLogLevel] = "4"           // Set log level to Info
	options[lpfc.AttrFilecoinBinary] = binpath // Use the repo binary

	genesisURI := env.GenesisCar()
	genesisMiner, err := env.GenesisMiner()
	if err != nil {
		exitcode = handleError(err, "failed to retrieve miner information from genesis;")
		return
	}

	fastenvOpts := fast.FilecoinOpts{
		InitOpts:   []fast.ProcessInitOption{fast.POGenesisFile(genesisURI)},
		DaemonOpts: []fast.ProcessDaemonOption{fast.POBlockTime(blocktime)},
	}

	ctx = series.SetCtxSleepDelay(ctx, blocktime)

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

	if err := genesis.MiningStart(ctx); err != nil {
		exitcode = handleError(err, "failed to start mining on genesis node;")
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
	// CreateStorageMinerWithAsk
	// 5. Create a new miner
	// 6. Set the miner price, and get ask
	//
	// ImportAndStore
	// 7. Generated some random data and import it to genesis
	// 8. Genesis proposes a storage deal with miner
	//
	// WaitForDealState
	// 9. Query deal till complete

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

		ask, err := series.CreateStorageMinerWithAsk(ctx, miner, minerCollateral, minerPrice, minerExpiry)
		if err != nil {
			exitcode = handleError(err, "failed series.CreateStorageMinerWithAsk;")
			return
		}

		var data bytes.Buffer
		dataReader := io.LimitReader(rand.Reader, int64(getMaxUserBytesPerStagedSector()))
		dataReader = io.TeeReader(dataReader, &data)
		_, deal, err := series.ImportAndStore(ctx, genesis, ask, files.NewReaderFile(dataReader))
		if err != nil {
			exitcode = handleError(err, "failed series.ImportAndStore;")
			return
		}

		deals = append(deals, deal)
	}

	for _, deal := range deals {
		err = series.WaitForDealState(ctx, genesis, deal, storagedeal.Complete)
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

func getMaxUserBytesPerStagedSector() uint64 {
	return libsectorbuilder.GetMaxUserBytesPerStagedSector(types.OneKiBSectorSize.Uint64())
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

func getProofsMode(smallSectors bool) types.ProofsMode {
	if smallSectors {
		return types.TestProofsMode
	}
	return types.LiveProofsMode
}

func getFilecoinBinary() (string, error) {
	gopath, err := getGoPath()
	if err != nil {
		return "", err
	}

	bin := filepath.Join(gopath, "/src/github.com/filecoin-project/go-filecoin/go-filecoin")
	_, err = os.Stat(bin)
	if err != nil {
		return "", err
	}

	if os.IsNotExist(err) {
		return "", err
	}

	return bin, nil
}

func getGoPath() (string, error) {
	gp := os.Getenv("GOPATH")
	if gp != "" {
		return gp, nil
	}

	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, "go"), nil
}

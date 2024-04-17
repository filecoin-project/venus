package constants

import (
	"fmt"
	"os"
	"strconv"
)

// FevmEnableEthRPC enables eth rpc, and enables storing a mapping of eth transaction hashes to filecoin message Cids.
var FevmEnableEthRPC = os.Getenv("VENUS_FEVM_ENABLEETHRPC") == "1"

// InsecurePoStValidation use to attach debug
var InsecurePoStValidation = os.Getenv("INSECURE_POST_VALIDATION") == "1"

// NoSlashFilter will not check whether the miner's block violates the consensus
var NoSlashFilter = os.Getenv("VENUS_NO_SLASHFILTER") == "_yes_i_know_and_i_accept_that_may_loss_my_fil"

// NoMigrationResultCache will not use cached migration results
var NoMigrationResultCache = os.Getenv("VENUS_NO_MIGRATION_RESULT_CACHE") == "1"

// SyncForkTimeout in seconds, default is 5 minutes
var SyncForkTimeout = 5 * 60

func init() {
	syncForkTimeoutStr := os.Getenv("VENUS_SYNC_FORK_TIMEOUT")
	if len(syncForkTimeoutStr) > 0 {
		var err error
		SyncForkTimeout, err = strconv.Atoi(syncForkTimeoutStr)
		if err != nil {
			panic(fmt.Errorf("failed to parse VENUS_SYNC_FORK_TIMEOUT: %s, error %w", syncForkTimeoutStr, err))
		} else {
			fmt.Println("set SyncForkTimeout: ", SyncForkTimeout)
		}
	}
}

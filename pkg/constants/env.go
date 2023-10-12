package constants

import "os"

// FevmEnableEthRPC enables eth rpc, and enables storing a mapping of eth transaction hashes to filecoin message Cids.
var FevmEnableEthRPC = os.Getenv("VENUS_FEVM_ENABLEETHRPC") == "1"

// InsecurePoStValidation use to attach debug
var InsecurePoStValidation = os.Getenv("INSECURE_POST_VALIDATION") == "1"

// NoSlashFilter will not check whether the miner's block violates the consensus
var NoSlashFilter = os.Getenv("VENUS_NO_SLASHFILTER") == "_yes_i_know_and_i_accept_that_may_loss_my_fil"

// NoMigrationResultCache will not use cached migration results
var NoMigrationResultCache = os.Getenv("VENUS_NO_MIGRATION_RESULT_CACHE") == "1"

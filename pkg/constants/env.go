package constants

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
)

// FevmEnableEthRPC enables eth rpc, and enables storing a mapping of eth transaction hashes to filecoin message Cids.
var FevmEnableEthRPC = os.Getenv("VENUS_FEVM_ENABLEETHRPC") == "1"

// InsecurePoStValidation use to attach debug
var InsecurePoStValidation = os.Getenv("INSECURE_POST_VALIDATION") == "1"

// NoSlashFilter will not check whether the miner's block violates the consensus
var NoSlashFilter = os.Getenv("VENUS_NO_SLASHFILTER") == "_yes_i_know_and_i_accept_that_may_loss_my_fil"

// NoMigrationResultCache will not use cached migration results
var NoMigrationResultCache = os.Getenv("VENUS_NO_MIGRATION_RESULT_CACHE") == "1"

// DisableF3 disable f3
var DisableF3 = os.Getenv("VENUS_DISABLE_F3") == "1"

const F3DisableActivation = "VENUS_DISABLE_F3_ACTIVATION"

func parseF3DisableActivationEnv() (contractAddrs []string, epochs []int64) {
	v, envVarSet := os.LookupEnv(F3DisableActivation)
	if !envVarSet || strings.TrimSpace(v) == "" {
		// Environment variable is not set or empty, activation is not disabled
		return
	}

	// Parse the variable which can be in format "contract:addrs" or "epoch:epochnumber" or both
	parts := strings.Split(v, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}

		key := strings.TrimSpace(strings.ToLower(kv[0]))
		value := strings.TrimSpace(kv[1])

		switch key {
		case "contract":
			// If contract address matches, disable activation
			contractAddrs = append(contractAddrs, value)
		case "epoch":
			parsedEpoch, err := strconv.ParseInt(value, 10, 64)
			if err == nil {
				epochs = append(epochs, parsedEpoch)
			} else {
				fmt.Printf("error parsing %s env variable, cannot parse epoch\n", F3DisableActivation)
			}
		}
	}
	return contractAddrs, epochs
}

// IsF3EpochActivationDisabled checks if F3 activation is disabled for the given
// epoch number based on environment variable configuration.
func IsF3EpochActivationDisabled(epoch int64) bool {
	_, epochs := parseF3DisableActivationEnv()
	return slices.Contains(epochs, epoch)
}

// IsF3ContractActivationDisabled checks if F3 activation is disabled for the given contract address
// based on environment variable configuration.
func IsF3ContractActivationDisabled(contract string) bool {
	contracts, _ := parseF3DisableActivationEnv()
	return slices.Contains(contracts, contract)
}

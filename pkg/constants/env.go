package constants

import "os"

// FevmEnableEthRPC enables eth rpc, and enables storing a mapping of eth transaction hashes to filecoin message Cids.
var FevmEnableEthRPC = os.Getenv("VENUS_FEVM_ENABLEETHRPC") == "1"

// InsecurePoStValidation use to attach debug
var InsecurePoStValidation = os.Getenv("INSECURE_POST_VALIDATION") == "1"

package constants

import "os"

// FevmEnableEthRPC enables eth rpc, and enables storing a mapping of eth transaction hashes to filecoin message Cids.
var FevmEnableEthRPC = os.Getenv("VENUS_FEVM_ENABLEETHRPC") == "1"

package types

import (
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"

	"github.com/filecoin-project/go-filecoin/address"
	wutil "github.com/filecoin-project/go-filecoin/wallet/util"
)

var log = logging.Logger("types")

// Signature is the result of a cryptographic sign operation.
type Signature []byte

// IsValidSignature cryptographically verifies that 'sig' is the signed hash of 'data' with
// the public key belonging to `addr`.
func IsValidSignature(data []byte, addr address.Address, sig Signature) bool {
	maybePk, err := wutil.Ecrecover(data, sig)
	if err != nil {
		// Any error returned from Ecrecover means this signature is not valid.
		log.Infof("error in signature validation: %s", err)
		return false
	}
	maybeAddr, err := address.NewSecp256k1Address(maybePk)
	if err != nil {
		log.Infof("error in recovered address: %s", err)
		return false
	}

	return maybeAddr == addr
}

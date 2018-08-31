package types

import (
	logging "gx/ipfs/QmRREK2CAZ5Re2Bd9zZFG6FeYDppUWt5cMgsoUEp3ktgSr/go-log"

	wutil "github.com/filecoin-project/go-filecoin/wallet/util"
)

var log = logging.Logger("types")

// Signature is the result of a cryptographic sign operation.
type Signature = Bytes

// VerifySignature cryptographically verifies that 'sig' is the signed hash of 'data' with
// the public key belonging to `addr`.
func VerifySignature(data []byte, addr Address, sig Signature) bool {
	maybePk, err := wutil.Ecrecover(data, sig)
	if err != nil {
		// Any error returned from Ecrecover means this signature is not valid.
		log.Infof("error in signature validation: %s", err)
		return false
	}
	maybeAddrHash := AddressHash(maybePk)

	return NewMainnetAddress(maybeAddrHash) == addr
}

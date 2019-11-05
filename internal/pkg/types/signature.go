package types

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	logging "github.com/ipfs/go-log"
	"github.com/minio/blake2b-simd"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

var log = logging.Logger("types")

// Signature is the result of a cryptographic sign operation.
type Signature []byte

// IsValidSignature cryptographically verifies that 'sig' is the signed hash of 'data' with
// the public key belonging to `addr`.
func IsValidSignature(data []byte, addr address.Address, sig Signature) bool {
	switch addr.Protocol() {
	case address.SECP256K1:
		return isValidSecpSignature(data, addr, sig)
	case address.BLS:
		return isValidBLSSignature(data, addr, sig)
	default:
		return false
	}
}

func isValidSecpSignature(data []byte, addr address.Address, signature Signature) bool {
	hash := blake2b.Sum256(data)
	maybePk, err := crypto.EcRecover(hash[:], signature)
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

func isValidBLSSignature(data []byte, addr address.Address, signature Signature) bool {
	return crypto.VerifyBLS(addr.Payload(), data, signature[:])
}

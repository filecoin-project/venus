package crypto

import (
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	log "github.com/ipfs/go-log"
	"github.com/minio/blake2b-simd"
)

//
// Address-based signature validation
//

var lg = log.Logger("crypto")

type Signature = crypto.Signature
type SigType = crypto.SigType

const (
	SigTypeSecp256k1 = crypto.SigTypeSecp256k1
	SigTypeBLS       = crypto.SigTypeBLS
)

func Sign(data []byte, secretKey []byte, sigtype SigType) (Signature, error) {
	var signature []byte
	var err error
	if sigtype == SigTypeSecp256k1 {
		hash := blake2b.Sum256(data)
		signature, err = SignSecp(secretKey, hash[:])
	} else if sigtype == SigTypeBLS {
		signature, err = SignBLS(secretKey, data)
	} else {
		err = fmt.Errorf("unknown signature type %d", sigtype)
	}
	return Signature{
		Type: sigtype,
		Data: signature,
	}, err
}

// IsValidSignature cryptographically verifies that 'sig' is the signed hash of 'data' with
// the public key belonging to `addr`.
func IsValidSignature(data []byte, addr address.Address, sig Signature) bool {
	switch addr.Protocol() {
	case address.SECP256K1:
		return sig.Type == SigTypeSecp256k1 && IsValidSecpSignature(data, addr, sig.Data)
	case address.BLS:
		return sig.Type == SigTypeBLS && IsValidBLSSignature(data, addr, sig.Data)
	default:
		return false
	}
}

func IsValidSecpSignature(data []byte, addr address.Address, signature []byte) bool {
	hash := blake2b.Sum256(data)
	maybePk, err := EcRecover(hash[:], signature)
	if err != nil {
		// Any error returned from Ecrecover means this signature is not valid.
		lg.Infof("error in signature validation: %s", err)
		return false
	}
	maybeAddr, err := address.NewSecp256k1Address(maybePk)
	if err != nil {
		lg.Infof("error in recovered address: %s", err)
		return false
	}
	return maybeAddr == addr
}

func IsValidBLSSignature(data []byte, addr address.Address, signature []byte) bool {
	if addr.Protocol() != address.BLS {
		return false
	}
	return VerifyBLS(addr.Payload(), data, signature)
}

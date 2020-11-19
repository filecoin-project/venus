package crypto

import (
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/minio/blake2b-simd"
)

//
// Address-based signature validation
//

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

// ValidateSignature cryptographically verifies that 'sig' is the signed hash of 'data' with
// the public key belonging to `addr`.
func ValidateSignature(data []byte, addr address.Address, sig Signature) error {
	switch addr.Protocol() {
	case address.SECP256K1:
		if sig.Type != SigTypeSecp256k1 {
			return fmt.Errorf("incorrect signature type (%v) for address expected SECP256K1 signature", sig.Type)
		}
		return ValidateSecpSignature(data, addr, sig.Data)
	case address.BLS:
		if sig.Type != SigTypeBLS {
			return fmt.Errorf("incorrect signature type (%v) for address expected BLS signature", sig.Type)
		}
		return ValidateBlsSignature(data, addr, sig.Data)
	default:
		return fmt.Errorf("incorrect address protocol (%v) for signature validation", addr.Protocol())
	}
}

func ValidateSecpSignature(data []byte, addr address.Address, signature []byte) error {
	if addr.Protocol() != address.SECP256K1 {
		return fmt.Errorf("address protocol (%v) invalid for SECP256K1 signature verification", addr.Protocol())
	}
	hash := blake2b.Sum256(data)
	maybePk, err := EcRecover(hash[:], signature)
	if err != nil {
		return err
	}
	maybeAddr, err := address.NewSecp256k1Address(maybePk)
	if err != nil {
		return err
	}
	if maybeAddr != addr {
		return fmt.Errorf("invalid SECP signature")
	}
	return nil
}

func ValidateBlsSignature(data []byte, addr address.Address, signature []byte) error {
	if addr.Protocol() != address.BLS {
		return fmt.Errorf("address protocol (%v) invalid for BLS signature verification", addr.Protocol())
	}
	if valid := VerifyBLS(addr.Payload(), data, signature); !valid {
		return fmt.Errorf("invalid BLS signature")
	}
	return nil
}

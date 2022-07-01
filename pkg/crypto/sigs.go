package crypto

import (
	"fmt"
	"io"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/crypto"
)

//
// address-based signature validation
//

type Signature = crypto.Signature
type SigType = crypto.SigType

const (
	SigTypeSecp256k1 = crypto.SigTypeSecp256k1
	SigTypeBLS       = crypto.SigTypeBLS
)

const (
	BLSSignatureBytes = 96
)

// Sign takes in signature type, private key and message. Returns a signature for that message.
// Valid sigTypes are: "secp256k1" and "bls"
func Sign(msg []byte, privkey []byte, sigType SigType) (*crypto.Signature, error) {
	sv, ok := sigs[sigType]
	if !ok {
		return nil, fmt.Errorf("cannot sign message with signature of unsupported type: %v", sigType)
	}

	sb, err := sv.Sign(privkey, msg)
	if err != nil {
		return nil, err
	}
	return &crypto.Signature{
		Type: sigType,
		Data: sb,
	}, nil
}

// Verify verifies signatures
func Verify(sig *crypto.Signature, addr address.Address, msg []byte) error {
	if sig == nil {
		return fmt.Errorf("signature is nil")
	}

	if addr.Protocol() == address.ID {
		return fmt.Errorf("must resolve ID addresses before using them to verify a signature")
	}

	sv, ok := sigs[sig.Type]
	if !ok {
		return fmt.Errorf("cannot verify signature of unsupported type: %v", sig.Type)
	}

	return sv.Verify(sig.Data, addr, msg)
}

func VerifyAggregate(pubKeys, msgs [][]byte, signature []byte) error {
	if signature == nil {
		return fmt.Errorf("signature is nil")
	}

	sv, ok := sigs[crypto.SigTypeBLS]
	if !ok {
		return fmt.Errorf("bls not register")
	}

	if !sv.VerifyAggregate(pubKeys, msgs, signature) {
		return fmt.Errorf("verify aggregate message fail")
	}
	return nil
}

// Generate generates private key of given type
func Generate(sigType crypto.SigType) ([]byte, error) {
	sv, ok := sigs[sigType]
	if !ok {
		return nil, fmt.Errorf("cannot generate private key of unsupported type: %v", sigType)
	}

	return sv.GenPrivate()
}

// ToPublic converts private key to public key
func ToPublic(sigType crypto.SigType, pk []byte) ([]byte, error) {
	sv, ok := sigs[sigType]
	if !ok {
		return nil, fmt.Errorf("cannot generate public key of unsupported type: %v", sigType)
	}

	return sv.ToPublic(pk)
}

// SigShim is used for introducing signature functions
type SigShim interface {
	GenPrivate() ([]byte, error)
	GenPrivateFromSeed(seed io.Reader) ([]byte, error)
	ToPublic(pk []byte) ([]byte, error)
	Sign(pk []byte, msg []byte) ([]byte, error)
	Verify(sig []byte, a address.Address, msg []byte) error
	VerifyAggregate(pubKeys, msgs [][]byte, signature []byte) bool
}

var sigs map[crypto.SigType]SigShim

// RegisterSignature should be only used during init
func RegisterSignature(typ crypto.SigType, vs SigShim) {
	if sigs == nil {
		sigs = make(map[crypto.SigType]SigShim)
	}
	sigs[typ] = vs
}

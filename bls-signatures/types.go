package bls

// SignatureBytes is the length of a BLS signature
const SignatureBytes = 96

// PrivateKeyBytes is the length of a BLS private key
const PrivateKeyBytes = 32

// PublicKeyBytes is the length of a BLS public key
const PublicKeyBytes = 48

// DigestBytes is the length of a BLS message hash/digest
const DigestBytes = 96

// Signature is a compressed affine
type Signature [SignatureBytes]byte

// PrivateKey is a compressed affine
type PrivateKey [PrivateKeyBytes]byte

// PublicKey is a compressed affine
type PublicKey [PublicKeyBytes]byte

// Message is a byte array
type Message []byte

// Digest is a compressed affine
type Digest [DigestBytes]byte

// HashRequest contain the message to compute the digest of
type HashRequest struct {
	message Message
}

// HashResponse contains the computed digest of a message
type HashResponse struct {
	digest Digest
}

// VerifyRequest contains the signature to match against an pairing of digests
// to public keys
type VerifyRequest struct {
	signature  Signature
	digests    []Digest
	publicKeys []PublicKey
}

// VerifyResponse contains the result of a verification check
type VerifyResponse struct {
	result bool
}

// AggregateRequest contains the signatures to aggregate
type AggregateRequest struct {
	signatures []Signature
}

// AggregateResponse contains an aggregated signature
type AggregateResponse struct {
	signature Signature
}

// PrivateKeyGenerateResponse contains a generated private key
type PrivateKeyGenerateResponse struct {
	privateKey PrivateKey
}

// PrivateKeySignRequest contains the message and private key to sign it with
type PrivateKeySignRequest struct {
	privateKey PrivateKey
	message    Message
}

// PrivateKeySignResponse contains the signature of a message with a key
type PrivateKeySignResponse struct {
	signature Signature
}

// PrivateKeyPublicKeyRequest contains the private key to get the public key of
type PrivateKeyPublicKeyRequest struct {
	privateKey PrivateKey
}

// PrivateKeyPublicKeyResponse contains the public key of a private key
type PrivateKeyPublicKeyResponse struct {
	publicKey PublicKey
}

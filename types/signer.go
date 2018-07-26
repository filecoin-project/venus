package types

// Signer is an interface for SignBytes
type Signer interface {
	SignBytes(data []byte, addr Address) (Signature, error)
}

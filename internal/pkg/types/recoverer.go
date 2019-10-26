package types

// Recoverer is an interface for ecrecover
type Recoverer interface {
	Ecrecover(data []byte, sig Signature) ([]byte, error)
}

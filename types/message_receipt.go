package types

import (
	cbor "gx/ipfs/QmSyK1ZiAP98YvnxsTfQpb669V2xeTHRbG4Y6fgKS3vVSd/go-ipld-cbor"
)

func init() {
	cbor.RegisterCborType(MessageReceipt{})
}

// MessageReceipt represents the result of sending a message.
type MessageReceipt struct {
	// `0` is success, anything else is an error code in unix style.
	ExitCode uint8 `json:"exitCode"`

	// Return contains the return values, if any, from processing a message.
	// This can be non-empty even in the case of error (e.g., to provide
	// programmatically readable detail about errors).
	Return []Bytes `json:"return"`
}

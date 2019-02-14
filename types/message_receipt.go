package types

import (
	cbor "gx/ipfs/QmRZxJ7oybgnnwriuRub9JXp5YdFM9wiGSyRq38QC7swpS/go-ipld-cbor"
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

	// GasAttoFIL Charge is the actual amount of FIL transferred from the sender to the miner for processing the message
	GasAttoFIL *AttoFIL `json:"gasAttoFIL"`
}

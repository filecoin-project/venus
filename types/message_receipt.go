package types

import (
	cbor "gx/ipfs/QmZpue627xQuNGXn7xHieSjSZ8N4jot6oBHwe9XTn3e4NU/go-ipld-cbor"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

func init() {
	cbor.RegisterCborType(MessageReceipt{})
}

// MessageReceipt represents the result of sending a message.
type MessageReceipt struct {
	// The ID of the message this references
	Message *cid.Cid
	// `0` is success, anything else is an error code in unix style.
	ExitCode uint8
	// The value returned from the message.
	// TODO: limit size
	Return []byte
}

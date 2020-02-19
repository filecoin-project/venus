package types

import "github.com/filecoin-project/go-filecoin/internal/pkg/encoding"

// TODO: remove these Atlas registrations after the the Un/SignedMessage types no longer use
// refmt to compute CIDs.
func init() {
	encoding.RegisterIpldCborType(UnsignedMessage{})
	encoding.RegisterIpldCborType(SignedMessage{})
}

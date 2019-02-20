package types

import (
	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/proofs"
)

func init() {
	cbor.RegisterCborType(Commitments{})
}

// Commitments is a struct containing the replica and data commitments produced
// when sealing a sector.
type Commitments struct {
	CommD     proofs.CommD
	CommR     proofs.CommR
	CommRStar proofs.CommRStar
}

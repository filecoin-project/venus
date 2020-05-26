package block

import (
	"bytes"
	"fmt"

	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
)

// A Ticket is a marker of a tick of the blockchain's clock.  It is the source
// of randomness for proofs of storage and leader election.  It is generated
// by the miner of a block using a VRF.
type Ticket struct {
	_ struct{} `cbor:",toarray"`
	// A proof output by running a VRF on the VRFProof of the parent ticket
	VRFProof crypto.VRFPi
}

// String returns the string representation of the VRFProof of the ticket
func (t Ticket) String() string {
	return fmt.Sprintf("%x", t.VRFProof)
}

func (t *Ticket) Compare(o *Ticket) int {
	tDigest := t.VRFProof.Digest()
	oDigest := o.VRFProof.Digest()
	return bytes.Compare(tDigest[:], oDigest[:])
}

package types

import "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"

// StorageDealProposal represents a storage client's desire that a storage miner
// store one of its pieces for a specified quantity of time.
type StorageDealProposal struct {
	PieceRef  []byte
	PieceSize Uint64

	Client   address.Address
	Provider address.Address

	ProposalExpiration Uint64
	Duration           Uint64

	StoragePricePerEpoch Uint64
	StorageCollateral    Uint64

	ProposerSignature *Signature
}

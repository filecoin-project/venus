package submodule

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/retrieval"
)

// RetrievalProtocolSubmodule enhances the `Node` with "Retrieval" protocol capabilities.
type RetrievalProtocolSubmodule struct {
	RetrievalAPI *retrieval.API

	// Retrieval Interfaces
	RetrievalMiner *retrieval.Miner
}

// NewRetrievalProtocolSubmodule creates a new retrieval protocol submodule.
func NewRetrievalProtocolSubmodule(ctx context.Context) (RetrievalProtocolSubmodule, error) {
	return RetrievalProtocolSubmodule{
		// RetrievalAPI: nil,
		// RetrievalMiner: nil,
	}, nil
}

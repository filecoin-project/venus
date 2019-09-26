package node

import "github.com/filecoin-project/go-filecoin/protocol/retrieval"

// RetrievalProtocolSubmodule enhances the `Node` with "Retrieval" protocol capabilities.
type RetrievalProtocolSubmodule struct {
	RetrievalAPI *retrieval.API

	// Retrieval Interfaces
	RetrievalMiner *retrieval.Miner
}

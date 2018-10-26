// Package retrieval implements a very simple retrieval protocol that works on high level like this:
//
// 1. CLIENT opens /fil/retrieval/free/0.0.0 stream to MINER
// 2. CLIENT sends MINER a RetrievePieceRequest
// 3. MINER sends CLIENT a RetrievePieceResponse with Status set to Success if it has PieceRef in a sealed sector
// 4. MINER sends CLIENT RetrievePieceChunks until all data associated with PieceRef has been sent
// 5. CLIENT reads RetrievePieceChunk from stream until EOF and then closes stream
package retrieval

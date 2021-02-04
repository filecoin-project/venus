package types

// BlockMessagesInfo contains messages for one block in a tipset.
type BlockMessagesInfo struct { //nolint
	BlsMessages   []ChainMsg
	SecpkMessages []ChainMsg
	Block         *BlockHeader
}

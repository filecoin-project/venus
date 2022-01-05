package types

// BlockMessagesInfo contains messages for one newBlock in a tipset.
type BlockMessagesInfo struct { //nolint
	BlsMessages   []ChainMsg
	SecpkMessages []ChainMsg

	Block *BlockHeader
}

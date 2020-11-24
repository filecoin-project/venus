package block

import "github.com/filecoin-project/venus/pkg/types"

// BlockMessagesInfo contains messages for one block in a tipset.
type BlockMessagesInfo struct { //nolint
	BlsMessages   []types.ChainMsg
	SecpkMessages []types.ChainMsg
	Block         *Block
}

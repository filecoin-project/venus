package message

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
)

// chainProvider provides chain access for updating the message pool in response to new heads.
type chainProvider interface {
	// The TipSetProvider is used only for counting non-null tipsets when expiring messages. We could remove
	// this dependency if expiration was based on round number, or if this object maintained a short
	// list of non-empty tip heights.
	chain.TipSetProvider
	GetHead() block.TipSetKey
}

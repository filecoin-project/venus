package core

import (
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/types"
)

// chainProvider provides chain access for updating the message pool in response to new heads.
type chainProvider interface {
	// The TipSetProvider is used only for counting non-null tipsets when expiring messages. We could remove
	// this dependency if expiration was based on round number, or if this object maintained a short
	// list of non-empty tip heights.
	chain.TipSetProvider
	GetHead() types.TipSetKey
}

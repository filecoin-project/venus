package multisig

import (
	"github.com/filecoin-project/venus/app/submodule/apiface"
	chain2 "github.com/filecoin-project/venus/pkg/chain"
)

type MultiSigSubmodule struct { //nolint
	state apiface.IChain
	mpool apiface.IMessagePool
	store *chain2.Store
}

// MessagingSubmodule enhances the `Node` with multisig capabilities.
func NewMultiSigSubmodule(chainState apiface.IChain, msgPool apiface.IMessagePool, store *chain2.Store) *MultiSigSubmodule {
	return &MultiSigSubmodule{state: chainState, mpool: msgPool, store: store}
}

//API create a new multisig implement
func (sb *MultiSigSubmodule) API() apiface.IMultiSig {
	return newMultiSig(sb)
}

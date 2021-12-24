package multisig

import (
	apiwrapper "github.com/filecoin-project/venus/app/submodule/multisig/v0api"
	chain2 "github.com/filecoin-project/venus/pkg/chain"
	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

type MultiSigSubmodule struct { //nolint
	state v1api.IChain
	mpool v1api.IMessagePool
	store *chain2.Store
}

// MessagingSubmodule enhances the `Node` with multisig capabilities.
func NewMultiSigSubmodule(chainState v1api.IChain, msgPool v1api.IMessagePool, store *chain2.Store) *MultiSigSubmodule {
	return &MultiSigSubmodule{state: chainState, mpool: msgPool, store: store}
}

//API create a new multisig implement
func (sb *MultiSigSubmodule) API() v1api.IMultiSig {
	return newMultiSig(sb)
}

func (sb *MultiSigSubmodule) V0API() v0api.IMultiSig {
	return &apiwrapper.WrapperV1IMultiSig{
		IMultiSig:    newMultiSig(sb),
		IMessagePool: sb.mpool,
	}
}

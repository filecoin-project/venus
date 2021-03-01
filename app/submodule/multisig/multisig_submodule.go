package multisig

import (
	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/mpool"
)

type MultiSigSubmodule struct { //nolint
	state chain.IChain
	mpool mpool.IMessagePool
}

func NewMultiSigSubmodule(chainState chain.IChain, msgPool mpool.IMessagePool) *MultiSigSubmodule {
	return &MultiSigSubmodule{state: chainState, mpool: msgPool}
}

func (ps *MultiSigSubmodule) API() IMultiSig {
	return newMultiSig(ps.state, ps.mpool)
}

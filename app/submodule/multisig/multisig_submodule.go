package multisig

import (
	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/app/submodule/apiface/v0api"
	multisigv0 "github.com/filecoin-project/venus/app/submodule/multisig/v0api"
	chain2 "github.com/filecoin-project/venus/pkg/chain"
)

type MultiSigSubmodule struct { //nolint
	state apiface.IChain
	mpool apiface.IMessagePool
	store *chain2.Store
}

func NewMultiSigSubmodule(chainState apiface.IChain, msgPool apiface.IMessagePool, store *chain2.Store) *MultiSigSubmodule {
	return &MultiSigSubmodule{state: chainState, mpool: msgPool, store: store}
}

func (sb *MultiSigSubmodule) API() apiface.IMultiSig {
	return newMultiSig(sb)
}

func (sb *MultiSigSubmodule) V0API() v0api.IMultiSig {
	return &multisigv0.WrapperV1IMultiSig{
		IMultiSig:    newMultiSig(sb),
		IMessagePool: sb.mpool,
	}
}

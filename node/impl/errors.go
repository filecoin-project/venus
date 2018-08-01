package impl

import "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

var (
	// ErrHeaviestTipSetNotFound indicates that the node does not have a
	// heaviest tipset, which should not happen in the normal course of events.
	ErrHeaviestTipSetNotFound = errors.New("chain manager does not have a heaviest tipset")
)

package node_api

import "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

var (
	// ErrHeaviestTipSetNotFound indicates that the node does not have a
	// heaviest tipset, which should not happen in the normal course of events.
	ErrHeaviestTipSetNotFound = errors.New("chain manager does not have a heaviest tipset")
	// ErrCouldNotDefaultFromAddress indicates that the user didn't specify the
	// "from" address and we couldn't default it because there were zero or
	// more than one to choose from.
	ErrCouldNotDefaultFromAddress = errors.New("no from address specified and no default address available")
)

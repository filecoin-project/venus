package impl

import (
	"github.com/pkg/errors"
)

var (
	// ErrCouldNotDefaultFromAddress indicates that the user didn't specify the
	// "from" address and we couldn't default it because there were zero or
	// more than one to choose from.
	ErrCouldNotDefaultFromAddress = errors.New("no from address specified and no default address available")
	// ErrCannotPingSelf indicates that you tried to ping yourself but cannot.
	ErrCannotPingSelf = errors.New("cannot ping self")
	// ErrNodeOffline indicates that the node must not be offline for the operation performed.
	ErrNodeOffline = errors.New("node must be online")
)

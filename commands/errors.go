package commands

import (
	"fmt"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
)

var (
	// ErrAlreadyRunning is the error returned when trying to start the daemon, even though it is already running.
	ErrAlreadyRunning = errors.New("daemon is already running")

	// ErrCannotPingSelf indicates that you tried to ping yourself but cannot.
	ErrCannotPingSelf = fmt.Errorf("cannot ping self")

	// ErrInvalidSize indicates that the provided size was invalid.
	ErrInvalidSize = fmt.Errorf("invalid size")

	// ErrInvalidPrice indicates that the provided price was invalid.
	ErrInvalidPrice = fmt.Errorf("invalid price")

	// ErrInvalidCollateral indicates that provided collateral was invalid.
	ErrInvalidCollateral = fmt.Errorf("invalid collateral")

	// ErrInvalidPledge indicates that provided pledge was invalid.
	ErrInvalidPledge = fmt.Errorf("invalid pledge")

	// ErrLatestBlockStateRootNil indicates that the state root in latest block was nil.
	ErrLatestBlockStateRootNil = fmt.Errorf("state root in latest block was nil")

	// ErrMissingDaemon is the error returned when trying to execute a command that requires the daemon to be started.
	ErrMissingDaemon = errors.New("daemon must be started before using this command")

	// ErrNodeOffline indicates that
	ErrNodeOffline = fmt.Errorf("node must be online")

	// ErrNoWalletAddresses indicates that there are no addresses in wallet to mine to.
	ErrNoWalletAddresses = fmt.Errorf("no addresses in wallet to mine to")
)

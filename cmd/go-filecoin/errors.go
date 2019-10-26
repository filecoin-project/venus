package commands

import (
	"fmt"
	"github.com/pkg/errors"
)

var (
	// ErrInvalidSize indicates that the provided size was invalid.
	ErrInvalidSize = fmt.Errorf("invalid size")

	// ErrInvalidPrice indicates that the provided price was invalid.
	ErrInvalidPrice = fmt.Errorf("invalid price")

	// ErrInvalidAmount indicates that the provided amount was invalid.
	ErrInvalidAmount = fmt.Errorf("invalid amount")

	// ErrInvalidCollateral indicates that provided collateral was invalid.
	ErrInvalidCollateral = fmt.Errorf("invalid collateral")

	// ErrInvalidPledge indicates that provided pledge was invalid.
	ErrInvalidPledge = fmt.Errorf("invalid pledge")

	// ErrInvalidBlockHeight indicates that the provided block height was invalid.
	ErrInvalidBlockHeight = fmt.Errorf("invalid block height")

	// ErrMissingDaemon is the error returned when trying to execute a command that requires the daemon to be started.
	ErrMissingDaemon = errors.New("daemon must be started before using this command")

	// ErrNoWalletAddresses indicates that there are no addresses in wallet to mine to.
	ErrNoWalletAddresses = fmt.Errorf("no addresses in wallet to mine to")
)

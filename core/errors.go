package core

import "github.com/filecoin-project/go-filecoin/vm/errors"

var (
	// Most errors should live in the actors that throw them. However some
	// errors will be pervasive so we define them centrally here.

	// ErrCannotTransferNegativeValue signals a transfer error, value must be positive.
	ErrCannotTransferNegativeValue = errors.NewRevertError("cannot transfer negative values")
	// ErrInsufficientBalance signals insufficient balance for a transfer.
	ErrInsufficientBalance = errors.NewRevertError("not enough balance")
)

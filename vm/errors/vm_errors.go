package errors

// ReservedErrors is the highest error code that may not be used by actors
const ReservedErrors = 32

const (
	_ = iota
	_
	// ErrCannotTransferNegativeValue is the error code for attempting to transfer a negative number
	ErrCannotTransferNegativeValue
	// ErrInsufficientBalance is the error code for attempting to send more than you have
	ErrInsufficientBalance
	// ErrMissingExport is the error code for a message that calls a non-existent method
	ErrMissingExport
	// ErrNoActorCode indicates the recipient's code could not be loaded.
	ErrNoActorCode
)

// Errors is a map from exit codes to errors.
// Most errors should live in the actors that throw them. However some
// errors will be pervasive so we define them centrally here.
var Errors = map[uint8]error{
	ErrCannotTransferNegativeValue: NewCodedRevertError(ErrCannotTransferNegativeValue, "cannot transfer negative values"),
	ErrInsufficientBalance:         NewCodedRevertError(ErrInsufficientBalance, "not enough balance"),
	ErrMissingExport:               NewCodedRevertError(ErrInsufficientBalance, "actor does not export method"),
	ErrNoActorCode:                 NewCodedRevertError(ErrNoActorCode, "actor code not found"),
}

// VMExitCodeToError tries to locate an error in either the VM errors or the provide error map
// If it fails to locate the error it will return an unknown error
func VMExitCodeToError(exitCode uint8, actorErrors map[uint8]error) error {
	var (
		err   error
		found bool
	)

	if exitCode <= ReservedErrors {
		err, found = Errors[exitCode]
	} else {
		err, found = actorErrors[exitCode]
	}

	if found {
		return err
	}

	return NewCodedRevertError(exitCode, "Error executing command. Consult the logs.")
}

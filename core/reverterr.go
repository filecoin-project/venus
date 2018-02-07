package core

// revertError is an error wrapper that signifies that this error means a
// process should revert state changes made by the call that returned the error.
type revertErrorWrap struct {
	err error
}

func (re *revertErrorWrap) Error() string {
	return re.err.Error()
}

func (re *revertErrorWrap) RevertState() bool {
	return true
}

type revertError interface {
	RevertState() bool
}

func ShouldRevert(err error) bool {
	rev, ok := err.(revertError)
	return ok && rev.RevertState()
}

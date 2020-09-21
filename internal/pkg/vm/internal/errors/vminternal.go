// Package internal has all the things vm and only vm need.
//
// This contents can be slowly placed on the vm internal.
package errors

const (
	// ErrInsufficientGas indicates that an actor did not have sufficient gas to run a message
	// Dragons: remove when new actors come in
	ErrInsufficientGas = 36
)

package commands

import (
	"fmt"
	"io"
)

// Stringer is everything that has a String() method
type Stringer interface {
	String() string
}

// PrintString prints a given Stringer to the writer.
func PrintString(w io.Writer, s Stringer) error {
	_, err := fmt.Fprintln(w, s.String())
	return err
}

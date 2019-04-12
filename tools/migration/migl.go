package migration

import (
	"log"
	"os"
)

// right now this is only a wrapper for golang logger, but will
// take a logfile os.File to write to also.
//   there are two logs here, one for the logfile and one for stdout.
//   everything is output to the logfile.
//   if verbose is true, stdout/err are identical to the logfile.
//   if verbose is false, all that is shown is status + errors.
type Migl struct {
	logfile *os.File
	verbose bool
}

// verbose is hard-coded to true for now.
func NewMigl(out *os.File, verb bool) Migl {
	log.SetOutput(os.Stdout)
	return Migl{logfile: out, verbose: verb}
}

func (m *Migl) Fatal(msg string) {
	log.Fatal(msg)
}

func (m *Migl) Print(msg string) {
	log.Print(msg)
}

func (m *Migl) Panic(msg string) {
	log.Panic(msg)
}

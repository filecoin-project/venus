package internal

import (
	"log"
	"os"
)

// TODO: see Issue #2584
type Migl struct {
	logfile *os.File
	verbose bool
}

func NewMigl(out *os.File, verb bool) Migl {
	log.SetOutput(out)
	return Migl{logfile: out, verbose: verb}
}

func (m *Migl) Error(err error) {
	log.Printf("ERROR: %s", err.Error())
}

func (m *Migl) Print(msg string) {
	log.Print(msg)
}

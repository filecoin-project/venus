package migration

import (
	"github.com/prometheus/common/log"
	"os"
)

// right now this is only a wrapper for golang logger, but will
// take a logfile os.File to write to also.
type Migl struct {
	logfile *os.File
}

func NewMigl(out *os.File) Migl {
	return Migl{logfile: out}
}

func (m *Migl) Debug(str string) {
	log.Debug(str)
}

func (m *Migl) Error(str string) {
	log.Error(str)
}

func (m *Migl) Info(str string) {
	log.Info(str)
}

func (m *Migl) Warn(str string) {
	log.Warn(str)
}

package pluginlocalfilecoin

import (
	"io"
)

// Events not implemented
func (l *Localfilecoin) Events() (io.ReadCloser, error) {
	panic("Not Implemented")
}

// StderrReader provides an io.ReadCloser to the running daemons stderr
func (l *Localfilecoin) StderrReader() (io.ReadCloser, error) {
	return l.readerFor("daemon.stderr")
}

// StdoutReader provides an io.ReadCloser to the running daemons stdout
func (l *Localfilecoin) StdoutReader() (io.ReadCloser, error) {
	return l.readerFor("daemon.stdout")
}

// Heartbeat not implemented
func (l *Localfilecoin) Heartbeat() (map[string]string, error) {
	panic("Not Implemented")
}

// Metric not implemented
func (l *Localfilecoin) Metric(key string) (string, error) {
	panic("Not Implemented")
}

// GetMetricList not implemented
func (l *Localfilecoin) GetMetricList() []string {
	panic("Not Implemented")
}

// GetMetricDesc not implemented
func (l *Localfilecoin) GetMetricDesc(key string) (string, error) {
	panic("Not Implemented")
}

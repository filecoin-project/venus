package pluginmockfilecoin

import (
	"io"
	"io/ioutil"
)

// StderrReader returns an io.ReadCloser that represents daemon stderr
func (m *Mockfilecoin) StderrReader() (io.ReadCloser, error) {
	return ioutil.NopCloser(&m.stderr), nil
}

// StdoutReader returns an io.ReadCloser that represents daemon stdout
func (m *Mockfilecoin) StdoutReader() (io.ReadCloser, error) {
	return ioutil.NopCloser(&m.stdout), nil
}

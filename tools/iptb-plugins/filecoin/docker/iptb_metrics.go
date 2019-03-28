package pluginlocalfilecoin

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
)

// Events not implemented
func (l *Dockerfilecoin) Events() (io.ReadCloser, error) {
	panic("Not Implemented")
}

// StderrReader provides an io.ReadCloser to the running daemons stderr
func (l *Dockerfilecoin) StderrReader() (io.ReadCloser, error) {
	cli, err := l.GetClient()
	if err != nil {
		return nil, err
	}

	options := types.ContainerLogsOptions{
		ShowStdout: false,
		ShowStderr: true,
		Follow:     true,
	}

	return cli.ContainerLogs(context.Background(), l.ID, options)
}

// StdoutReader provides an io.ReadCloser to the running daemons stdout
func (l *Dockerfilecoin) StdoutReader() (io.ReadCloser, error) {
	cli, err := l.GetClient()
	if err != nil {
		return nil, err
	}

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: false,
		Follow:     true,
	}

	return cli.ContainerLogs(context.Background(), l.ID, options)
}

// Heartbeat not implemented
func (l *Dockerfilecoin) Heartbeat() (map[string]string, error) {
	panic("Not Implemented")
}

// Metric not implemented
func (l *Dockerfilecoin) Metric(key string) (string, error) {
	panic("Not Implemented")
}

// GetMetricList not implemented
func (l *Dockerfilecoin) GetMetricList() []string {
	panic("Not Implemented")
}

// GetMetricDesc not implemented
func (l *Dockerfilecoin) GetMetricDesc(key string) (string, error) {
	panic("Not Implemented")
}

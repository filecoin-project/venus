package testhelpers

import (
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/filecoin-project/go-filecoin/build/project"
	"github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

// GetFreePort gets a free port from the kernel
// Credit: https://github.com/phayes/freeport
func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close() // nolint: errcheck
	return l.Addr().(*net.TCPAddr).Port, nil
}

// MustGetFilecoinBinary returns the path where the filecoin binary will be if it has been built and panics otherwise.
func MustGetFilecoinBinary() string {
	path, err := GetFilecoinBinary()
	if err != nil {
		panic(err)
	}

	return path
}

// GetFilecoinBinary returns the path where the filecoin binary will be if it has been built
func GetFilecoinBinary() (string, error) {
	bin, provided := testflags.BinaryPath()
	if !provided {
		bin = project.Root("go-filecoin")
	}

	_, err := os.Stat(bin)
	if err != nil {
		return "", err
	}

	if os.IsNotExist(err) {
		return "", err
	}

	return bin, nil
}

// WaitForIt waits until the given callback returns true.
func WaitForIt(count int, delay time.Duration, cb func() (bool, error)) error {
	var done bool
	var err error
	for i := 0; i < count; i++ {
		done, err = cb()
		if err != nil {
			return err
		}
		if done {
			break
		}
		time.Sleep(delay)
	}

	if !done {
		return fmt.Errorf("timeout waiting for it")
	}

	return nil
}

// WaitTimeout waits for the waitgroup for the specified max timeout.
// Returns true if waiting timed out.
func WaitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}

package pluginlocalfilecoin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
)

func (l *Localfilecoin) isAlive() (bool, error) {
	pid, err := l.getPID()
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, nil
	}

	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true, nil
	}

	return false, nil
}

func (l *Localfilecoin) getPID() (int, error) {
	b, err := ioutil.ReadFile(filepath.Join(l.dir, "daemon.pid"))
	if err != nil {
		return -1, err
	}

	return strconv.Atoi(string(b))
}

func (l *Localfilecoin) env() ([]string, error) {
	envs := os.Environ()
	filecoinpath := "FIL_PATH=" + l.dir

	for i, e := range envs {
		if strings.HasPrefix(e, "FIL_PATH=") {
			envs[i] = filecoinpath
			return envs, nil
		}
	}

	return append(envs, filecoinpath), nil
}

func (l *Localfilecoin) signalAndWait(p *os.Process, waitch <-chan struct{}, signal os.Signal, t time.Duration) error {
	err := p.Signal(signal)
	if err != nil {
		return fmt.Errorf("error killing daemon %s: %s", l.dir, err)
	}

	select {
	case <-waitch:
		return nil
	case <-time.After(t):
		return errTimeout
	}
}

func (l *Localfilecoin) readerFor(file string) (io.ReadCloser, error) {
	return os.OpenFile(filepath.Join(l.dir, file), os.O_RDONLY, 0)
}

// GetPeerID returns the nodes peerID by running its `id` command.
// TODO this a temp fix, should read the nodes keystore instead
func (l *Localfilecoin) GetPeerID() (*cid.Cid, error) {
	// run the id command
	out, err := l.RunCmd(context.TODO(), nil, "go-filecoin", "id", "--format=<id>")
	if err != nil {
		return nil, err
	}

	if out.ExitCode() != 0 {
		return nil, errors.New("Could not get PeerID, non-zero exit code")
	}

	_, err = io.Copy(os.Stdout, out.Stderr())
	if err != nil {
		return nil, err
	}

	// convert the reader to a string TODO this is annoying
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(out.Stdout())
	if err != nil {
		return nil, err
	}
	cidStr := strings.TrimSpace(buf.String())

	// decode the parsed string to a cid...maybe
	return cid.Decode(cidStr)
}

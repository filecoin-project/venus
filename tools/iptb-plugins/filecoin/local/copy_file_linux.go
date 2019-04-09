// +build linux

package pluginlocalfilecoin

import (
	"os/exec"
)

// We can't copy the bits in go due to the following bug
// https://github.com/golang/go/issues/22315
func copyFileContents(src, dst string) (err error) {
	cmd := exec.Command("cp", src, dst)
	if err := cmd.Start(); err != nil {
		return err
	}

	return cmd.Wait()
}

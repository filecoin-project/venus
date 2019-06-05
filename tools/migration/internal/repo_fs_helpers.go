package internal

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/filecoin-project/go-filecoin/repo"
	rcopy "github.com/otiai10/copy"
)

// This is a set of file system helpers for repo migration.
//
// CloneRepo and InstallRepo expect a symlink that points to the filecoin repo
// directory, typically ~/.filecoin or whatever FIL_PATH is set to.
//
// This does not touch sector data.

// CloneRepo copies a linked repo directory to a new, writable repo directory.
// The directory created will be named with a timestamp, version number, and
// uniqueifyig tag if necessary.
//
// Params:
//   linkPath: path to a symlink, which links to an actual repo directory to be cloned.
func CloneRepo(linkPath string, newVersion uint) (string, error) {
	repoDirPath, err := os.Readlink(linkPath)
	if err != nil {
		return "", fmt.Errorf("repo path must be a symbolic link: %s", err)
	}

	newDirPath, err := makeNewRepoPath(linkPath, newVersion)
	if err != nil {
		return "", err
	}

	if err := rcopy.Copy(repoDirPath, newDirPath); err != nil {
		return "", err
	}
	if err := os.Chmod(newDirPath, os.ModeDir|0744); err != nil {
		return "", err
	}
	return newDirPath, nil
}

// InstallNewRepo updates a symlink to point to a new repo directory.
func InstallNewRepo(linkPath, newRepoPath string) error {
	// Check that linkPath is a symlink.
	if _, err := os.Readlink(linkPath); err != nil {
		return err
	}

	// Check the repo exists.
	if _, err := os.Stat(newRepoPath); err != nil {
		return err
	}

	// Replace the symlink.
	if err := os.Remove(linkPath); err != nil {
		return err
	}
	if err := os.Symlink(newRepoPath, linkPath); err != nil {
		return err
	}
	return nil
}

// makeNewRepoPath generates a new repo path for a migration.
// Params:
//     linkPath:  the actual old repo path
//     version:   the prospective version for the new repo
// Returns:
//     a vacant path for a new repo directory
// Example output:
//     /Users/davonte/.filecoin-20190806-150455-v002
func makeNewRepoPath(linkPath string, version uint) (string, error) {
	// Search for a free name
	now := time.Now()
	var newpath string
	for i := uint(0); i < 1000; i++ {
		newpath = repo.MakeRepoDirName(linkPath, now, version, i)
		if _, err := os.Stat(newpath); os.IsNotExist(err) {
			return newpath, nil
		}
	}
	// this should never happen, but just in case.
	return "", errors.New("couldn't find a free dirname for cloning")
}

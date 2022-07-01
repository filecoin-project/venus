package fsutil

import (
	"fmt"
	"syscall"
)

func Statfs(path string) (FsStat, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return FsStat{}, fmt.Errorf("statfs: %w", err)
	}

	// force int64 to handle platform specific differences
	//nolint:unconvert
	return FsStat{
		Capacity:  int64(stat.Blocks) * int64(stat.Bsize),
		Available: int64(stat.Bavail) * int64(stat.Bsize),
	}, nil
}

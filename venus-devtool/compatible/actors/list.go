package main

import (
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
)

var filterWithSuffix = func(suffix string) func(path string, d fs.DirEntry) bool {
	return func(path string, d fs.DirEntry) bool {
		if d.IsDir() {
			return true
		}

		if !strings.HasSuffix(path, suffix) {
			return true
		}

		return false
	}
}

func listFilesInDir(dir string, filter func(string, fs.DirEntry) bool) ([]string, error) {
	var paths []string

	err := fs.WalkDir(os.DirFS(dir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walking %s: %w", path, err)
		}

		if filter(path, d) {
			return nil
		}

		paths = append(paths, path)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk through the chain/actors subdir: %w", err)
	}

	sort.Strings(paths)
	return paths, nil
}

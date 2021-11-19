package main

import (
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
)

func listFilesInDir(dir string, ext string) ([]string, error) {
	var paths []string

	err := fs.WalkDir(os.DirFS(dir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walking %s: %w", path, err)
		}

		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ext) {
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

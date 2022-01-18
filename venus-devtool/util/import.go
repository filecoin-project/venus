package util

import (
	"go/build"
)

func FindLocationForImportPath(path string) (string, error) {
	pkg, err := build.Import(path, ".", build.FindOnly)
	if err != nil {
		return "", err
	}

	return pkg.Dir, nil
}

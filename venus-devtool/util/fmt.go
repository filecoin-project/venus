package util

import "golang.org/x/tools/imports"

func FmtFile(path string, src []byte) ([]byte, error) {
	return imports.Process(path, src, nil)
}

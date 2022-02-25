package util

import (
	"go/build"
	"sync"
)

type PackageBuildInfo struct {
	*build.Package
	Err error
}

var pkgCache = struct {
	sync.RWMutex
	found map[string]PackageBuildInfo
}{
	found: map[string]PackageBuildInfo{},
}

func FindPackage(importPath string) PackageBuildInfo {
	pkgCache.RLock()
	found, ok := pkgCache.found[importPath]
	pkgCache.RUnlock()

	if !ok {
		pkgCache.Lock()
		pkg, err := build.Import(importPath, ".", 0)

		found = PackageBuildInfo{
			Package: pkg,
			Err:     err,
		}

		pkgCache.found[importPath] = found
		pkgCache.Unlock()
	}

	return found
}

func FindPackageLocation(importPath string) (string, error) {
	found := FindPackage(importPath)
	if found.Err != nil {
		return "", found.Err
	}

	return found.Dir, nil
}

package api

// VersionInfo holds details about a version of go-filecoin.
type VersionInfo struct {
	// Commit, is the git sha that was used to build this version of go-filecoin.
	Commit string
}

type Version interface {
	// Full, returns all version information that is available.
	Full() (*VersionInfo, error)
}

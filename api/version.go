package api

// VersionInfo holds details about a version of go-filecoin.
type VersionInfo struct {
	// Commit, is the git sha that was used to build this version of go-filecoin.
	Commit string
}

// Version is the interface that defines methods to view version information about this node.
type Version interface {
	// Full, returns all version information that is available.
	Full() (*VersionInfo, error)
}

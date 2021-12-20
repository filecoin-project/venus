package v0api

import "context"

type IConfig interface {
	// Rule[perm:admin]
	ConfigSet(ctx context.Context, dottedPath string, paramJSON string) error
	// Rule[perm:read]
	ConfigGet(ctx context.Context, dottedPath string) (interface{}, error)
}

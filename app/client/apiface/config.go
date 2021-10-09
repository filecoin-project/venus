package apiface

import "context"

type IConfig interface {
	// Rule[perm:read]
	ConfigSet(ctx context.Context, dottedPath string, paramJSON string) error
	// Rule[perm:read]
	ConfigGet(ctx context.Context, dottedPath string) (interface{}, error)
}

package config

import (
	"context"
)

var _ IConfig = &configAPI{}

type configAPI struct { //nolint
	config *ConfigModule
}

// ConfigSet sets the given parameters at the given path in the local config.
// The given path may be either a single field name, or a dotted path to a field.
// The JSON value may be either a single value or a whole data structure to be replace.
// For example:
// ConfigSet("datastore.path", "dev/null") and ConfigSet("datastore", "{\"path\":\"dev/null\"}")
// are the same operation.
func (ca *configAPI) ConfigSet(ctx context.Context, dottedPath string, paramJSON string) error {
	return ca.config.Set(dottedPath, paramJSON)
}

// ConfigGet gets config parameters from the given path.
// The path may be either a single field name, or a dotted path to a field.
func (ca *configAPI) ConfigGet(ctx context.Context, dottedPath string) (interface{}, error) {
	return ca.config.Get(dottedPath)
}

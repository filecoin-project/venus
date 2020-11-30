package config

type ConfigAPI struct { //nolint
	config *ConfigModule
}

// ConfigSet sets the given parameters at the given path in the local config.
// The given path may be either a single field name, or a dotted path to a field.
// The JSON value may be either a single value or a whole data structure to be replace.
// For example:
// ConfigSet("datastore.path", "dev/null") and ConfigSet("datastore", "{\"path\":\"dev/null\"}")
// are the same operation.
func (configAPI *ConfigAPI) ConfigSet(dottedPath string, paramJSON string) error {
	return configAPI.config.Set(dottedPath, paramJSON)
}

// ConfigGet gets config parameters from the given path.
// The path may be either a single field name, or a dotted path to a field.
func (configAPI *ConfigAPI) ConfigGet(dottedPath string) (interface{}, error) {
	return configAPI.config.Get(dottedPath)
}

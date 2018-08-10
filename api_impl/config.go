package api_impl

// NodeConfig is an implementation of api.Config interface for node.
// It provides api methods to interact with the configuration of a node.
type NodeConfig struct {
	api *NodeAPI
}

// NewNodeConfig creates an instance of the NodeConfig struct.
func NewNodeConfig(api *NodeAPI) *NodeConfig {
	return &NodeConfig{api: api}
}

// Get, returns the configuration value for the passed in key.
func (api *NodeConfig) Get(key string) (interface{}, error) {
	repo := api.api.node.Repo
	cfg := repo.Config()

	cf, err := cfg.Get(key)
	if err != nil {
		return nil, err
	}

	return cf, nil
}

// Set, sets the configuration value for the passed in key, to the given value.
// Returns the newly set value on success.
func (api *NodeConfig) Set(key, value string) (interface{}, error) {
	repo := api.api.node.Repo
	cfg := repo.Config()

	cf, err := cfg.Set(key, value)
	if err != nil {
		return nil, err
	}
	if err := repo.ReplaceConfig(cfg); err != nil {
		return nil, err
	}

	return cf, nil
}

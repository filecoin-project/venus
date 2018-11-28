package impl

type nodeConfig struct {
	api *nodeAPI
}

func newNodeConfig(api *nodeAPI) *nodeConfig {
	return &nodeConfig{api: api}
}

// Get, returns the configuration value for the passed in key.
func (api *nodeConfig) Get(key string) (interface{}, error) {
	repo := api.api.node.Repo
	cfg := repo.Config()

	cf, err := cfg.Get(key)
	if err != nil {
		return nil, err
	}

	return cf, nil
}

// Set, sets the configuration value for the passed in key, to the given value.
func (api *nodeConfig) Set(key, value string) error {
	repo := api.api.node.Repo
	cfg := repo.Config()

	err := cfg.Set(key, value)
	if err != nil {
		return err
	}
	if err := repo.ReplaceConfig(cfg); err != nil {
		return err
	}

	return nil
}

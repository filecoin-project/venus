package config

// Config is an in memory representation of the filecoin configuration file
type Config struct {
	API   *APIConfig
	Swarm *SwarmConfig
}

// APIConfig holds all configuration options related to the api.
type APIConfig struct {
	Address string
}

func newDefaultAPIConfig() *APIConfig {
	return &APIConfig{
		Address: ":3453",
	}
}

// SwarmConfig holds all configuration options related to the swarm.
type SwarmConfig struct {
	Address string
}

func newDefaultSwarmConfig() *SwarmConfig {
	return &SwarmConfig{
		Address: "/ip4/127.0.0.1/tcp/6000",
	}
}

// NewDefaultConfig returns a config object with all the fields filled out to
// their default values
func NewDefaultConfig() *Config {
	return &Config{
		API:   newDefaultAPIConfig(),
		Swarm: newDefaultSwarmConfig(),
	}
}

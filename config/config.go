package config

// Config is an in memory representation of the filecoin configuration file
type Config struct {
	// look at all the pretty configurable things
}

// NewDefaultConfig returns a config object with all the fields filled out to
// their default values
func NewDefaultConfig() *Config {
	return new(Config)
}

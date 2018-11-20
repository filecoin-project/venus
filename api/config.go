package api

// Config is the interface that defines methods to show and edit the configuration.
type Config interface {
	// Get, returns the configuration value for the passed in key.
	Get(key string) (interface{}, error)
	// Set, sets the configuration value for the passed in key, to the given value.
	// Returns the newly set value on success.
	Set(key, value string) error
}

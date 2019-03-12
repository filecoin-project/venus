package fixtures

// Genesis returns bytes for a genesis.car file generated during build time
func Genesis() []byte {
	return []byte("DON'T COMMIT CHANGES TO THIS FILE, IT WILL BE MODIFIED AND THEN REVERTED DURING BUILD TIME")
}

package fixtures

func Genesis() []byte {
	return []byte("DONT COMMIT CHANGES TO THIS FILE, IT WILL BE MODIFIED AND THEN REVERTED DURING BUILD TIME")
}

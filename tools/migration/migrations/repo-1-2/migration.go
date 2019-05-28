package migration12

// MetadataFormatJSONtoCBOR is the migration from version 1 to 2.
type MetadataFormatJSONtoCBOR struct {
}

// Describe describes the steps this migration will take.
func (m *MetadataFormatJSONtoCBOR) Describe() string {
	return `
    MetadataFormatJSONtoCBOR migrates the storage repo from version 1 to 2.

    This migration changes the chain store metadata serialization from JSON to CBOR.
    Your chain store metadata will be read in as JSON and rewritten as CBOR. This consists
    of CIDs and the chain State Root.  No other repo data is changed.
`
}

// Migrate performs the migration steps
func (m *MetadataFormatJSONtoCBOR) Migrate(newRepoPath string) error {
	return nil
}

// Versions returns the old and new versions that are valid for this migration
func (m *MetadataFormatJSONtoCBOR) Versions() (from, to uint) {
	return 1, 2
}

// Validate performs validation tests for the migration steps
func (m *MetadataFormatJSONtoCBOR) Validate(oldRepoPath, newRepoPath string) error {
	return nil
}

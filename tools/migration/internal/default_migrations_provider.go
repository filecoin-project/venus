package internal

// DefaultMigrationsProvider provides a list of migrations available for migrating
// in production.
// To add a migration:
// 1. add a migration folder in tools/migration/migrations like repo-<fromversion>-<toversion>
// 2. put the migration code in the folder in its own package
// 2. add the new migration package to the imports above
// 3. instantiate and append it to the list of migrations returned by DefaultMigrationsProvider
//
// See runner_test for examples.

import (
	migration12 "github.com/filecoin-project/go-filecoin/tools/migration/migrations/repo-1-2"
)

// DefaultMigrationsProvider is the migrations provider dependency used in production.
// You may provide a test version when needed. Please see runner_test.go for more information.
func DefaultMigrationsProvider() []Migration {
	return []Migration{
		&migration12.MetadataFormatJSONtoCBOR{},
	}
}

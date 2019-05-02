package internal

// DefaultMigrationsProvider provides a list of migrations available for migrating
// in production.

// To add a migration:
// 1. add a migration folder in tools/migration/migrations like repo-<fromversion>-<toversion>
// 2. put the migration code in the folder in its own package
// 2. add the new migration package to the imports above
// 3. instantiate and append it to the list of migrations returned.
//
// See runner_test for examples.
func DefaultMigrationsProvider() []Migration {
	return []Migration{}
}
# IMPORTANT

**DO NOT REGENERATE** test fixtures associated with a migration, using go-filecoin code that is later than the intended "oldVersion" associated with the migration.

This will not only invalidate the tests, but users will be unable to run migrations on 
their repo, because the migration code becomes polluted with behaviors past its original 
intended versions. The migration will likely become completely unable to read their repo.

If your changes have broken migration tests, then one of the following scenarios may apply:

1. A migration has already been created and merged for the upcoming release, and you are are making more breaking changes to the repo. In this case, you will need to:

    * write your own migration for your changes and backport the old code to the other one, 
        <br>OR
    * include your changes in the other migration.
2. The migration is recent but applies to a previous release. In this case you need to write a new migration and backport the old code.
3. The previous migration version is enough versions behind that no nodes should even be running it by now. Consider invalidating the broken migration.
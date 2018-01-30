# Contributor guidelines

We track work items via the [Project board](https://github.com/orgs/filecoin-project/projects/2).

## What's the bar for inclusion in master?

Presently (Q1'18) the minimum bar is:
 * Must be unittested
 * Must be code reviewed by all of {@phritz, @dignifiedquire, @whyrusleeping}
 * Must match a subset of the spec
 * Functionality should be _somehow_ user-visible (eg, via command line interface)
 * Should be bite-sized

Present (Q1'18) NON-requirements:
 * Doesn’t have to do all or even much of anything it will ultimately do
 * Doesn't have to be fast

Likely future requirements:
 * Integration tested
 * Respects Juan’s most important API requirements

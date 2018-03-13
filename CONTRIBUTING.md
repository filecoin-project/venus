# Contributor guidelines

We track work items via the [Project board](https://github.com/orgs/filecoin-project/projects/2).

## Conventions & style

There are always exceptions but generally:
 * Accessors: `Foo(), SetFoo()`
 * Predicates: `isFoo()`
 * Variable names
   * Channels: `fooCh`
   * Cids: `fooCid`
 * Test-only methods: use a parallel type
 * Logging
   * `Warning`: noteworthy but not completely unexpected
   * `Error`: a truly unexpected condition that should not happen in Real Life and that a dev should go look at

## Error handling

The overarching concern is safety of the user: do not give the user an
incorrect view of the world.

* If an error is likely a function of an input, discard the input.
* If an error could be transient, attempt to continue making progress.
* If an error appears to be permanent, panic.
* If we find we have inconsistent internal state, panic.

## The Spec

The spec must be in sync with the code. Updates to the spec should
happen before the code is merged into master. Significant changes to
the spec requires discussion and buy-in from {@nicola, @whyrusleeping,
@dignifiedquire}.

At present (Q1'18) we give ourselves some leeway with spec/code update
lag: it's ok for it to be out of sync for a short period of time while
exploring the right thing to do in the code. The important thing at
this early stage is "the spec is updated" rather than "the spec is
updated in precise lockstep with the code". That will be the policy at
some point in the future.
 
## What's the bar for inclusion in master?

Presently (Q1'18) the minimum bar is:
 * Must be unittested
 * Must be code reviewed by all of {@phritz, @dignifiedquire, @whyrusleeping}
 * Must lint (`go run ./build/*.go lint`)
 * CI must be green
 * Must match a subset of the spec, unless when considering changes to the spec -- see details above
 * Functionality should be _somehow_ user-visible (eg, via command line interface)
 * Should be bite-sized

Present (Q1'18) NON-requirements:
 * Doesn’t have to do all or even much of anything it will ultimately do
 * Doesn't have to be fast

Likely future requirements:
 * Integration tested
 * Respects Juan’s most important API requirements

## Testing philosophy
* We use both unit tests (for functions, etc.) and command tests (that test commands executing on a filecoin node)
* Test the output/contracts, not the individual lines of code (which we expect to change)

## Merge strategy

  * Always squash commits.
  * Anyone is free to merge but slight preference to let the creator
    merge because they have most context. Creators, if you really care
    about the commit message, squash ahead of time and provide a nice
    commit message because someone might merge for you.


# Contributor guidelines

:+1::tada: First off, thanks for taking the time to contribute! :tada::+1:

The following is a set of guidelines for contributing to the Filecoin
Project. Feel free to propose changes, as these guidelines are a living
document.

Filecoin, including go-filecoin and all related modules, follows the
[Filecoin Code of Conduct](CODE_OF_CONDUCT.md).

### Table Of Contents
* [Developer Do's and Don'ts](#developer-dos-and-donts)
* [Code Reviews](#code-reviews)
* [Conventions and Style](#conventions-and-style)
* [Error Handling](#error-handling)
* [The Spec](#the-spec)
* [Platform Considerations](#platform-considerations)
* [Testing](#testing)
* [Profiling](#profiling)
* [What is the bar for inclusion in master?](#what-is-the-bar-for-inclusion-in-master)
* [Pull Requests](#pull-requests)
* [Gotchas](#gotchas)

## Developer Do's and Don'ts

In order to keep the project moving efficiently we ask that contributors
adhere to the following guidance:

  * **DO enable progress**: As a contributor your _#1 priority_ should
  be enabling the progress of other contributors. When we enable our
  teammates we increase our leverage and velocity. Don't be a gate-keeper, 
  someone who introduces unactionable or unnecessary obstacles to progress.
     * Corollary: *make progress*. Don't get stuck. Ask for help.
     * Corollary: *prioritize code reviews*. Progress is measured by working
     code that delivers values to users, please help people get it merged.

  * **DO NOT create technical debt**: We have passed the point in the project
  where "do the simplest thing possible that works" is a good strategy. We need
  to be completing features so that they are "done done", and not deferring
  an ever-increasing amount of work into the future. This is the only way
  to make the project converge. Half-completed features give us a false sense
  of progress and kludgy features are even worse: we end up doing the work twice,
  once as a hack and then a second time right. Build it right from the start.
    * Example: you add a cache for something. To prevent the cache
    from becoming a DOS vector it requires the addition of a tricky
    replacement policy. You are tempted to defer that implementation
    because there is a lot else to do. "Do not create technical debt"
    means that you should implement the replacement policy along with the cache,
    and not defer that work into the future (thereby creating
    technical debt).
    * Example: to add your feature you can do a hacky thing that you know is
    not a good long term solution and will be painful for whomever has to touch
    the code next. Or you could do the right thing which is going to take you
    longer. "Do not create technical debt" means do the right thing.

  * **DO capture design intent on paper, ahead of implementation, and subject
  it to constructive feedback**: explicitly identify the problem you are solving,
  the requirements and constraints the solution must meet, the proposal, its rationale,
  and any alternatives we should not pursue. This can be done in the form of a 
  [designdoc](designdocs.md), but
  really anything that captures this intent is fine. Note there is a
  [similar process for protocol-level changes](https://github.com/filecoin-project/specs/blob/master/process.md).
 
  * **DO NOT add dependencies on `Node` or add more implementation to `Node`**: The 
  `Node` has become a god object and a dependency of convenience. Abstractions
  should not take a `Node` as a dependency: they should take the narrower
  set of things they actually need. Building blocks should not go on `Node`,
  they should go into separate, smaller abstractions that depend on the 
  narrow set of things they actually need. [More detail in this issue.](https://github.com/filecoin-project/go-filecoin/issues/1223#issuecomment-433764709).
 
  * **DO embrace asynchronous workflows**: To the extent possible, drive work
  asynchronously through issues. Accept as both a strength and constraint
  of the project that sometimes the best person to do your code review is 12
  time zones away, and it might take you several days to get a PR in because of
  time zone issues.


## Code Reviews

With "enable progress" as a primary directive we can derive some
corollaries for code reviews:

  * **Avoid lengthy design discussions in PR reviews** unless the
  reviewee asks for it. Prefer to merge and spin out an issue for
  discussion if the conversation snowballs, and then follow up on the
  process failure that led to needing a big design discussion in a PR.

  * Pinging reviewers to look at a review is encouraged.

As for approvals:
  * 2 approvals for all PRs, with at least one of {@whyrusleeping, @dignifiedquire, @phritz}
  * 3/3 of the above for far-reaching/foundational or protocol level changes

### Code review protocol:
  * Comments should be actionable and clearly state what is requested.
  * By default, code review comments are advisory: reviewee
 should consider them but doesn't _have_ to respond or
 address them
  * A comment that says "BLOCKING" must be addressed and responded
 to. A reviewer has to decide how to deliver a blocking comment: via
 "Request Changes" (merge blocking) or via "Add Comments" or "Approve"
 (not merge blocking):
    * If a reviewer makes a blocking comment while blocking merge
  ("Request Changes") they are signaling that they want to have another
  look after the chages are made. Don't merge until they approve. Example:
  the whole design of an abstraction is wrong and reviewer wants to see it
  reworked.
    * If a reviewer makes a blocking comment but does not block
  merging ("Add Comments" or "Approve") then the reviewee can
  merge if the issue is addressed. Example: you have an off by one
  error. It's mandatory to fix but doesn't necessarily require
  another look from the reviewer.
  * Approval means approval even in the face of minor changes.
  github should be configured to allow merging with earlier
  approval even after rebasing/minor changes.

Comments should be blocking or come with an approval unless you are
still looking things over or you're asking for clarification. It's
ok/encouraged to ask for explanations.

## Conventions and Style

There are always exceptions but generally:
 * Comments:
   * **Use precise language.**
     * NO: "Actor is the builtin actor responsible for individual accounts". What does "responsible for" mean? What is an *individual* account?
     * YES: "Actor represents a user's account, holding its balance and nonce. A message from a user is sent from their account actor."
   * If there is a concept in the code, define it. Precisely.
   * Use complete sentences. With punctuation.
   * Comments shouldn't just say what the thing does, they should briefly say *why* it might be called or *how* it is used.
     * NO: "flushAndCache flushes and caches the input tipset's state"
     * YES: "flushAndCache flushes and caches the input tipset's state. It is called after successfully running messages to persist state changes."
 * Accessors: `Foo(), SetFoo()`
 * Predicates: `isFoo()`
 * Variable names
   * Channels: `fooCh`
   * Cids: `fooCid`
 * Test-only methods: use a parallel type
 * Logging
   * `Warning`: noteworthy but not completely unexpected
   * `Error`: a truly unexpected condition that should not happen in Real Life and that a dev should go look at
 * Protocol messages are nouns (eg, `DealQuery`, `DealResponse`) and their handlers are verbs (eg, `QueryDeal`)
 * Do not put implementation inline in command functions. Command
   implementation should be minimal, calling out functionality that
   exists elsewhere (but NOT on the node). Commands implementation is
   an important API which gets muddled when implementation happens
   inline in command functions.

We use the following import ordering.
```
import (
        [stdlib packages, alpha-sorted]
        <single, empty line>
        [gx-installed packages]
        <single, empty line>
        [go get-installed packages (not part of go-filecoin)]
        <single, empty line>
        [go-filecoin packages]
)
```

Example:

```go
import (
	"context"
	"testing"

	cmds "gx/ipfs/QmYMj156vnPY7pYvtkvQiMDAzqWDDHkfiW5bYbMpYoHxhB/go-ipfs-cmds"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)
```

## Error Handling

The overarching concern is safety of the user: do not give the user an
incorrect view of the world.

* If an error is likely a function of an input, discard the input.
* If an error could be transient, attempt to continue making progress.
* If an error appears to be permanent or we have inconsistent internal state, error out to the top level
  and exit cleanly if possible. 
* **DO NOT INTENTIONALLY CRASH THE NODE**. Don't panic if you can exit cleanly. Panicing stops the node
  from doing useful work that might be unrelated to the error and does not give subsystems an opportunity to
  clean up, thus potentially creating additional problems. 

We should log an ERROR only in truly unexpected conditions
that should not happen and that a dev should go look at.

## The Spec

The spec must be in sync with the code. 

TODO(Q4'18): we need an updated process here.

## Platform Considerations

It often makes sense to derive patterns from more established, closely related
platforms than to derive them from first principles.  For example for
things like configuration, commands, persistence, and the like we draw
heavily on patterns in IPFS. Similarly we draw on patterns in Ethereum
for message processing. Features should be informed by an awareness of
related platforms.

## Testing
* All code must be unit tested.
* Prefer to test the output/contracts.
* Daemon tests (integration tests that run a node and send it commands):
  * Daemon tests are not a substitute for unit tests: the foo command implementation should be unit tested in the `foo_test.go` file
  * Daemon tests should test integration, not comprehensive functionality
  * Daemon tests should validate that their responses conform to a JSON schema
  * Daemon tests for foo go into `foo_daemon_test.go` (so `foo.go` should have *both* `foo_test.go` and `foo_daemon_test.go`)

## Profiling

We use [pprof](https://golang.org/pkg/runtime/pprof/) to capture and visualize
performance metrics.

To capture (for example) a CPU profile, launch the daemon and then make an HTTP
request of the following form:

```shell
curl 'http://localhost:${CMDAPI_PORT}/debug/pprof/profile?seconds=15' > /tmp/profile.dump
```

Then, use pprof to view the dump:

```shell
go tool pprof /tmp/profile.dump
```

## What is the bar for inclusion in master?

Presently (Q4'18) the minimum bar is:
 * Unit tests.
 * Tests must not by flaky.
 * Must pass CI.
 * Code review (see above).
 * Lint (`go run ./build/*.go lint`).
 * Must match a subset of the spec.

## Pull Requests

* PRs should modify no more than 400 lines or 8 files. If unavoidable
  to make a larger PR, break changes into clearly scoped commits.  It
  is ok if individual commits do not build when making the PR. Only
  squashed commits are required to build.
* Always squash commits.
* Anyone is free to merge but preference to let the creator merge
  because they have most context. Creators, if you really care about
  the commit message, squash ahead of time and provide a nice commit
  message because someone might merge for you.

## Where should I start?
* Take a look at issues labelled [E-good-first-issue](https://github.com/filecoin-project/go-filecoin/issues?q=is%3Aopen+is%3Aissue+label%3AE-good-first-issue) or [E-help-wanted](https://github.com/filecoin-project/go-filecoin/issues?q=is%3Aopen+is%3Aissue+label%3AE-helped-wanted).

## Gotchas
* Equality
  * Don't use `==` to compare `*types.Block`; use `Block.Equals()`
  * Ditto for `*cid.Cid`
  * For `types.Message` use `types.MsgCidsEqual` (or better submit a PR to add `Message.Equals()`)
  * DO use `==` for `address.Address`, it's just an address

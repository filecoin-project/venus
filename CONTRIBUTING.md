# Contributor guidelines

​:+1::tada: First off, thanks for taking the time to contribute! :tada::+1:

The following is a set of guidelines for contributing to the Filecoin
Project. Feel free to propose changes, as this is a living
document.

To get a sense of what we’ve been working on recently, you might like to check out [recently closed PRs](https://github.com/filecoin-project/go-filecoin/pulls?q=is%3Apr+is%3Aclosed).

Filecoin, including go-filecoin and all related modules, follows the
[Filecoin Code of Conduct](CODE_OF_CONDUCT.md).

### Table Of Contents
* [How can I contribute?](#how-can-i-contribute)
* [What should I know before getting started?](#what-should-i-know-before-getting-started)
  * [Pull Requests](#pull-requests)
  * [Code Reviews](#code-reviews)
  * [Developer Do's and Don'ts](#developer-dos-and-donts)
* [Good First Issues](#good-first-issues)
* [Additional Developer Notes](#additional-developer-notes)
  * [Conventions and Style](#conventions-and-style)
  * [Error Handling](#error-handling)
  * [Testing](#testing)
  * [Profiling](#profiling)
  * [Gotchas](#gotchas)
  * [The Spec](#the-spec)
  * [What is the bar for inclusion in master?](#what-is-the-bar-for-inclusion-in-master)


## How can I contribute?

Here at `go-filecoin`, there’s always a lot of work to do. There are many ways you can support the project, from progamming, writing, organizing, and more. Consider these as starting points:

- **Submit bugs**: Perform a cursory [search](https://github.com/filecoin-project/go-filecoin/issues) to see if the problem has already been reported. If it does exist, add a 👍 to the issue to indicate this is also an issue for you, and add a comment if there is extra information you can contribute. If it does not exist, [create a new issue](https://github.com/filecoin-project/go-filecoin/issues/new/choose) (using the Bug report template).

- **Write code:** Once you've read this contributing guide, check out [Good First Issues](#good-first-issues) for well-prepared starter issues.

- **Improve or write documentation:** You can add feedback to [#1689](https://github.com/filecoin-project/go-filecoin/issues/1689), or submit a PR with proposed changes.

- **New ideas:** Open a thread on the [Discussion Board](https://discuss.filecoin.io/).



## What should I know before getting started?

### Pull Requests

- PRs should modify no more than 400 lines or 8 files. If a larger PR is unavoidable, break it into clearly scoped commits. It is ok if individual commits do not build when making the PR. Only squashed commits are required to build.
- Always squash commits.
- [TODO: rewrite to assume no write access] Anyone is free to merge, but we prefer to let the creator merge because they have most context. Creators: if you really care about the commit message, squash ahead of time and provide a nice commit
  message because someone might merge for you.

### Code Reviews

We use the following conventions for code reviews:

- By default, code review comments are advisory: the reviewee should consider them but doesn't _have_ to respond or address them.
- Comments that start with "BLOCKING" must be addressed and responded
  to. If a reviewer makes a blocking comment but does not block merging (by marking the review "Add Comments" or "Approve") then the reviewee can merge if the issue is addressed.
  - *Example: reviewer points out an off by one error in a blocking comment, but Approves the PR. Reviewee must fix the error, but the reviewer trusts you to do that and you can merge without further review once it is fixed.*
- If a reviewer makes a blocking comment while blocking merge
  ("Request Changes"), don't merge until they approve.
  - *Example: the whole design of an abstraction is wrong and reviewer wants to see it*
    *reworked.*

**Approvals:** `go-filecoin` requires 2 approvals for all PRs, with at least one of {@phritz, @whyrusleeping, @dignifiedquire}. If your PR hasn't been reviewed in 3 days, pinging reviewers via Github or community chat is also welcome and encouraged.

#### Code Reviewer Responsibilities:

**Avoid lengthy design discussions in PR reviews** unless the
reviewee asks for it. Prefer to merge and spin out an issue for
discussion if the conversation snowballs, and then follow up on the
process failure that led to needing a big design discussion in a PR.

### Developer Do's and Don'ts

- **DO write down design intent before writing code, and subject it to constructive feedback.** Major changes should have a [designdoc](designdocs.md). For minor changes, a good issue description may suffice.
- **DO be aware of patterns from closely related systems.** It often makes sense to draw patterns from more established, closely related platforms than to derive them from first principles. For example, we draw heavily on patterns in IPFS for things like configuration, commands, and persistence. Similarly, we draw on patterns in Ethereum for message processing. 
- **DO NOT create technical debt.** Half-completed features give us a false sense of progress.
  - Example: you add a cache for something. To prevent the cache
    from becoming a DOS vector, it requires the addition of a tricky
    replacement policy. You are tempted to defer that work
    because there is a lot else to do. "Do not create technical debt"
    means you should implement the replacement policy along with the cache,
    and not defer that work into the future.
* **DO NOT add dependencies on `Node` or add more implementation to `Node`**: The 
  `Node` has become a god object and a dependency of convenience. Abstractions
  should not take a `Node` as a dependency: they should take the narrower
  set of things they actually need. Building blocks should not go on `Node`;
  they should go into separate, smaller abstractions that depend on the 
  narrow set of things they actually need. [More detail in this issue](https://github.com/filecoin-project/go-filecoin/issues/1223#issuecomment-433764709).

## Good First Issues

Ready to begin? Here are well-prepared starter issues ([E-good-first-issue](https://github.com/filecoin-project/go-filecoin/issues?q=is%3Aopen+is%3Aissue+label%3AE-good-first-issue)) for your coding pleasure. They have clear problem statements, pointers to the right areas of the code base, and clear acceptance criteria. 

To pick up an issue:

1. **Assign** it to yourself
2. **Ask** for any clarifications via the issue, pinging in [community chat](https://github.com/filecoin-project/community#chat) as needed.
3. **Create a PR** with your changes, following the [Pull Request and Code Review guidelines]().

For continued adventures, search for issues with the label [E-help-wanted](https://github.com/filecoin-project/go-filecoin/issues?q=is%3Aopen+is%3Aissue+label%3AE-help-wanted). These are slightly thornier problems that are also reasonably well-prepared.

## Additional Developer Notes

#### Error Handling

The overarching concern is safety of the user: do not give the user an
incorrect view of the world.

* If an error is likely a function of an input, discard the input.
* If an error could be transient, attempt to continue making progress.
* If an error appears to be permanent or we have inconsistent internal state, error out to the top level
  and exit cleanly if possible. 
* **DO NOT INTENTIONALLY CRASH THE NODE**. Don't panic() if you can exit cleanly. panic()ing stops the node
  from doing useful work that might be unrelated to the error and does not give subsystems an opportunity to
  clean up, thus potentially creating additional problems. 

We should log an ERROR only in truly unexpected conditions
that should not happen and that a dev should go look at.

#### Testing

- All code must be unit tested.
- Prefer to test the output/contracts.
- Daemon tests (integration tests that run a node and send it commands):
  - Daemon tests are not a substitute for unit tests: the foo command implementation should be unit tested in the `foo_test.go` file
  - Daemon tests should test integration, not comprehensive functionality
  - Daemon tests should validate that their responses conform to a JSON schema
  - Daemon tests for foo go into `foo_daemon_test.go` (so `foo.go` should have *both* `foo_test.go` and `foo_daemon_test.go`)

#### Profiling

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

#### Gotchas

- Equality
  - Don't use `==` to compare `*types.Block`; use `Block.Equals()`
  - Ditto for `*cid.Cid`
  - For `types.Message` use `types.MsgCidsEqual` (or better submit a PR to add `Message.Equals()`)
  - DO use `==` for `address.Address`, it's just an address

#### Conventions and Style

There are always exceptions, but generally:
* Comments:
   * **Use precise language.**
     * NO: "Actor is the builtin actor responsible for individual accounts". What does "responsible for" mean? What is an *individual* account?
     * YES: "Actor represents a user's account, holding its balance and nonce. A message from a user is sent from their account actor."
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
* Protocol messages are nouns (eg, `DealQuery`, `DealResponse`). Handlers are verbs (eg, `QueryDeal`).
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

#### The Spec

The Filecoin Specification (https://github.com/filecoin-project/specs) must be in sync with the code. 

#### What is the bar for inclusion in master?

Presently (Q1'19) the minimum bar is:
* Unit tests.
* Tests must not be flaky.
* Must pass CI.
* Code review (see above).
* Lint (`go run ./build/*.go lint`).
* Must match a subset of the spec.
* Documentation is up to date.

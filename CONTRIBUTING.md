# Contributing Guidelines

​:+1::tada: First off, thanks for taking the time to contribute! :tada::+1:

The following is a set of guidelines for contributing to the Filecoin
Project. Feel free to propose changes, as this is a living
document.

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

- **Improve or write documentation:** Docs currently live in the [Wiki](https://github.com/filecoin-project/go-filecoin/wiki). You can add feedback to [#1689](https://github.com/filecoin-project/go-filecoin/issues/1689), or submit a PR with proposed changes (process coming soon).

- **New ideas:** Open a thread on the [Discussion Board](https://discuss.filecoin.io/).



## What should I know before getting started?

Check out the [Go-Filecoin code overview](CODEWALK.md) for a brief tour of the code.

### Design Before Code
- Write down design intent before writing code, and subject it to constructive feedback.
- Major changes should have a [Design Doc](designdocs.md).
- For minor changes, file an issue for your change if it doesn't yet have one, and outline your implementation plan on the issue.

### Pull Requests

- Try to keep PRs small, no more than 400 lines or 8 files.
- Always squash commits.
- For now, you'll need to request write access (via chat) to create a PR because CircleCI doesn't play well with private branches. Once we open source this repo, forking is preferred.
- Committers should merge their own PRs after Approval, because they have the most context.

### Code Reviews

`go-filecoin` requires 2 approvals for all PRs, with at least one of {@phritz, @acruikshank, @whyrusleeping}. If your PR hasn't been reviewed in 3 days, pinging reviewers via Github or community chat is also welcome and encouraged.

We use the following conventions for code reviews:

- "Approve" means approval. If there are comments and approval, it is expected that you'll address the comments before merging. Ask for clarification if you're not sure.
  - *Example: reviewer points out an off by one error in a blocking comment, but Approves the PR. Reviewee must fix the error, but the reviewer trusts you to do that.*
- "Request Changes" means you don't have approval, and the reviewer wants another look.
  - *Example: the whole design of an abstraction is wrong and reviewer wants to see it reworked.*
  
- [There is a proposal in progress to invert BLOCKING, to match most contributors' expectations.] By default, code review comments are advisory: the reviewee should consider them but doesn't _have_ to respond or address them. Comments that start with "BLOCKING" must be addressed and responded to. If a reviewer makes a blocking comment but does not block merging (by marking the review "Add Comments" or "Approve") then the reviewee can merge if the issue is addressed.


#### Code Reviewer Responsibilities:

**Avoid lengthy design discussions in PR reviews,** since major design questions should be addressed during the [Design Before Code](#design-before-code) step. If the conversation snowballs, prefer to merge and spin out an issue for discussion, and then follow up on the process failure that led to needing a big design discussion in a PR.

It is considered helpful to add blocking comments to PRs that introduce protocol changes that do not appear in the [spec](#the-spec).

### Developer Do's and Don'ts

- **DO be aware of patterns from closely related systems.** It often makes sense to draw patterns from more established, closely related platforms than to derive them from first principles. For example, we draw heavily on patterns in IPFS for things like configuration, commands, and persistence. Similarly, we draw on patterns in Ethereum for message processing. 
- **DO NOT create technical debt.** Half-completed features give us a false sense of progress.
  - *Example: you add a cache for something. To prevent the cache
    from becoming a DOS vector, it requires the addition of a tricky
    replacement policy. You are tempted to defer that work
    because there is a lot else to do. "Do not create technical debt"
    means you should implement the replacement policy along with the cache,
    and not defer that work into the future.*
* **DO NOT add dependencies on `Node` or add more implementation to `Node`**: The 
  `Node` has become a god object and a dependency of convenience. Abstractions
  should not take a `Node` as a dependency: they should take the narrower
  set of things they actually need. Building blocks should not go on `Node`;
  they should go into separate, smaller abstractions that depend on the 
  narrow set of things they actually need. [More detail in #1223](https://github.com/filecoin-project/go-filecoin/issues/1223#issuecomment-433764709).

## Good First Issues

Ready to begin? Here are well-prepared starter issues ([E-good-first-issue](https://github.com/filecoin-project/go-filecoin/issues?q=is%3Aopen+is%3Aissue+label%3AE-good-first-issue)) for your coding pleasure. They have clear problem statements, pointers to the right areas of the code base, and clear acceptance criteria. 

To pick up an issue:

1. **Assign** it to yourself.
2. **Ask** for any clarifications via the issue, pinging in [community chat](https://github.com/filecoin-project/community#chat) as needed.
3. For issues labeled `PROTOCOL BREAKING` see [the spec section](#the-spec) for additional instructions.
4. **Create a PR** with your changes, following the [Pull Request and Code Review guidelines]().

For continued adventures, search for issues with the label [E-help-wanted](https://github.com/filecoin-project/go-filecoin/issues?q=is%3Aopen+is%3Aissue+label%3AE-help-wanted). These are slightly thornier problems that are also reasonably well-prepared.

## Additional Developer Notes

#### Error Handling

The overarching concern is safety of the user: do not give the user an
incorrect view of the world.

* **DO NOT INTENTIONALLY CRASH THE NODE**. Don't panic() if you can exit cleanly. panic()ing stops the node
  from doing useful work that might be unrelated to the error and does not give subsystems an opportunity to
  clean up, thus potentially creating additional problems. 
  * If an error is likely a function of an input, discard the input.
  * If an error could be transient, attempt to continue making progress.
  * If an error appears to be permanent or we have inconsistent internal state, error out to the top level
  and exit cleanly if possible. 

We should log an ERROR only in truly unexpected conditions
that should not happen and that a dev should go look at.

#### Testing

- All new code should be accompanied by unit tests (except commands, which should be daemon tested). Prefer focussed unit tests to integration tests for thorough validation of behaviour. Existing code is not necessarily a good model, here.
- Prefer to test the output/contracts.
- Daemon tests (integration tests that run a node and send it commands):
  - Daemon tests should test integration, not comprehensive functionality
  - Daemon tests should validate that their responses conform to a JSON schema

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
* Do not put implementation in commands. Commands should call out to functionality elsewhere, in a pattern borrowed from [Git Internals: Plumbing and Porcelain](https://git-scm.com/book/en/v2/Git-Internals-Plumbing-and-Porcelain).

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

The [Filecoin Specification](https://github.com/filecoin-project/specs) must be in sync with the code.

We have some light process in place to keep the go-filecoin implementation from getting out of sync with the spec.
Most tasks are performed by the [spec shepherd](https://github.com/filecoin-project/pm/pull/107).  The following are the tasks all contributors should be aware of.

##### Filing / triaging go-filecoin issues
A `protocol change` is a modification to go-filecoin that breaks interoperability with the current version of go-filecoin.
When filing issues that involve a protocol change please flag this issue with the `PROTOCOL BREAKING` label and identify the breaking changes in the `Protocol Changes` section of the issue template.
If you do not know if an issue involves a protocol change you should just leave this section blank.
Protocol changes will be added during the triage process.

##### Picking up issues
When picking up an issue with items in the `Protocol Changes` section it is your responsibility to dig into the relevant sections of the spec.
If the spec does not have the information necessary for a developer to implement this issue it is your responsibility to file an issue, or better yet, a PR in the specs repo.
In other words the `PROTOCOL BREAKING` label **implies an additional acceptance criterion**, that **all protocol changes are documented in the spec**.
You can go ahead and implement in parallel, but note that your PR will be blocked from merging until the spec reflects this protocol change.
If a change requires enough work to warrant multiple PRs and all this work is blocked on a spec PR merging it is encouraged to submit these PRs to a branch other than master.
Then once the spec PR is merged this branch can be merged with master.
If at any point the change to the spec seems too onerous, for example if you would have to write multiple pages or a completely new section, reach out to the spec shepherd who will clarify your responsibilities and potentially move the spec work to a separate issue.
The spec shepherd may choose to overrule blocking on the spec based on relative priorities of the spec and go-filecoin projects.

#### What is the bar for inclusion in master?

Presently (Q1'19) the minimum bar is:
* Unit tests.
* Tests must not be flaky.
* Must pass CI.
* Code review (see above).
* Lint (`go run ./build/*.go lint`).
* Must match a subset of the spec.
* Documentation is up to date.

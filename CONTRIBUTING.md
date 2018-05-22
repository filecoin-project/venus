# Contributor guidelines

:+1::tada: First off, thanks for taking the time to contribute! :tada::+1:

The following is a set of guidelines for contributing to the Filecoin
Project. Feel free to propose changes to this document in a pull
request. These guidelines can and should change as the project changes
phases.

Also: these guidelines should not replace common sense. The golden rule
is __if something feels wrong, stop and surface the issue.__

### Table Of Contents
* [Enable Progress](#enable-progress)
* [Code Reviews](#code-reviews)
* [Project Management](#project-management)
* [Conventions and Style](#conventions-and-style)
* [Error Handling](#error-handling)
* [The Spec](#the-spec)
* [Platform Considerations](#platform-considerations)
* [Testing Philosophy](#testing-philosophy)
* [What is the bar for inclusion in master?](#what-is-the-bar-for-inclusion-in-master)
* [Pull Requests](#pull-requests)
* [Gotchas](#gotchas)

## Enable Progress

A primary directive for every dev is to _enable other devs to make
progress_. We increase our leverage and velocity by enabling our
teammates. Be an enabler -- find ways to help the person make progress.
Don't be a gate-keeper -- someone who introduces un-actionable
or unnecessary obstacles to progress. Prioritize code reviews and
answering questions over your own work to a reasonable extent.
Prioritize progress over perfection when possible.

On the flip side: make progress. If you need help, ask for it.  Ask
for immediate feedback in slack, a short video meeting, or a group
code walk-through if it would be helpful.

## Code Reviews

With "prioritize progress" as a primary directive we can derive some
corollaries for code reviews: 

  * Unless a reviewee asks for it, **avoid lengthy design discussions
  in PR reivews**. Design discussions in PRs shouldn't consume a lot
  of time or be open-ended. If it seems like something is going to
  take more than a few quick iterations or would require sweeping
  changes, prefer to merge and defer the design discussion to a follow
  up issue (and then follow up). Recognize that when this happens it
  might point to a process failure where design should've been
  discussed earlier. If that's the case go fix that problem as well.

  Exercise judgement about when not to defer design discussion. For
  example if deferring would ultimately require lots of painful
  refactoring or undoing lots of work, consider not deferring.
  
  * Limit scope of comments to the story itself: avoid feature creep.
  As above, prefer to defer the addition of new features to followup 
  work.

  * Get comfortable with "good enough" and recognize that "good
  enough" is context-dependent. For example "good enough" is different
  for v0 block explorer than for the consensus algorithm.  Ask
  yourself if, given the context, whether the details you're about to
  ask someone to spend time changing will materially affect the
  success of the project.  If not, consider holding off.

As for approvals:
  * 2 approvals for all PRs (except typos, etc) with at least one of {@whyrusleeping, @dignifiedquire, @phritz}
  * 3/3 for far-reaching/foundational or protocol level changes

In order to minimize the time wasted in code review and handoff we have
the following protocol:
  * prefer to go to in-person/voice if the PR is open >= 3 days
  * pinging reviewers to look at a PR considered useful and encouraged
  * comments should be actionable and clearly state what is requested
  * by default code review comments are advisory: reviewee
 should consider them but doesn't _have_ to respond or
 address them
  * a comment that says "BLOCKING" must be addressed
 and responded to. A reviewer has to decide how 
 to deliver a blocking comment: via "Request Changes" (merge blocking) or via
 "Add Comments" or "Approve" (not merge blocking):
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

**Do not just leave comments in a code review.** Comments should be
blocking or come with an approval unless you are still looking things
over or you're asking for clarification. It's ok/encouraged to ask
for explanations. The thing we want to avoid is *unnecessarily* 
requiring mutiple round trips from someone whose next availability
 might be 12 hours away.

## Project Management

* We use a modified Agile scrum method. Details are in [Agile and Scrum at Protocol Labs - Filecoin](https://docs.google.com/document/d/16PQVR5vTnbt6DtgxVIEdjc4n0TrqWEtOiWZLItQXC74/edit?ts=5ab28811#).
* We track work items via the [Project Board](https://app.zenhub.com/workspace/o/filecoin-project/go-filecoin/boards?repos=113219518). We use Zenhub because it provides agile project management tools, but integrates directly with Github.

| Stage | What is it for? |
|---|---|
| Icebox | Things we decided not to do for now. |
| Backlog | New issues. |
| Ready | Issues move here after they have good descriptions, acceptance criteria, and initial estimates. |
| This Sprint | Selected by team for current sprint. Looking for work? Start here. |
| In Progress | When you start work on an issue, move here and assign yourself. |
| Review/QA | For PRs and issues. Move here manually when you create a PR. |
| Blocked | If you are working on something but get stuck because of external factors. |
| Closed | :tada:|

- The first and last stages are kept up-to-date automatically. 
   - New issues created in `go-filecoin` show up automatically in `Backlog`
   - When a PR is merged, the referenced issues are closed and moved to `Closed`.
- All other stages are updated manually.
- All work you are doing should be tracked through an issue. If an issue doesn't exist, create one.

### Picking up a story

In order to surface design constraints as early as possible clarify an
implementation plan ahead of implementation with someone knowledgable
of the issue. That is, when you pick up a story spend some time with
it and then clarify what you think should happen with someone familiar
with the issue and code. Typically this is the person who filed the
issue but a different stakeholder could be called out explicitly in
the issue or found in slack. Reflect any decisions that were made or
directives you receive back into the issue to avoid misunderstandings.
Only implement once you're sure you have a solid plan.

## Conventions and Style

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
 * Protocol messages are nouns (eg, `DealQuery`, `DealResponse`) and their handlers are verbs (eg, `QueryDeal`)
 * Do not put implementation inline in command functions. Command implementation should be minimal, calling out functionality that exists elsewhere (eg on the node). Commands implementation is an important API which gets muddled when implementation happens inline in command functions.

## Error Handling

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

## Platform Considerations

Prefer to derive patterns from more established, closely related
platforms than to derive them from first principles.  For example for
things like configuration, commands, persistence, and the like we draw
heavily on patterns in IPFS. Similarly we draw on patterns in Ethereum
for message processing. Features should be informed by an awareness of
related platforms.

## Testing Philosophy
* All code must be unit tested and should hit our target coverage rate (80%). 
* We prefer to test the output/contracts, not the individual lines of code (which we expect to change significantly during early work).
* Daemon tests (integration tests that run a node and send it commands):
  * Daemon tests are not a substitute for unit tests: the foo command implementation should be unit tested in the `foo_test.go` file
  * Daemon tests should test integration, not comprehensive functionality
  * Daemon tests should validate that their responses conform to a JSON schema
  * Daemon tests for foo go into `foo_daemon_test.go` (so `foo.go` should have *both* `foo_test.go` and `foo_daemon_test.go`)


## What is the bar for inclusion in master?

Presently (Q1'18) the minimum bar is:
 * Must be unittested (80% target, some slippage ok); if PR is a command, it must be command tested
 * Code review (see above)
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

## Pull Requests

#### Workflow
* When creating a pull request:
  * Move the corresponding issue into `Review/QA`
  * Assign reviewers to the pull request
  * Place pull request into `This Sprint`.
* Once something is merged to `master` close the issue and move it to `Closed`. (Closing an issue moves it to `Closed` automatically)
* In PRs and commit messages, reference the issue(s) you are addressing with [keywords](https://help.github.com/articles/closing-issues-using-keywords/) like `Closes #123`.

#### Size
* PRs should modify no more than 400 lines or 8 files
* If unavoidable to make a larger PR break changes into clearly scoped commits.  It is ok if individual commits do not build when making the PR, only squashed commits are required to build.

#### Style
* Always squash commits.
* Anyone is free to merge but slight preference to let the creator
    merge because they have most context. Creators, if you really care
    about the commit message, squash ahead of time and provide a nice
    commit message because someone might merge for you.

## Gotchas
* Equality
  * Don't use `==` to compare `*types.Block`; use `Block.Equals()`
  * Ditto for `*cid.Cid`
  * For `types.Message` use `types.MsgCidsEqual` (or better submit a PR to add `Message.Equals()`)
  * DO use `==` for `types.Address`, it's just an address



<p align="center">
  <a href="https://venus.filecoin.io/Overview.html" title="Filecoin Docs">
    <img src="documentation/images/venus_logo_big2.jpg" alt="Project Venus Logo" width="330" />
  </a>
</p>



<h1 align="center">Project Venus - 启明星</h1>

<p align="center">
  <a href="https://circleci.com/gh/filecoin-project/venus"><img src="https://circleci.com/gh/filecoin-project/venus.svg?style=svg"></a>
  <a href="https://github.com/filecoin-project/venus/releases/latest"><img src="https://img.shields.io/endpoint.svg?color=brightgreen&style=flat&logo=GitHub&url=https://raw.githubusercontent.com/filecoin-project/go-filecoin-badges/master/user-devnet.json"></a>
  <a href="https://github.com/filecoin-project/venus/releases"><img src="https://img.shields.io/endpoint.svg?color=blue&style=flat&logo=GitHub&url=https://raw.githubusercontent.com/filecoin-project/go-filecoin-badges/master/nightly-devnet.json" /></a>  
  <a href="https://github.com/filecoin-project/venus/releases"><img src="https://img.shields.io/endpoint.svg?color=brightgreen&style=flat&logo=GitHub&url=https://raw.githubusercontent.com/filecoin-project/go-filecoin-badges/master/staging-devnet.json" /></a>
  <br>
</p>

Venus is an implementation of the Filecoin Distributed Storage Network. For more details about Filecoin, check out the [Filecoin Spec](https://spec.filecoin.io).

## Building & Documentation

For instructions on how to build, install and join a venus storage pool, please visit [here](https://venus.filecoin.io/guide/Using-venus-Shared-Modules.html).

## Venus architecture

With key features like security, ease of use and distributed storage pool, the deployment of a node using Venus is quite different from the one using [Lotus](https://github.com/filecoin-project/lotus). Details of mining architecture can be found [here](https://venus.filecoin.io/guide/#how-venus-works).

## Related modules

Venus loosely describes a collection of modules that work together to realize a fully featured Filecoin implementation. List of stand-alone venus modules repos can ben found [here](https://venus.filecoin.io/guide/Using-venus-Shared-Modules.html#introducing-venus-modules), each assuming different roles in the functioning of Filecoin.

## Contribute

Venus is a universally open project and welcomes contributions of all kinds: code, docs, and more. However, before making a contribution, we ask you to heed these recommendations:

1. If the proposal entails a protocol change, please first submit a [Filecoin Improvement Proposal](https://github.com/filecoin-project/FIPs).
2. If the change is complex and requires prior discussion, [open an issue](https://github.com/filecoin-project/venus/issues) or a [discussion](https://github.com/filecoin-project/venus/discussions) to request feedback before you start working on a pull request. This is to avoid disappointment and sunk costs, in case the change is not actually needed or accepted.
3. Please refrain from submitting PRs to adapt existing code to subjective preferences. The changeset should contain functional or technical improvements/enhancements, bug fixes, new features, or some other clear material contribution. Simple stylistic changes are likely to be rejected in order to reduce code churn.

When implementing a change:

1. Adhere to the standard Go formatting guidelines, e.g. [Effective Go](https://golang.org/doc/effective_go.html). Run `go fmt`.
2. Stick to the idioms and patterns used in the codebase. Familiar-looking code has a higher chance of being accepted than eerie code. Pay attention to commonly used variable and parameter names, avoidance of naked returns, error handling patterns, etc.
3. Comments: follow the advice on the [Commentary](https://golang.org/doc/effective_go.html#commentary) section of Effective Go.
4. Minimize code churn. Modify only what is strictly necessary. Well-encapsulated changesets will get a quicker response from maintainers.
5. Lint your code with [`golangci-lint`](https://golangci-lint.run) (CI will reject your PR if unlinted).
6. Add tests.
7. Title the PR in a meaningful way and describe the rationale and the thought process in the PR description.
8. Write clean, thoughtful, and detailed [commit messages](https://chris.beams.io/posts/git-commit/). This is even more important than the PR description, because commit messages are stored _inside_ the Git history. One good rule is: if you are happy posting the commit message as the PR description, then it's a good commit message.

## License

This project is dual-licensed under [Apache 2.0](https://github.com/filecoin-project/venus/blob/master/LICENSE-APACHE) and [MIT](https://github.com/filecoin-project/venus/blob/master/LICENSE-MIT).

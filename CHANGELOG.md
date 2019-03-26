# go-filecoin changelog

## go-filecoin 0.1.4

We're happy to announce go-filecoin 0.1.4. This release contains a better install experience, Proof-of-Spacetime integration, more reliable message sending and networking, and many other improvements. Get pumped! 🎁

### Install and Setup

#### 💝 Binary releases

Linux and MacOS binaries for go-filecoin are now available! See [Installing from binary](https://github.com/filecoin-project/go-filecoin/wiki/Getting-Started#installing-from-binary) for instructions.

#### 🍱 Precompiled proofs parameters

Running secure proofs requires parameter files that are several GB in size. Previously, these files were generated at install, an extremely memory-intensive process causing slow or impossible builds for many users. Now, you can download pre-generated files during install by running `paramfetch`. This step is now included in the [Installing from binary](https://github.com/filecoin-project/go-filecoin/wiki/Getting-Started#installing-from-binary) instructions.

#### 🦖 Version checking

go-filecoin now checks that it is running the same version (at the same commit) before connecting to a devnet. This is a temporary fix until a model for change is implemented, allowing different versions to interoperate.

### Features

#### 💎 Proof-of-Spacetime Integration

For each storage deal, miners now call `rust-fil-proofs` to periodically generate proofs of continued storage. With this major integration in place, you can expect some follow-up  (for example, storage mining faults do not yet appear on-chain) and continued optimizations to the underlying Proof-of-Spacetime implementation.

### Performance and Reliability

#### 🤝 Networking

We’ve upgraded to [go-libp2p](http://github.com/libp2p/go-libp2p) 6.0.35 which has fixed autorelay reliability issues. We’ve also added a `go-filecoin dht` command for interacting with and debugging our dht.  

#### 🎈 Better message sending

In the past, if messages failed, they failed silently. go-filecoin would continue to select nonces higher than the sent message, effectively deadlocking message sending. We have now implemented several improvements to message sending: incoming and outgoing queues, better nonce selection logic, and a message timeout after a certain number of blocks. See [message status](https://github.com/filecoin-project/go-filecoin/blob/6a34245644cd62436239b885cd7ba1f0f29d0ca5/commands/message.go) and mpool ls/show/rm commands for more.

#### 🔗 Chain syncing is faster

Chain is now faster due to use of bitswap sessions. Woohoo!

#### ⌛ Context deadline errors fixed

In the past, the context deadline was set artificially low for file transfer. This caused some large file transfers to time out, preventing storage deals from being completed. Thank you to @markwylde, @muronglaowang, @pengxiankaikai, @sandjj, and others for bug reports.

### Refactors and Endeavors

#### 🦊 FAST (Filecoin Automation & System Toolkit)

FAST is a common library of go-filecoin code that can be used in daemon testing, devnet initialization, and other applications like network randomization that involve managing nodes, running commands against them, and observing their state.

Using FAST, we’ve developed [localnet](https://github.com/filecoin-project/go-filecoin/tree/master/tools/fast/bin/localnet), a new tool to quickly and easily set up a local network for testing, debugging, development, and more. Want to give it a whirl? Check out the [localnet README](https://github.com/filecoin-project/go-filecoin/tree/master/tools/fast/bin/localnet).

#### 👾 Porcelain/Plumbing refactor for node object

Previously, the node object contained both interfaces and internals for much of the core protocol. It was difficult to unit test due to many dependencies and complicated setup. Following the [porcelain and plumbing pattern from Git](https://git-scm.com/book/en/v2/Git-Internals-Plumbing-and-Porcelain), we have now decoupled the node object from many of its dependencies. We have also created a separate API for block storage and retrieval mining.

### Changelog

A full list of [all 82 PRs in this release](https://github.com/filecoin-project/go-filecoin/pulls?utf8=%E2%9C%93&q=is%3Apr+merged%3A%3E%3D2019-02-14+-merged%3A%3E%3D2019-03-06) can be found on Github.

### Contributors

❤️ Huge thank you to everyone that made this release possible! By alphabetical order, here are all the humans who contributed to the release:

PENDING

### 🙌🏽 Want to contribute?

Would you like to contribute to the Filecoin project and don’t know how? Here are a few places you can get started:

- Check out the [Contributing Guidelines](https://github.com/filecoin-project/go-filecoin/blob/master/CONTRIBUTING.md)
- Look for issues with the `good-first-issue` label in [go-filecoin](https://docs.google.com/document/d/1dfTVASs9cQMo4NPqJmXjEEX-Ju_M9Vw-4AelN1aHOV8/edit#) and [rust-fil-proofs](https://github.com/filecoin-project/rust-fil-proofs/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22)
- Join the [community chat on Matrix/Slack](https://github.com/filecoin-project/community#chat), introduce yourself in #_fil-lobby, and let us know where you would like to contribute

### ⁉️ Do you have questions?

The best place to ask your questions about go-filecoin, how it works, and what you can do with it is at [discuss.filecoin.io](https://discuss.filecoin.io). We are also available at the [community chat on Matrix/Slack](https://github.com/filecoin-project/community#chat).

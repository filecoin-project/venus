**Note: THE FILECOIN PROJECT IS STILL EXTREMELY CONFIDENTIAL. Do not share or discuss the project outside of designated preview channels (chat channels, Discourse forum, GitHub, emails to Filecoin team), not even with partners/spouses/family members. If you have any questions, please email [legal@protocol.ai](mailto:legal@protocol.ai).**

# Filecoin (go-filecoin)

[![CircleCI](https://circleci.com/gh/filecoin-project/go-filecoin.svg?style=svg&circle-token=5a9d1cb48788b41d98bdfbc8b15298816ec71fea)](https://circleci.com/gh/filecoin-project/go-filecoin)

> Filecoin implementation in Go, turning the world’s unused storage into an algorithmic market.

## Table of Contents

- [What is Filecoin?](#what-is-filecoin)
- [Install](#install)
  - [System Requirements](#system-requirements)
  - [Install from Source](#install-from-source)
    - [Install Go and Rust](#install-go-and-rust)
    - [Clone Repository](#clone-repository)
    - [Install Dependencies](#install-dependencies)
    - [Build, Run Tests, and Install](#build-run-tests-and-install)
      - [Manage Submodules Manually](#manage-submodules-manually-optional)
- [Usage](#usage)
   - [Sample Commands](#sample-commands)
   - [Helpful Environment Variables](#helpful-environment-variables)
   - [Advanced Usage](#advanced-usage)
     - [Run Multiple Nodes with IPTB](#run-multiple-nodes-with-iptb)
- [Clusters](#clusters)
- [Contributing](#contributing)
- [Community](#community)
- [License](#license)

## What is Filecoin?
Filecoin is a decentralized storage network that turns the world’s unused storage into an algorithmic market, creating a permanent, decentralized future for the web. **Miners** earn the native protocol token (also called “filecoin”) by providing data storage and/or retrieval. **Clients** pay miners to store or distribute data and to retrieve it. Check out [How Filecoin Works](https://github.com/filecoin-project/go-filecoin/wiki/1.-How-Filecoin-Works) for more.

## Install

👋 Welcome to Go Filecoin!

- To **run** `go-filecoin` for mining, storing or other exploring, jump straight to
  [detailed setup instructions](https://github.com/filecoin-project/go-filecoin/wiki/2.-Getting-Started).
- To **build** `go-filecoin` from source for development, keep following this README.

### System Requirements
Filecoin can build and run on most Linux and MacOS systems. Windows is not yet supported.

### Install from Source

#### Install Go and Rust

  - The build process for go-filecoin requires at least [Go](https://golang.org/doc/install) version 1.11.2. If you're setting up Go for the first time, we recommend [this tutorial](https://www.ardanlabs.com/blog/2016/05/installing-go-and-your-workspace.html) which includes environment setup.
  - go-filecoin also needs `pkg-config` to handle generating linker flags
  	- Linux devs are good to go
  	- Mac OS devs can install through brew `brew install pkg-config`
  - If you want to build rust-proofs from source (instead of using the 
    precompiled libfilecoin_proofs library), you'll need 
    [Rust](https://www.rust-lang.org/) (v1.29.0 or later).
  - Due to our use of cgo, you'll need a C compiler to build go-filecoin whether 
    you're using a prebuilt libfilecoin_proofs (our cgo-compatible rust-proofs
    library) or building it yourself from source. If you want to use GCC (e.g.
    `export CC=gcc`) when building go-filecoin, you will need to use version
    7.4.0 or higher.
  - You must have libclang on you linker search path in order to build
    rust-proofs from source. You can satisfy this requirement in most
    environments by installing Clang using your favorite package manager.

#### Clone Repository

```sh
mkdir -p ${GOPATH}/src/github.com/filecoin-project
git clone git@github.com:filecoin-project/go-filecoin.git ${GOPATH}/src/github.com/filecoin-project/go-filecoin
```

#### Install Dependencies

`go-filecoin` depends on some proofs code written in Rust, housed in the 
[rust-proofs](https://github.com/filecoin-project/rust-proofs) repo and consumed as a submodule. If you're not
planning to edit the proofs, you can fetch pre-compiled assets for your build rather than compiling them locally.

To do so, set these two environment variables before fetching dependencies:
  - `GITHUB_TOKEN` - a personal access token generated on GitHub
  - `FILECOIN_USE_PRECOMPILED_RUST_PROOFS=1`
  
go-filecoin's dependencies are managed by [gx][2]; this project is not "go gettable." To install gx, gometalinter, and
other build and test dependencies, run:

```sh
cd ${GOPATH}/src/github.com/filecoin-project/go-filecoin
go run ./build/*.go deps
```

### Build, Run Tests, and Install

```sh
# First, build the binary...
go run ./build/*.go build

# Then, run the tests.
go run ./build/*.go test

# Build and test can be combined!
go run ./build/*.go best

# Install go-filecoin to ${GOPATH}/bin
go run ./build/*.go install
```

Other handy build commands include:

```sh
# Check the code for style and correctness issues
go run ./build/*.go lint

# Test with a coverage report
go run ./build/*.go test -cover

# Test with Go's race-condition instrumentation and warnings (see https://blog.golang.org/race-detector)
go run ./build/*.go test -race

# Deps, Lint, Build, Test (any args will be passed to `test`)
go run ./build/*.go all
```

Note: Any flag passed to `go run ./build/*.go test` (e.g. `-cover`) will be passed on to `go test`.

**If you have problems with the build, please see the [Troubleshooting & FAQ](https://github.com/filecoin-project/go-filecoin/wiki/8.-Troubleshooting-&-FAQ) Wiki page.**


#### Manage Submodules Manually (optional)

If you're editing `rust-proofs`, you need to manage the submodule manually. If you're *not* editing `rust-proofs` you can relax: 
`deps` build (above) will do it for you. You may need to run `deps` again after pulling master if the submodule is
updated by someone else (it will appear modified in `git status`).

To initialize the submodule:

```sh
cd ${GOPATH}/src/github.com/filecoin-project/go-filecoin
git submodule update --init
```

Later, when the head of the `rust-proofs` `master` branch changes, you may want to update `go-filecoin` to use these changes:

```sh
git submodule update --remote
```

Note that updating the `rust-proofs` submodule in this way will require a commit to `go-filecoin` (changing the submodule hash).

## Usage

The [Getting Started](https://github.com/filecoin-project/go-filecoin/wiki/2.-Getting-Started) wiki page contains
a simple sequence to get your Filecoin node up and running and connected to a cluster.

The [Commands](https://github.com/filecoin-project/go-filecoin/wiki/7.-Commands) page contains further detail about
specific commands and environment variables, as well as scripts for for setting up a miner and making a deal.

To see a full list of commands, run `go-filecoin --help`.

### Advanced usage

#### Run Multiple Nodes with IPTB

The [`localfilecoin` IPTB plugin](https://github.com/filecoin-project/go-filecoin/tree/master/tools/iptb-plugins) provides an automation layer that makes it easy to run multiple filecoin nodes. For example, it enables you to easily start up 10 mining nodes locally on your machine.

## Contributing

We ❤️ all our contributors; this project wouldn’t be what it is without you! If you want to help out, please see [CONTRIBUTING.md](CONTRIBUTING.md).

## Community

Here are a few places to get help and connect with the Filecoin community:
- [Documentation Wiki](https://github.com/filecoin-project/go-filecoin/wiki) — for tutorials, troubleshooting, and FAQs
- #fil-dev on [Filecoin Project Slack](https://filecoinproject.slack.com/messages/CEHHJNJS3/) or [Matrix/Riot](https://riot.im/app/#/room/#fil-dev:matrix.org) - for live help and some dev discussions
- [Filecoin Community Forum](https://discuss.filecoin.io) - for talking about design decisions, use cases, implementation advice, and longer-running conversations
- [GitHub issues](https://github.com/filecoin-project/go-filecoin/issues) - for now, use only to report bugs, and view or contribute to ongoing development. PRs welcome! Please see [our contributing guidelines](CONTRIBUTING.md).

Looking for even more? See the full rundown at [filecoin-project/community](https://github.com/filecoin-project/community).

#

## Clusters

Both the Test and Nightly clusters run with `FIL_USE_SMALL_SECTORS=true`, this means that the proofs system only ever seals the first 127 bytes of a sector. Because of this users connecting to the cluster must:

- Set `FIL_USE_SMALL_SECTORS=true` before starting the daemon, else they risk experiencing undefined behavior.
- Only store files that are <= 127 bytes.

To get an idea of the state of each cluster, you can visit the cluster dashboard (port 8010).
Once the page is loaded, wait about 5 to 10 seconds for heartbeats to arrive. A healthy
cluster should show 5 nodes with the nickname "boot".

Each cluster is setup to mine a new block about every 30 seconds. If the "Last Block" column
on the far right remains blank, or the number increments far beyond the 30 second mining time,
this indociates that the cluster is no longer mining, or is not mining at the desired rate.

Another indication that a cluster is not in a healthy state will be indicated by the background
color of the tipset value. If all colors are the same, the nodes are in consensus. Two or more
colors that persist may indicate that the nodes have fallen out of consensus.

### Nightly Cluster

Deployed from master by CI every day at 0600 UTC. **If you want to use a cluster
you should probably be using this one.**

- Faucet: http://nightly.kittyhawk.wtf:9797/
- Dashboard: http://nightly.kittyhawk.wtf:8010/
- Genesis File: http://nightly.kittyhawk.wtf:8020/genesis.car
- Block explorer: http://nightly.kittyhawk.wtf:8000/
- Prometheus Endpoint: http://nightly.kittyhawk.wtf:9082/metrics
- Connected Nodes PeerID's: http://nightly.kittyhawk.wtf:9082/nodes

The [Getting Started](https://github.com/filecoin-project/go-filecoin/wiki/2.-Getting-Started) wiki page contains
instructions for connecting to the nightly cluster.

### Test Cluster (for Infra)

Deployed via CI by tagging a commit with `redeploy_test_cluster`. **This cluster
is for people working on infra. You should probably avoid it unless that describes you.**

- Faucet: http://test.kittyhawk.wtf:9797/
- Dashboard: http://test.kittyhawk.wtf:8010/
- Genesis File: http://test.kittyhawk.wtf:8020/genesis.car
- Block explorer: http://test.kittyhawk.wtf:8000/
- Prometheus Endpoint: http://test.kittyhawk.wtf:9082/metrics
- Connected Nodes PeerID's: http://test.kittyhawk.wtf:9082/nodes

To connect, follow the instructions in [Getting Started](https://github.com/filecoin-project/go-filecoin/wiki/2.-Getting-Started),
but replace `nightly.kittyhawk.wtf` with `test.kittyhawk.wtf`.

#

## License

The Filecoin Project is dual-licensed under Apache 2.0 and MIT terms:

- Apache License, Version 2.0, ([LICENSE-APACHE](https://github.com/filecoin-project/go-filecoin/blob/master/LICENSE-APACHE) or http://www.apache.org/licenses/LICENSE-2.0)
- MIT license ([LICENSE-MIT](https://github.com/filecoin-project/go-filecoin/blob/master/LICENSE-MIT) or http://opensource.org/licenses/MIT)


[1]: https://golang.org/dl/
[2]: https://github.com/whyrusleeping/gx
[3]: https://github.com/RichardLitt/standard-readme
[4]: https://golang.org/doc/install
[5]: https://www.rust-lang.org/en-US/install.html

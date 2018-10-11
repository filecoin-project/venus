**Note: THE FILECOIN PROJECT IS STILL EXTREMELY CONFIDENTIAL. Do not share anything outside of Protocol Labs. Do not discuss anything related to Filecoin outside of Protocol Labs, not even with your partners/spouses/other family members. If you have any questions about what can be discussed, please email [legal@protocol.ai](mailto:legal@protocol.ai).**

# Filecoin (go-filecoin)

> Filecoin implementation in Go

Filecoin is a decentralized storage network that turns cloud storage into an algorithmic market. The
market runs on a blockchain with a native protocol token (also called "filecoin" or FIL), which miners earn
by providing storage to clients.

👋Trying out the project for the first time? We recommend heading to the [Wiki](https://github.com/filecoin-project/go-filecoin/wiki/) for more detailed instructions and guides.

## Table of Contents

- [Installation](#installation)
- [Development](#development)
  - [Install Go and Rust](#install-go-and-rust)
  - [Clone](#clone)
  - [Install Dependencies](#install-dependencies)
  - [Managing Submodules](#managing-submodules)
  - [Running tests](#running-tests)
  - [Build Commands](#build-commands)
- [Running Filecoin](#running-filecoin)
   - [Running multiple nodes with IPTB](#running-multiple-nodes-with-iptb)
   - [Sample Commands](#sample-commands)
- [Community](#community)
- [License](#license)

## Installation

You can download prebuilt binaries for Linux and MacOS from CircleCI.

  - Go to the [filecoin project page on CircleCI](https://circleci.com/gh/filecoin-project/go-filecoin/tree/master). (You may need to authenticate with GitHub first.)
  - Click on the most recent successful build for your OS (`build_linux` or `build_macos`)
  - Click the 'Artifacts' tab.
  - Click `Container 0 > filecoin.tar.gz` to download the release.

## Development

### Install Go and Rust

  - The build process for go-filecoin requires at least [Go](https://golang.org/doc/install) version 1.10. If you're setting up Go for the first time, we recommend [this tutorial](https://www.ardanlabs.com/blog/2016/05/installing-go-and-your-workspace.html) which includes environment setup.  
  - You'll also need Rust (v1.29.0 or later) to build the `rust-proofs` submodule. You can download it [here](https://www.rust-lang.org/).

### Clone

```sh
> mkdir -p ${GOPATH}/src/github.com/filecoin-project
> git clone git@github.com:filecoin-project/go-filecoin.git ${GOPATH}/src/github.com/filecoin-project/go-filecoin
```

### Install Dependencies

go-filecoin's dependencies are managed by [gx][2]; this project is not "go gettable." To install gx, gometalinter, and
other build and test dependencies, run:

```sh
> cd ${GOPATH}/src/github.com/filecoin-project/go-filecoin
> go run ./build/*.go deps
```

### Managing Submodules

This step is necessary if you want to edit `rust-proofs`. If you're not editing `rust-proofs` there's no need to do this manually, because the `deps` build step will do it for you.

Filecoin uses Git Submodules to consume `rust-proofs`. To initialize the submodule, either run `deps` (as per above), or
initialize the submodule manually:

```sh
> cd ${GOPATH}/src/github.com/filecoin-project/go-filecoin
> git submodule update --init
```

Later, when the head of the `rust-proofs` `master` branch changes, you may want to update `go-filecoin` to use these changes:

```sh
> git submodule update --remote
```

Note that updating the `rust-proofs` submodule in this way will require a commit to `go-filecoin` (changing the submodule hash).

### Running Tests

The filecoin binary must be built prior to testing changes made during development. To do so, run:

```sh
> go run ./build/*.go build
```

Then, run the tests:

```sh
> go run ./build/*.go test
```

Note: Build and test can be combined:

```sh
> go run ./build/*.go best
```

### Build Commands

```sh
# Build
> go run ./build/*.go build

# Install
> go run ./build/*.go install

# Test
> go run ./build/*.go test

# Build & Test
> go run ./build/*.go best

# Coverage
> go run ./build/*.go test -cover

# Lint
> go run ./build/*.go lint

# Race
> go run ./build/*.go test -race

# Deps, Lint, Build, Test (with args passed to Test)
> go run ./build/*.go all
```

Note: Any flag passed to `go run ./build/*.go test` (e.g. `-cover`) will be passed on to `go test`.

## Running Filecoin

```
rm -fr ~/.filecoin      # <== optional, in case you have a pre-existing install
go-filecoin init        # Creates config in ~/.filecoin; to see options: `go-filecoin init --help`
go-filecoin daemon      # Starts the daemon, you may now issue it commands in another terminal
```

To set up a single node capable of mining:
```
rm -fr ~/.filecoin
go-filecoin init --genesisfile ./fixtures/genesis.car
go-filecoin daemon
# Note the output of the daemon, it should say "My peer ID is <W>", where <W>
# is a long cid string starting with "Qm".  <W> is used in a later command.
# Switch terminals
# The miner is present in the genesis block car file created from the 
# json file, but the node is not yet configured to use it. Get the 
# miner address from the json file fixtures/gen.json and replace <X>
# in the command below with it:
go-filecoin config mining.minerAddress '"<X>"'
# The account that owns the miner is also not yet configured in the node
# so note that owner key name in fixtures/gen.json, we'll call it <Y> for short,
# and import that key from the fixtures:
go-filecoin wallet import fixtures/<Y>.key
# Note the output of this command, call it <Z>. This output is the address of 
# the account that owns the miner.
# The miner was not created with a pre-set peerid, so set it so that
# clients can find it.
go-filecoin miner update-peerid --from=<Z> <X> <W>
# Now you can run a lookup
go-filecoin address lookup <X>
# the output should now be <W>
```

To configure a node's auto-sealing scheduler:
```
# The auto-sealer is used to automatically seal the data that a miner received
# from a client into a sector. Without this feature, miners would wait until
# they had accumulated $SECTORSIZE worth of client data before initiating the
# sealing process so that they didn't waste precious hard drive space. With
# auto-sealing enabled, the miner will use $SECTORSIZE bytes of storage each
# period unless there are no data to store that period.
#
# To control how frequently the auto-sealer runs, provide a positive integer
# value for the --auto-seal-interval-seconds option. To disable this feature,
# provide a 0.
#
# If the option is omitted, a default of 120 seconds will be used.
#
rm -fr ~/.filecoin
go-filecoin init --auto-seal-interval-seconds=0 --genesisfile ./fixtures/genesis.car
go-filecoin daemon
```

#### Running multiple nodes with IPTB

IPTB provides an automtion layer that makes it easy run multiple filecoin nodes. 
For example, it enables you to easily start up 10 mining nodes locally on your machine.
Please refer to the [README.md](https://github.com/filecoin-project/go-filecoin/blob/master/tools/iptb-plugins/README.md).

#### Sample commands

```
# ----- List and ping a peer ----- 
go-filecoin swarm peers
go-filecoin ping <peerID>

#  ----- View latest mined block ----- 
go-filecoin chain head
go-filecoin show block <blockID> | jq

#  ----- Create a miner ----- 
# Requires the node be a part of a cluster that already has miners and 
# no miner configured for this node yet.
go-filecoin miner create 10 10
# Waits for the message to be included on chain, updates the minerAddress in the
# node's config, and sets the peerid appropriately.
# Get your miner address
go-filecoin config mining.minerAddress
# And the owner:
go-filecoin miner owner <minerAddress>

#  ----- As a miner, force a block to be mined immediately ----- 
go-filecoin mine once

#  ----- As a miner, make an ask ----- 
# Get your miner address
go-filecoin config mining.minerAddress
# Get your miner owner address 
go-filecoin miner owner <minerAddress>
go-filecoin miner add-ask <minerAddress> <size> <price> --from=<ownerAddress>
# Wait for the block to be mined (~30s) and view the ask:
go-filecoin orderbook asks | jq

#  ----- As a client, make a deal ----- 
echo "Hi my name is $USER"> hello.txt
go-filecoin client import ./hello.txt
# Verify it was imported:
go-filecoin client cat <data CID>
# Get the file size:
go-filecoin client cat <data CID> | wc -c
# Find a miner by looking through the orderbook
go-filecoin orderbook asks | jq
# Propose a storage deal, using the <miner address> from the ask.
go-filecoin client propose-storage-deal <miner address> <data CID> <duration> --price=2
# Check the status:
go-filecoin client query-storage-deal <id returned above>
# If you want to retreive the piece immediately you can bypass the retrieval market.
# Note that this is kind of cheatsy but what works at the moment.
go-filecoin client cat <data CID>

# TDOO Retrieval Miner
# If you want to fetch the piece from the miner's sealed sector, 
# wait for the deal to be Sealed per query-storage-deal status above, and
# then use the retrieval miner. Warning: this requires the sector be unsealed, 
# which takes minutes to run:
# go-filecoin retrieval-client retrieve-piece ... (exact command line TBD)
```

## Community

Here are a few places to get help and hang out with the Filecoin community:

- [Documentation Wiki](https://github.com/filecoin-project/go-filecoin/wiki) — for tutorials, troubleshooting, and FAQs
- [#filecoin-chat on Slack](https://protocollabs.slack.com/messages/CD4RLHMU0/convo/CCYS39YKZ-1538593031.000100/) — for live support and hacking with others
- [Discussion forum](https://filecoin1.trydiscourse.com/) - for talking about design decisions, use cases, implementation advice, and longer-running conversations
- [GitHub issues](https://github.com/filecoin-project/go-filecoin/issues) - for now, use only to report bugs, and view or contribute to ongoing development. PRs welcome! Please see [our contributing guidelines](CONTRIBUTING.md). 

## License

The Filecoin Project is dual-licensed under Apache 2.0 and MIT terms:

- Apache License, Version 2.0, ([LICENSE-APACHE](https://github.com/filecoin-project/go-filecoin/blob/master/LICENSE-APACHE) or http://www.apache.org/licenses/LICENSE-2.0)
- MIT license ([LICENSE-MIT](https://github.com/filecoin-project/go-filecoin/blob/master/LICENSE-MIT) or http://opensource.org/licenses/MIT)


[1]: https://golang.org/dl/
[2]: https://github.com/whyrusleeping/gx
[3]: https://github.com/RichardLitt/standard-readme
[4]: https://golang.org/doc/install
[5]: https://www.rust-lang.org/en-US/install.html

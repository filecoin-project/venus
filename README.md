# Filecoin (go-filecoin)

[![codecov](https://codecov.io/gh/filecoin-project/go-filecoin/branch/master/graph/badge.svg?token=J5QWYWkgHT)](https://codecov.io/gh/filecoin-project/go-filecoin)
[![CircleCI](https://circleci.com/gh/filecoin-project/go-filecoin.svg?style=svg&circle-token=5a9d1cb48788b41d98bdfbc8b15298816ec71fea)](https://circleci.com/gh/filecoin-project/go-filecoin)

> Filecoin implementation in Go

Filecoin is a decentralized storage network that turns cloud storage into an algorithmic market. The
market runs on a blockchain with a native protocol token (also called "filecoin" or FIL), which miners earn
by providing storage to clients.

## Table of Contents

- [Installation](#installation)
- [Development](#development)
  - [Install Go](#install-go)
  - [Clone](#clone)
  - [Install Dependencies](#install-dependencies)
  - [Managing Submodules](#managing-submodules)
  - [Running tests](#running-tests)
  - [Build Commands](#build-commands)
- [Running Filecoin](#running-filecoin)
   - [Running multiple nodes with IPTB](#running-multiple-nodes-with-iptb)
   - [Sample Commands](#sample-commands)
- [Contribute](#contribute)

## Installation

You can prebuilt download binaries for linux and macOS from CircleCI.

- Go to [the project page on CircleCI](https://circleci.com/gh/filecoin-project/go-filecoin/tree/master).
- Click on the top most successfull build, making sure that you use either `build_linux` or `build_macos` depending on the OS you want.
- Click on the `Artifacts` tab.
- Click on `Container 0 > filecoin.tar.gz` to download the release.

## Development

### Install Go and Rust

The build process for go-filecoin requires at least Go version 1.10, which you can download [here][1]. You'll also need
Rust to build the `rust-proofs` submodule, which you can download [here][5].

(If you run into trouble, see the [Go install instructions][4]).

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

Filecoin uses Git Submodules to consume `go-proofs`. To initialize the submodule, either run `deps` (as per above), or
initialize the submodule manually:

```sh
> cd ${GOPATH}/src/github.com/filecoin-project/go-filecoin
> git submodule update --init
```

Later, when the head of the `go-proofs` `master` branch changes, you may want to update `go-filecoin` to use these changes:

```sh
> git submodule update --remote
```

Note that updating the `go-proofs` submodule in this way will require a commit to `go-filecoin` (changing the submodule hash).

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
go-filecoin init --genesisfile ./fixtures/genesis.car     # TODO include instructions on setting sealing params, etc
go-filecoin daemon
# Switch terminals
# The miner is present in the genesis block car file created from the 
# json file, but the node is not yet configured to use it. Get the 
# miner address from the json file fixtures/gen.json and replace X
# in the command below with it:
go-filecoin config mining.minerAddress \"X\"
# The account that owns the miner is also not yet configured in the node
# so note that owner key name in fixtures/gen.json (eg, "a") and 
# import that key from the fixtures, assuming it was X:
go-filecoin wallet import fixtures/X.key
# The miner was not created with a pre-set peerid, so set it so that
# clients can find it.
go-filecoin miner update-peerid
```

To set up a node and connect into an existing cluster:
```
TODO
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
# Wait for the deal to be Sealed, once it is, retrieve the piece
# Note: might want to configure setting so that this happens quickly (TODO include this config option above)
go-filecoin client cat <data CID>  # Note: this command likely replaced soon by retrieval-client command.
```

## Contribute

See [the contribute file](CONTRIBUTING.md).

If editing the readme, please conform to the [standard-readme][3] specification.

## License

The Filecoin Project is dual-licensed under Apache 2.0 and MIT terms:

 * Apache License, Version 2.0, ([LICENSE-APACHE](LICENSE-APACHE) or http://www.apache.org/licenses/LICENSE-2.0)
 * MIT license ([LICENSE-MIT](LICENSE-MIT) or http://opensource.org/licenses/MIT)

[1]: https://golang.org/dl/
[2]: https://github.com/whyrusleeping/gx
[3]: https://github.com/RichardLitt/standard-readme
[4]: https://golang.org/doc/install
[5]: https://www.rust-lang.org/en-US/install.html

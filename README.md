**Note: THE FILECOIN PROJECT IS STILL EXTREMELY CONFIDENTIAL. Do not share anything outside of Protocol Labs. Do not discuss anything related to Filecoin outside of Protocol Labs, not even with your partners/spouses/other family members. If you have any questions about what can be discussed, please email [legal@protocol.ai](mailto:legal@protocol.ai).**

# Filecoin (go-filecoin)

[![CircleCI](https://circleci.com/gh/filecoin-project/go-filecoin.svg?style=svg&circle-token=5a9d1cb48788b41d98bdfbc8b15298816ec71fea)](https://circleci.com/gh/filecoin-project/go-filecoin)
[![codecov](https://codecov.io/gh/filecoin-project/go-filecoin/branch/master/graph/badge.svg?token=J5QWYWkgHT)](https://codecov.io/gh/filecoin-project/go-filecoin)	

> Filecoin implementation in Go

Filecoin is a decentralized storage network that turns cloud storage into an algorithmic market. The
market runs on a blockchain with a native protocol token (also called "filecoin" or FIL), which miners earn
by providing storage to clients.

👋**Trying out the project for the first time?** We recommend heading to the [Wiki](https://github.com/filecoin-project/go-filecoin/wiki/) for more detailed instructions and guides.

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
- [Cluster](#Clusters)
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
mkdir -p ${GOPATH}/src/github.com/filecoin-project
git clone git@github.com:filecoin-project/go-filecoin.git ${GOPATH}/src/github.com/filecoin-project/go-filecoin
```

### Install Dependencies

go-filecoin's dependencies are managed by [gx][2]; this project is not "go gettable." To install gx, gometalinter, and
other build and test dependencies, run:

```sh
cd ${GOPATH}/src/github.com/filecoin-project/go-filecoin
go run ./build/*.go deps
```

### Managing Submodules

This step is necessary if you want to edit `rust-proofs`. If you're not editing `rust-proofs` there's no need to do this manually, because the `deps` build step will do it for you.

Filecoin uses Git Submodules to consume `rust-proofs`. To initialize the submodule, either run `deps` (as per above), or
initialize the submodule manually:

```sh
cd ${GOPATH}/src/github.com/filecoin-project/go-filecoin
git submodule update --init
```

Later, when the head of the `rust-proofs` `master` branch changes, you may want to update `go-filecoin` to use these changes:

```sh
git submodule update --remote
```

Note that updating the `rust-proofs` submodule in this way will require a commit to `go-filecoin` (changing the submodule hash).

### Running Tests

The filecoin binary must be built prior to testing changes made during development. To do so, run:

```sh
go run ./build/*.go build
```

Then, run the tests:

```sh
go run ./build/*.go test
```

Note: Build and test can be combined:

```sh
go run ./build/*.go best
```

### Build Commands

```sh
# Build
go run ./build/*.go build

# Install
go run ./build/*.go install

# Test
go run ./build/*.go test

# Build & Test
go run ./build/*.go best

# Coverage
go run ./build/*.go test -cover

# Lint
go run ./build/*.go lint

# Race
go run ./build/*.go test -race

# Deps, Lint, Build, Test (with args passed to Test)
go run ./build/*.go all
```

Note: Any flag passed to `go run ./build/*.go test` (e.g. `-cover`) will be passed on to `go test`.

**If you have problems with the build, please see [8. Troubleshooting & FAQ](https://github.com/filecoin-project/go-filecoin/wiki/8.-Troubleshooting-&-FAQ) Wiki page.**

## Helpful Environment Variables

| Variable                | Description                                                                                    |
|-------------------------|------------------------------------------------------------------------------------------------|
| `FIL_API`               | This is the default host and port for daemon commands.                                         |
| `FIL_PATH`              | Use this variable to avoid setting `--repodir` flag by providing a default value.              |
| `FIL_USE_SMALL_SECTORS` | Seal all sector data, as the proofs system only ever seals the first 127 bytes at the moment.  |
| `GO_FILECOIN_LOG_LEVEL` | This sets the log level for stdout.                                                            |

## Running Filecoin

```
rm -fr ~/.filecoin      # <== optional, in case you have a pre-existing install
go-filecoin init        # Creates config in ~/.filecoin; to see options: `go-filecoin init --help`
go-filecoin daemon      # Starts the daemon, you may now issue it commands in another terminal
```

To set up a single node capable of mining:
```
rm -fr ~/.filecoin   # only if you have a pre-existing install
go-filecoin init --genesisfile ./fixtures/genesis.car
go-filecoin daemon
```

Note the output of the daemon, it should say "My peer ID is `<peerID>`", where `<peerID>`
is a long [CID](https://github.com/filecoin-project/specs/blob/master/definitions.md#cid) string starting with "Qm".  `<peerID>` is used in a later command.

Switch terminals.

The miner is present in the genesis block car file created from the 
json file, but the node is not yet configured to use it. Get the 
miner address from the json file fixtures/gen.json and replace `<minerAddr>`
in the command below with it:

`go-filecoin config mining.minerAddress '"<minerAddr>"'`

The account that owns the miner is also not yet configured in the node
so note that owner key name in fixtures/gen.json. We'll call it `<minerOwnerKey>`,
and import that key from the fixtures:

`go-filecoin wallet import fixtures/<minerOwnerKey>.key`

Note the output of this command, call it `<minerOwnerAddress>`. This output is the address of 
the account that owns the miner.
The miner was not created with a pre-set Peer ID, so set it so that
clients can find it.

`go-filecoin miner update-peerid --from=<minerOwnerAddress> <minerAddr> <peerID>`

Now you can run a lookup:
`go-filecoin address lookup <minerAddr>`

The output should now be `<peerID>`

To configure a node's auto-sealing scheduler:
The auto-sealer is used to automatically seal the data that a miner received
from a client into a sector. Without this feature, miners would wait until
they had accumulated $SECTORSIZE worth of client data before initiating the
sealing process so that they didn't waste precious hard drive space. With
auto-sealing enabled, the miner will use $SECTORSIZE bytes of storage each
period unless there are no data to store that period.

To control how frequently the auto-sealer runs, provide a positive integer
value for the --auto-seal-interval-seconds option. To disable this feature,
provide a 0.

If the option is omitted, a default of 120 seconds will be used.

```
rm -fr ~/.filecoin
go-filecoin init --auto-seal-interval-seconds=0 --genesisfile ./fixtures/genesis.car
go-filecoin daemon
```

## Running multiple nodes with IPTB

IPTB provides an automation layer that makes it easy to run multiple filecoin nodes. 
For example, it enables you to easily start up 10 mining nodes locally on your machine.
Please refer to the [README.md](https://github.com/filecoin-project/go-filecoin/blob/master/tools/iptb-plugins/README.md).

## Sample commands

### List and ping a peer 
```
go-filecoin swarm peers
go-filecoin ping <peerID>
```
#
###  View latest mined block 
```
go-filecoin chain head
go-filecoin show block <blockID> | jq
```
#
#### Create a miner
_NOTE: If you have followed the instructions in [Running Filecoin](#Running_Filecoin), a miner will already exist from the genesis file that you used, so these instructions will not work._
```
# Create a miner
# Requires the node be a part of a cluster that already has miners 
# and no miner configured for this node yet.
go-filecoin miner create 10 10

# Waits for the message to be included on chain, updates the minerAddress
# in the node's config, and sets the peerid appropriately.
# Get your miner address
go-filecoin config mining.minerAddress

# And the owner:
go-filecoin miner owner <minerAddress>
```

### As a miner, force a block to be mined immediately 
`go-filecoin mining once`

If successful, go-filecoin daemon output should show an indication of mining.

#
### As a miner, make an ask 
```
# As a miner, make an ask 
# First make sure mining is running
go-filecoin mining start

# Get your miner address
go-filecoin config mining.minerAddress

# Get your miner owner address 
go-filecoin miner owner <minerAddress>
go-filecoin miner add-ask <minerAddress> <size> <price> --from=<ownerAddress>

# Wait for the block to be mined (~30s) and view the ask:
go-filecoin client list-asks | jq
```
#
### As a client, make a deal 

```
# As a client, make a deal 
echo "Hi my name is $USER"> hello.txt
go-filecoin client import ./hello.txt

# Verify it was imported:
go-filecoin client cat <data CID>

# Get the file size:
go-filecoin client cat <data CID> | wc -c

# Find a miner by running client list-asks
go-filecoin client list-asks | jq

# Propose a storage deal, using the <miner address> from the ask.
# First make sure that mining is running
go-filecoin mining start

# propose the deal.
go-filecoin client propose-storage-deal <miner address> <data CID> <price> <durationBlocks> 

# TODO we want to be able to check the status, like this but the command above doesn't 
# return an id
go-filecoin client query-storage-deal <id returned above>

# If you want to retreive the piece immediately you can bypass the retrieval market.
# Note that this is kind of cheatsy but what works at the moment.
go-filecoin client cat <data CID>
```
#
### Retrieval Miner
If you want to fetch the piece from the miner's sealed sector, 
wait for the deal to be Sealed per query-storage-deal status above, and
then use the retrieval miner. Warning: this requires the sector be unsealed, 
which takes a minute to run (it doesn't yet cache). 
go-filecoin retrieval-client retrieve-piece <miner peer id> <data CID>
Ex on the miner's node, get the peer id from: go-filecoin id 
Then: 
```
go-filecoin retrieval-client \
   retrieve-piece QmXtaLS9N3URQ2uCkqpLP6KZv7rVbT5KyjU5MQAgQM6yCq \
   QmNqefRonNc2Rn5VwEB5wqJLE9arURmBUSay3kbjJoLJG9
```
#
## Community

Here are a few places to get help and hang out with the Filecoin community:

- [Documentation Wiki](https://github.com/filecoin-project/go-filecoin/wiki) — for tutorials, troubleshooting, and FAQs
- [Filecoin Dev on Matrix/Riot](https://riot.im/app/#/room/#fil-dev:matrix.org) — for live support and hacking with others
- [Discussion forum](https://filecoin1.trydiscourse.com/) - for talking about design decisions, use cases, implementation advice, and longer-running conversations
- [GitHub issues](https://github.com/filecoin-project/go-filecoin/issues) - for now, use only to report bugs, and view or contribute to ongoing development. PRs welcome! Please see [our contributing guidelines](CONTRIBUTING.md). 

#

## Clusters

### Test Cluster

Deployed via CI by tagging a commit with `redeploy_test_cluster`

- Faucet: http://test.kittyhawk.wtf:9797/
- Dashboard: http://test.kittyhawk.wtf:8010/
- Genesis File: http://test.kittyhawk.wtf:8020/genesis.car
- Block explorer: http://test.kittyhawk.wtf:8000/
- Prometheus Endpoint: http://test.kittyhawk.wtf:9082/metrics
- Connected Nodes PeerID's: http://test.kittyhawk.wtf:9082/nodes

### Nightly Cluster

Deployed from master by CI every day at 0600 UTC

- Faucet: http://nightly.kittyhawk.wtf:9797/
- Dashboard: http://nightly.kittyhawk.wtf:8010/
- Genesis File: http://nightly.kittyhawk.wtf:8020/genesis.car
- Block explorer: http://nightly.kittyhawk.wtf:8000/
- Prometheus Endpoint: http://nightly.kittyhawk.wtf:9082/metrics
- Connected Nodes PeerID's: http://nightly.kittyhawk.wtf:9082/nodes

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

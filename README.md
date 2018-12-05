**Note: THE FILECOIN PROJECT IS STILL EXTREMELY CONFIDENTIAL. Do not share or discuss the project outside of designated preview channels (chat channels, Discourse forum, GitHub, emails to Filecoin team), not even with partners/spouses/family members. If you have any questions, please email [legal@protocol.ai](mailto:legal@protocol.ai).**

# Filecoin (go-filecoin)

[![CircleCI](https://circleci.com/gh/filecoin-project/go-filecoin.svg?style=svg&circle-token=5a9d1cb48788b41d98bdfbc8b15298816ec71fea)](https://circleci.com/gh/filecoin-project/go-filecoin)
[![codecov](https://codecov.io/gh/filecoin-project/go-filecoin/branch/master/graph/badge.svg?token=J5QWYWkgHT)](https://codecov.io/gh/filecoin-project/go-filecoin)	

> Filecoin implementation in Go, turning the world’s unused storage into an algorithmic market.

## Table of Contents

- [What is Filecoin?](#what-is-filecoin)
- [Install](#install)
  - [System Requirements](#system-requirements)
  - [Install from Binary](#install-from-binary)
  - [Install from Source](#install-from-source)
    - [Install Go and Rust](#install-go-and-rust)
    - [Clone](#clone)
    - [Install Dependencies](#install-dependencies)
    - [Manage Submodules Manually](#manage-submodules-manually)
    - [Build, Run Tests, and Install](#build-run-tests-and-install)
    - [Build Commands](#build-commands)
- [Usage](#usage)
   - [Start Running Filecoin](#start-running-filecoin)
   - [Run Multiple Nodes with IPTB](#run-multiple-nodes-with-iptb)
   - [Sample Commands](#sample-commands)
   - [Helpful Environment Variables](#helpful-environment-variables)
- [Clusters](#clusters)
- [Contributing](#contributing)
- [Community](#community)
- [License](#license)

## What is Filecoin?
Filecoin is a decentralized storage network that turns the world’s unused storage into an algorithmic market, creating a permanent, decentralized future for the web. **Miners** earn the native protocol token (also called “filecoin”) by providing data storage and/or retrieval. **Clients** pay miners to store or distribute data and to retrieve it. Check out [How Filecoin Works](https://github.com/filecoin-project/go-filecoin/wiki/1.-How-Filecoin-Works) for more.

## Install

👋**Trying out the project for the first time?** We highly recommend the [detailed setup instructions](https://github.com/filecoin-project/go-filecoin/wiki/2.-Getting-Started) in the [Wiki](https://github.com/filecoin-project/go-filecoin/wiki/).

### System Requirements
Filecoin can run on most Linux and MacOS systems. Windows is not yet supported.

### Install from Binary

  - We host prebuilt binaries over at [CircleCI](https://circleci.com/gh/filecoin-project/go-filecoin/tree/master). Log in with Github.
  - Follow the remaining steps in [Getting Started](https://github.com/filecoin-project/go-filecoin/wiki/2.-Getting-Started)

### Install from Source

#### Install Go and Rust

  - The build process for go-filecoin requires at least [Go](https://golang.org/doc/install) version 1.11.2. If you're setting up Go for the first time, we recommend [this tutorial](https://www.ardanlabs.com/blog/2016/05/installing-go-and-your-workspace.html) which includes environment setup.  
  - You'll also need [Rust](https://www.rust-lang.org/) (v1.29.0 or later) to build the `rust-proofs` submodule.

#### Clone

```sh
mkdir -p ${GOPATH}/src/github.com/filecoin-project
git clone git@github.com:filecoin-project/go-filecoin.git ${GOPATH}/src/github.com/filecoin-project/go-filecoin
```

#### Install Dependencies

go-filecoin's dependencies are managed by [gx][2]; this project is not "go gettable." To install gx, gometalinter, and
other build and test dependencies, run:

```sh
cd ${GOPATH}/src/github.com/filecoin-project/go-filecoin
go run ./build/*.go deps
```

#### Manage Submodules Manually

_If you're not editing `rust-proofs` you can skip this step, because `deps` build (above) will do it for you._

Filecoin uses Git Submodules to consume `rust-proofs`. To initialize:

```sh
cd ${GOPATH}/src/github.com/filecoin-project/go-filecoin
git submodule update --init
```

Later, when the head of the `rust-proofs` `master` branch changes, you may want to update `go-filecoin` to use these changes:

```sh
git submodule update --remote
```

Note that updating the `rust-proofs` submodule in this way will require a commit to `go-filecoin` (changing the submodule hash).

### Build, Run Tests, and Install

```sh
# First, build the binary:
go run ./build/*.go build

# Then, run the tests:
go run ./build/*.go test

# Note: build and test can be combined:
go run ./build/*.go best

# Install go-filecoin
go run ./build/*.go install
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

**If you have problems with the build, please see the [Troubleshooting & FAQ](https://github.com/filecoin-project/go-filecoin/wiki/8.-Troubleshooting-&-FAQ) Wiki page.**


## Usage

### Start Running Filecoin
To start running Filecoin, you must initialize and start a daemon:

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

Note the output of the daemon. It should say "My peer ID is `<peerID>`", where `<peerID>`
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
The miner was not created with a pre-set `peerID`, so set it so that
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

### Run Multiple Nodes with IPTB

The [`localfilecoin` IPTB plugin](https://github.com/filecoin-project/go-filecoin/tree/master/tools/iptb-plugins) provides an automation layer that makes it easy to run multiple filecoin nodes. For example, it enables you to easily start up 10 mining nodes locally on your machine.

### Sample Commands

To see a full list of commands, run `go-filecoin --help`.

```sh
USAGE
  go-filecoin - A decentralized storage network

OPTIONS

  --cmdapiaddr           string - set the api port to use
  --repodir              string - set the directory of the repo, defaults to ~/.filecoin.
  --enc,      --encoding string - The encoding type the output should be encoded with (json, xml, or text). Default: text.
  --help                 bool   - Show the full command help text.
  -h                     bool   - Show a short version of the command help text.

SUBCOMMANDS

  START RUNNING FILECOIN
    init                   - Initialize a filecoin repo
    config <key> [<value>] - Get and set filecoin config values
    daemon                 - Start a long-running daemon process
    wallet                 - Manage your filecoin wallets
    address                - Interact with wallet addresses
    
  STORE AND RETRIEVE DATA
    client                 - Make deals, store data, retrieve data
    retrieval-client       - Manage retrieval client operations

  MINE
    miner                  - Manage a single miner actor
    mining                 - Manage all mining operations for a node

  VIEW DATA STRUCTURES
    chain                  - Inspect the filecoin blockchain 
    dag                    - Interact with IPLD DAG objects
    show                   - Get human-readable representations of filecoin objects

  NETWORK COMMANDS
    bootstrap              - Interact with bootstrap addresses
    id                     - Show info about the network peers 
    ping <peer ID>...      - Send echo request packets to p2p network members
    swarm                  - Interact with the swarm
 
  ACTOR COMMANDS
    actor      		         - Interact with actors. Actors are built-in smart contracts. 
    paych      		         - Payment channel operations
    
  MESSAGE COMMANDS
    message                - Manage messages
    mpool                  - View the mempool of outstanding messages

  TOOL COMMANDS
    log                    - Interact with the daemon event log output
    version                - Show go-filecoin version information


  Use 'go-filecoin <subcmd> --help' for more information about each command.
```

More details are in the [Filecoin Commands](https://github.com/filecoin-project/go-filecoin/wiki/7.-Filecoin-Commands) wiki page.

### Helpful Environment Variables

| Variable                | Description                                                                                    |
|-------------------------|------------------------------------------------------------------------------------------------|
| `FIL_API`               | This is the default host and port for daemon commands.                                         |
| `FIL_PATH`              | Use this variable to avoid setting `--repodir` flag by providing a default value.              |
| `FIL_USE_SMALL_SECTORS` | Seal all sector data, as the proofs system only ever seals the first 127 bytes at the moment.  |
| `GO_FILECOIN_LOG_LEVEL` | This sets the log level for stdout.                                                            |

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

### Nightly Cluster

Deployed from master by CI every day at 0600 UTC. **If you want to use a cluster
you should probably be using this one.**

- Faucet: http://nightly.kittyhawk.wtf:9797/
- Dashboard: http://nightly.kittyhawk.wtf:8010/
- Genesis File: http://nightly.kittyhawk.wtf:8020/genesis.car
- Block explorer: http://nightly.kittyhawk.wtf:8000/
- Prometheus Endpoint: http://nightly.kittyhawk.wtf:9082/metrics
- Connected Nodes PeerID's: http://nightly.kittyhawk.wtf:9082/nodes

To connect a Filecoin Node to the Nightly Cluster:

```bash
# Initalize the daemon to connect to the cluster and download the cluster genesis block
go-filecoin init --cluster-nightly --genesisfile=http://nightly.kittyhawk.wtf:8020/genesis.car

# Start the daemon (backgrounded for simplicity here)
go-filecoin daemon

# Give filecoin a nickname if you'd like (appears along side the PeerID in dashboard)
go-filecoin config heartbeat.nickname "Tatertot"

# Configure the filecoin daemon to connect to the clusters dashboard
go-filecoin config heartbeat.beatTarget "/dns4/nightly.kittyhawk.wtf/tcp/9081/ipfs/QmVR3UFv588pSu8AxSw9C6DrMHiUFkWwdty8ajgPvtWaGU"

```

### Test Cluster (for Infra)

Deployed via CI by tagging a commit with `redeploy_test_cluster`. **This cluster
is for people working on infra. You should probably avoid it unless that describes you.**

- Faucet: http://test.kittyhawk.wtf:9797/
- Dashboard: http://test.kittyhawk.wtf:8010/
- Genesis File: http://test.kittyhawk.wtf:8020/genesis.car
- Block explorer: http://test.kittyhawk.wtf:8000/
- Prometheus Endpoint: http://test.kittyhawk.wtf:9082/metrics
- Connected Nodes PeerID's: http://test.kittyhawk.wtf:9082/nodes

To connect a Filecoin Node to the Test Cluster:

```bash
# Initalize the daemon to connect to the cluster and download the cluster genesis block
go-filecoin init --cluster-test --genesisfile=http://test.kittyhawk.wtf:8020/genesis.car

# Start the daemon (backgrounded for simplicity here)
go-filecoin daemon

# Give filecoin a nickname if you'd like (appears along side the PeerID in dashboard)
go-filecoin config heartbeat.nickname "Porkchop"

# Configure the filecoin daemon to connect to the clusters dashboard
go-filecoin config heartbeat.beatTarget "/dns4/test.kittyhawk.wtf/tcp/9081/ipfs/QmVR3UFv588pSu8AxSw9C6DrMHiUFkWwdty8ajgPvtWaGU"

```

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

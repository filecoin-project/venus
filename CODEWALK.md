# Go-filecoin code overview

This document provides a high level tour of the go-filecoin implementation of the Filecoin protocols 
in Go.

This document assumes a reasonable level of knowledge about the Filecoin system and protocols,
which are not re-explained here. It is complemented by specs (link forthcoming) that describe 
the key concepts implemented here.
``

## Background
The go-filecoin implementations is the result of combined research and development effort.
The protocol spec and architecture evolved from a prototype, and is the result of 
iterating towards our goals. Go-filecoin is a work in progress. We are still working on clarifying
the architecture and propagating good patterns throughout the code. Please bear with us, and we’d
love your help.

Filecoin borrows a lot from the [IPFS](https://ipfs.io/) project, including some patterns, 
tooling, and packages. Some benefits of this include:
- the projects encode data in the same way ([IPLD](https://ipld.io/), 
[CIDs](https://github.com/multiformats/cid)), easing interoperability;
- the go-filecoin project can build on solid libraries like the IPFS commands.

Other patterns, we've evolving for our needs:
- Go-IPFS relies heavily on shell-based integration testing; we aim to rely heavily on unit
testing and Go-based integration tests.
- The Go-IPFS package structure involves a deep hierarchy of dependent implementations; we're moving
towards a more Go-idiomatic approach with narrow interfaces defined in consuming packages
(see [Patterns]) 
- The term “block” is heavily overloaded: a blockchain block (types/block.go), but also
content-id-addressed blocks in the block service. Blockchain blocks are stored in block service
blocks, but are not the same thing.

## Architecture overview

(Diagram goes here)

## A tour of the code

### History–the Node object
The Node ([node/](https://github.com/filecoin-project/go-filecoin/tree/master/node))
object is the “server”. It contains much of the core protocol 
implementation and plumbing. As an accident of history it has become something of a god-object, 
which we are working to resolve. The Node object is difficult to unit test due to its many 
dependencies and complicated set-up. We are
[moving away from this pattern](https://github.com/filecoin-project/go-filecoin/issues/1469#issuecomment-451619821),
and expect the Node 
object to be reduced to a glorified constructor over time.

The [api](https://github.com/filecoin-project/go-filecoin/tree/master/api) package contains the 
API of all the core building blocks upon which the protocols are
implemented. The implementation of this API is the Node. We are migrating away from this api
package to the plumbing package, see below.

The [protocol](https://github.com/filecoin-project/go-filecoin/tree/master/protocol) package 
contains much of the application-level protocol code. The protocols are
implemented in terms of the Node API (old) as well as the new plumbing & porcelain APIs (see below).
Currently the hello, retrieval and storage protocols are implemented here. Block mining should move
here (from the [mining](https://github.com/filecoin-project/go-filecoin/tree/master/mining) 
top-level package and Node internals). Chain syncing may move here too.

### Core services
At the bottom of the architecture diagram are core services. These are focussed implementations of
some functionality that don’t achieve much on their own, but are the means to the end. Core services
are the bottom level building blocks out of which application functionality can be built.
They are the source of truth in all data.

Core services are mostly found in top-level packages. Most are reasonably well factored and
testable in isolation.

Services include (not exhaustive):
- Message pool: hold messages that haven’t been mined into a block yet
- Chain store: stores & indexes blockchain blocks 
- Chain syncer: syncs chain blocks from the rest of the network 
- Processor: Defines how transaction messages drive state transitions
- Block service: content-addressed key value store that stores IPLD data, including blockchain blocks as well as the state tree (it’s poorly named)
- Wallet: manages keys

### Plumbing & porcelain
The [plumbing](https://github.com/filecoin-project/go-filecoin/tree/master/plumbing) & 
[porcelain](https://github.com/filecoin-project/go-filecoin/tree/master/porcelain) packages are 
the new API; over time, this patterns should completely 
replace the existing top-level api package and its implementation in Node. The plumbing & 
porcelain design pattern is explained in more detail below.

Plumbing is the set of public apis required to implement all user-, tool-, and protocol-level
features. Plumbing implementations depend on the core services they need, but not on the Node.
Plumbing is intended to be fairly thin, routing requests and data between core components. Plumbing
implementations are often tested with real implementations of the core services they use, but can
also be tested with fakes and mocks.

Porcelain implementations are convenience compositions of plumbing. They depend only on the plumbing
API, and can coordinate a sequence of actions. Porcelain is ephemeral; the lifecycle is the duration
of a single porcelain call: something calls into it, it does its thing, and then returns. Porcelain
implementations are ideally tested with fakes of the plumbing they use, but can also use full
implementations. 

### Commands
The `go-filecoin` binary can run in two different modes, either as a long-running daemon exposing a
JSON/HTTP RPC API, or as a command-line interface which interprets and routes requests as RPCs to a
daemon. In typical usage, you start the daemon with `go-filecoin daemon` then use the same
binary to issue commands like `go-filecoin wallet addrs`, which are transmitted to the 
daemon over the HTTP API.

The commands package uses the [go-ipfs command library](https://github.com/ipfs/go-ipfs-cmds) 
and defines commands as both CLI and JSON entry points.

[Commands](https://github.com/filecoin-project/go-filecoin/tree/master/commands) implement 
user- and tool-facing functionality. Command implementations should be very,
very small. With no logic of their own, they should call just into a single plumbing or porcelain
method (never into core APIs directly). The go-ipfs command library introduces some boilerplate
which we can reduce with some effort in the future. Right now, some of the command implementations
call into the node; this should change.

Tests for commands are generally end-to-end “demon tests” that exercise CLI. They start some nodes
and interact with them through shell commands. 

### Protocols
[Protocols](https://github.com/filecoin-project/go-filecoin/tree/master/commands) embody 
“application-level” functionality. They are persistent; they keep running without
active user/tool activity. Protocols interact with the network. Protocols depend on plumbing
and porcelain for their implementation, as well some "private" core APIs
(at present, many still depend on the Node object).

Protocols drive changes in, but do not own, core state. For example, the chain sync protocol drives
updates to the chain store (a core service), but the sync protocol does not own the chain data.
However, protocols may maintain their own non-core, protocol-specific datastores 
(e.g. unconfirmed deals). 

Application-level protocol implementations include:
- Storage protocol: the mechanism by which clients make deals with miners, transfer data for
storage, and then miners prove storage.
- Block mining protocol: the protocol for block mining and consensus. Miners who are storing data
participate in creating new blocks. Miners win elections in proportion to storage committed. This
block mining is spread through a few places in the code. Much in mining package, but also a bunch in the node implementation.
- Chain protocol: protocol for exchange of mined blocks

More detail on the individual protocols is coming soon.

### Network layer
Filecoin relies on [libp2p](https://libp2p.io/) for all its networking, such as peer discovery, 
NAT discovery and circuit relay. Filecoin uses two transport protocols from libp2p:

- [GossipSub](https://github.com/libp2p/specs/tree/master/pubsub/gossipsub) for pubsub gossip among
peers propagating blockchain blocks and unmined messages
- [Bitswap](https://github.com/ipfs/specs/tree/master/bitswap) for exchanging binary data


### Entry point
There’s no centrally dispatched event loop. The node starts up all the components, connects them
as needed, and waits. Protocols (goroutines) communicate through custom channels. This architecture
needs more thought, but we are considering moving more inter-module communication to use iterators
(c.f. those in Java). An event bus might also be a good pattern for some cases, though.

### Testing
(More content coming here)

A few functional tests have some bash scripts orchestrating complex test setups.

## Filesystem storage
The *repo*, aka `fsrepo`, is a directory stored on disk containing all necessary information to 
run a `go-filecoin daemon`, typically at `$HOME/.filecoin`. The repo does not include client data 
stored by storage miners, which is held instead in the sector base. The repo does 
include a JSON config file with preferences on how the daemon should operate, several key value 
datastores holding data important to the internal services, and the keystore which holds private 
key data for encryption.

### JSON Config
The JSON config file is stored at `$HOME/.filecoin/config.json`, and can be easily edited using the
`go-filecoin config` command. Users can also edit the file directly at their own peril.

### Datastores
The key value datastores in the repo include persisted data from a variety of systems within 
Filecoin. Most of them hold CBOR encoded data keyed on CID, however this varies. 
The key value stores include the badger, chain, deals, and wallet directories under $HOME/.filecoin.

The purpose of these directories is:
- Badger is a general purpose datastore currently only holding the genesis key, but in the future,
almost all our datastores should be merged into this one.
- Chain is where the local copy of the blockchain is stored.
- Deals is where the miner and client store persisted information on open deals for data storage, 
essentially who is storing what data, for what fee and which sectors have been sealed.
- Wallet is where the user’s Filecoin wallet information is stored.

### Keystore
The keystore contains the binary encoded peer key for interacting securely over the network. 
This data lives in a file at `$HOME/.filecoin/keystore/self`.

## Dependencies
Dependencies in Go-Filecoin are managed by [gx](https://github.com/whyrusleeping/gx), a 
content-addressed dependency manager. You’ll notice that the hash of a dependency’s content appears
in the import path. Almost all runtime dependencies are managed by gx (mostly being other
Protocol Labs-sponsored projects). 

The `gx-go` manages a package.json file. In order to be imported by gx, a package needs to be 
“gxed”. See the [gx-go repo](https://github.com/whyrusleeping/gx-go) for details about preparing a 
package for gxing and importing it into the project. If you want to depend on a package whos author 
has not gxed it, we can fork it and gx our fork.

Gx came about before Go modules, which aim to solve many of the same problems. The IPFS project 
and go-filecoin [may migrate to Go modules](https://github.com/ipfs/go-ipfs/issues/5850) 
in the future.

## Patterns
The project makes heavy use of or is moving towards a few key design patterns, explained here.

### Plumbing and porcelain
The plumbing and porcelain pattern is 
[borrowed from Git](https://git-scm.com/book/en/v2/Git-Internals-Plumbing-and-Porcelain).
Plumbing and porcelain form the API to the internal [core services](#core-services), and will 
replace the `api` package.

*Plumbing* is the small set of public building blocks of queries and operations that protocols, 
clients and humans want to use with a Filecoin node. These are things like `MessageSend`, `GetHead`,
`GetBlockTime`, etc. By fundamental, we mean that it doesn't make sense to expose anything lower 
level. The bar for adding new plumbing is high. It is very important, for testing and sanity, that
plumbing methods be implemented in terms of their narrowest actual dependencies on core services,
and that they not depend on Node or another god object.

The plumbing API is defined by its implementation in 
[plumbing/api.go](https://github.com/filecoin-project/go-filecoin/blob/master/plumbing/api.go).
Consumers of plumbing (re-)define the subset of plumbing on which they depend, which is an idiomatic
Go pattern (see below). Implementations of plumbing live in their own concisely named packages under 
[plumbing](https://github.com/filecoin-project/go-filecoin/tree/master/plumbing).

*Porcelain* are calls on top of the plumbing API. A porcelain call is a useful composition of 
plumbing calls and is implemented in terms of calls to plumbing. An example of a porcelain call 
is `CreateMiner == MessageSend + MessageWait + ConfigUpdate`. The bar is low for creation of 
porcelain calls. Porcelain calls should define the subset of the plumbing interface on which they 
depend for ease of testing.

Porcelain lives in a single 
[porcelain](https://github.com/filecoin-project/go-filecoin/blob/master/porcelain/) package. 
Porcelain calls are free functions that take the plumbing interface as an argument. The call 
defines the subset of the plumbing interface that it needs, which can be easily faked in testing.

We are in the [process of refactoring](https://github.com/filecoin-project/go-filecoin/issues/1469)
all protocols to depend only on porcelain, plumbing and other core APIs, instead of on the Node.

### Consumer-defined interfaces
Go interfaces generally belong in the package that *uses* values of the interface type, not the 
package that implements those values. This embraces 
[Postel's law](https://en.wikipedia.org/wiki/Robustness_principle), reducing direct 
dependencies between packages and making themeasier to test. 
It isolates small changes to small parts of the code.

Note that this is quite different to the more common 
pattern in object-oriented languages, where interfaces are defined near their implementations.
Our implementation of [plumbing and porcelain](#plumbing-and-porcelain) embraces this pattern,
and we are adopting it more broadly.

This idiom is unfortunately hidden away in a 
[wiki page about code review](https://github.com/golang/go/wiki/CodeReviewComments#interfaces).
See also Dave Cheney on [SOLID Go Design](https://dave.cheney.net/2016/08/20/solid-go-design)



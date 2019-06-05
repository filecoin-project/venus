# Filecoin networking

Filecoin relies on [libp2p](https://libp2p.io/) for its networking needs.

libp2p is a modular networking stack for the peer-to-peer era. It offers building blocks to tackle requirements such as
peer discovery, transport switching, multiplexing, content routing, NAT traversal, pubsub, circuit relay, etc., most of which Filecoin uses.
Developers can compose these blocks easily to build the networking layer behind their P2P system.

Here we'll explain some of the mechanics that drive the Filecoin network layer.

**Table of contents**

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->


- [Peer discovery](#peer-discovery)
- [NAT topology determination](#nat-topology-determination)
- [Autorelay services for solving connectivity issues](#autorelay-services-for-solving-connectivity-issues)
- [Pubsub via Gossipsub](#pubsub-via-gossipsub)
- [Contribute to libp2p](#contribute-to-libp2p)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Peer discovery

We use the [`go-libp2p-kad-dht`](https://github.com/libp2p/go-libp2p-kad-dht) service to:

1. Discover Filecoin peers at start up.
2. Lookup the network addresses of a specific peer ID, for the purposes of Filecoin-related storage and retrieval.
3. Discover providers of the auto-relay service in the network (see the Autorelay service section).

`go-libp2p-kad-dht` is a Kademlia-inspired distributed hash table (DHT) implementation that provides a mapping between peer IDs and their [multiaddress(es)](https://github.com/multiformats/multiaddr). The usage of a DHT promotes decentralisation and resilience, such that the network will continue to function even if some nodes go down. Kademlia is described in a [paper](http://www.scs.stanford.edu/~dm/home/papers/kpos.pdf), and exemplified in the BitTorrent implementation described primarily in the approachable [BEP 5](http://www.bittorrent.org/beps/bep_0005.html).

`go-libp2p-kad-dht` maintains a data structure of connected peers of varying distances from the local peer (per the distance metric described by Kademlia), known as the routing table. To bootstrap our routing table, and to keep it healthy over time, we currently traverse the network periodically (and once on startup) by performing random walks.
We kickstart this process by seeding our routing table with a set of trusted bootstrap nodes operated by the Filecoin project.
This approach will likely evolve in the future.

On each round, we generate a random peer ID and query our known DHT peers by sending them `FindPeer(id)` messages.
Each node returns a list of peers they believe to be closer to the search target (based on the Kademlia distance metric), along with their addresses.

As replies come in, we connect to new incoming nodes we've discovered, and we repeat this process iteratively, until either:

a. we find the peer; highly unlikely scenario, as the chance is 1/2^256 with our peer ID length; or
b. we yield because a timer runs out; the most probable scenario. This is a healthy timeout to timebox discovery iterations.

Throughout this process, the libp2p stack generates `CONNECTED` events for every new connection we establish to a peer,
for which `go-filecoin` registers a callback that triggers the Filecoin `HELLO` protocol negotiation.
If the other party responds positively, chain sync with that peer begins.

A few relevant aspects to note:

1. Thanks to libp2p _multiplexing_ , we only require a single physical connection to every peer (currently TCP),
   over which we channel multiple protocols (Kad DHT, Filecoin Hello, and others) in isolation from one another, by leveraging libp2p streams.

2. As we traverse the network, we make ourselves known to other parties, who will include us in their routing table as long as we remain connected,
   and there's a vacancy for us in their routing table.
   On shutdown, we will close all connections, and all peers will drop us from their routing tables, hence we will cease being locatable.

3. We exchange _multiaddrs_ through the `Identify` protocol, after connection establishment.
   If our addresses change (such as when connect to a relay), we update our connected peers through the `IdentifyPush` protocol.

The combination of points 2 & 3 is how our addresses "make it" into the DHT.
Whenever a peer searches for us, they will conduct an iterative search until they hit one of our neighbors, who will supply our up-to-date addresses to the requestor, thus ending the search.

## NAT topology determination

In a P2P world, one of the main challenges is achieving connectivity when one, or both, of the parties is/are behind a Network Address Translation (NAT) devices, such as routers.
NATs are commonplace in different network types: residential, companies, public hotspots, universities, etc.
Their purpose is to enable devices in a private network to share a common public IP address in the global, scarce IPv4 space.

Outbound connections behind a NAT tend to be frictionless, but inbound connections are problematic.
For a primer on this topic, we recommend reading [Peer-to-Peer Communication Across Network Address Translators](https://pdos.csail.mit.edu/papers/p2pnat.pdf).

To enable a pair of Filecoin peers behind NATs to communicate with one another, we use libp2p relay services:
nodes that pipe encrypted traffic between any two Filecoin peers, akin to [TURN-like servers](https://en.wikipedia.org/wiki/Traversal_Using_Relays_around_NAT).

Relay capability is activated by sensing our NAT status, once at start (may take a few minutes) and continuously at runtime.
Two protocols are involved here:

* **Identify** protocol: for every peer we connect to, we use the *Identify* protocol to learn the address that peer observes from us (source ip_address:port).
  We compile all our observed addresses as we learn them.
* **AutoNAT** protocol: for every peer we connect to that supports the AutoNAT service, we request a _dialback_ to our observed addresses.
  We wait for a grace period to receive the result of those dials.
    * If the peer was able to establish an incoming connection through any of our observed addresses, we consider ourselves to be publicly reachable,
      therefore we bypass setting up relay routing.
    * If the peer was unable to establish an incoming connection at all, we consider ourselves to be behind a private NAT,
      and as a result we initialise Autorelay to make ourselves reachable via relayed routing.

## Autorelay services for solving connectivity issues

The Autorelay service is responsible for:

1. discovering relay nodes around the world,
2. establishing long-lived connections to them, and
3. advertising relay-enabled addresses for ourselves to our peers, thus making ourselves routable through delegated routing.

The Filecoin project operates a fleet of relay nodes, all of which are enlisted in the Autorelay namespace in the DHT via provider records so that Filecoin clients can discover them.

When AutoNAT detects we're behind a NAT that blocks inbound connections, Autorelay jumps into action, and the following happens:

1. We locate candidate relays by running a DHT provider search for the `/libp2p/relay` namespace.
2. We select three results at random, and establish a long-lived connection to them (`/libp2p/circuit/relay/0.1.0` protocol). Support for using latency as a selection heuristic will be added soon.
3. We enhance our local address list with our newly acquired relay-enabled multiaddrs, with format: `/ip4/1.2.3.4/tcp/4001/p2p/QmRelay/p2p-circuit`, where:
   `1.2.3.4` is the relay's public IP address, `4001` is the libp2p port, and `QmRelay` is the peer ID of the relay.
   Elements in the multiaddr can change based on the actual transports at use.
4. We announce our new relay-enabled addresses to the peers we're already connected to via the `IdentifyPush` protocol.

The last step is crucial, as it enables peers to learn our updated addresses, and in turn return them when another peer looks us up.

## Pubsub via Gossipsub

Filecoin relies on [GossipSub](https://github.com/libp2p/specs/tree/master/pubsub/gossipsub) for pubsub gossip among peers propagating blockchain blocks and unmined messages.
Please refer to the linked spec for more infomation.

## Contribute to libp2p

Start exploring the libp2p universe on [our website](https://libp2p.io/) and [entrypoint Github repo](https://github.com/libp2p/libp2p).
There are multiple language implementations you can get involved in.
Also join the [#libp2p channel on Freenode](http://webchat.freenode.net/?channels=%23libp2p) to interact with the community.

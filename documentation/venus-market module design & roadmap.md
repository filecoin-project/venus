## venus-market module design & roadmap

by Venus team

Sep 2021

## Background

As the rebranding of filecoin terminology spearheaded by [FIP0018](https://github.com/filecoin-project/FIPs/blob/master/_fips/fip-0018.md) settled, consensus has been reached across communities (developers, providers, ecosystem partners and etc) to push for taking on more storage deals to improve the public perception on the fact that most of the network storages are still commited capacities (CCs). Given the above sentiment, design and implemenation of venus-market module has been put into the spot light. A clear long-term roadmap is due for Venus community to dissus and iterate on, also as a means for better communications with filecoin eocsystem in general.

While Lotus is leading the way of implementing a full-fledged market module according to the [spec](https://spec.filecoin.io/#section-systems.filecoin_markets), Venus has been making much efforts to catch up and closing the gap in regard to [markets](https://github.com/filecoin-project/venus/discussions/4532). Right now, Lotus supports a 1-to-1 (client-to-provider) storage and retrieval model where burdens of discoveries are mostly on storage clients and match making services like Estuary. Negotiation is a fairly mannual process and does not support much flexibility. As venus-team is picking up the reminiscences of the [Filecoin Component Architecture](https://docs.google.com/document/d/1ukPD8j6plLEbbzUjxfo7eCauIrOeC_tqxqYK_ls9xbc/edit#), emergent ways of how market could facilitate the dynamics between storage providers and storage clients are constatntly being intergrated into the long-term vision of Venus filecoin. 

## Goals

Current roadmap for venus-market are loosely broken into the following phases.

### Phase 1: peer-to-peer model (short-term)

For phase 1, venus-market will deliver a complete deal making experience as what lotus offers. This includes compatibility with lotus client where one can make deal with venus-market using lotus client, retrieve deal/data in the same way as lotus retrieves its data, setup storage ask and etc.

![image-20210910170740850](https://i.loli.net/2021/09/10/seIgEWBiko6AKc2.png)

- Implementation of the one-to-one model of lotus market like module and fully interoperable with lotus implementation, which means compatibility with lotus client and more
- venus-market deployed as independent module, like venus-sealer and venus-wallet
- Implementation of a reliable market module that runs a seperate process from the main storage process
- A clear module boundary that allows interoperability and user customizations
- Flexibilities of market module to interact with existing venus infrastructures using RPC APIs
- Supports for mainnet, calibration and Nerpa
- Lightweight client: compatibility with Lotus and support for venus-market unique features including client running seperately as a process and remove dependencies for node; great for bootstraping tests on deal making process

### Phase 2: platform model

For phase 2, venus-market is taking the following approach.

**platform-to-peer**: venus-market as deal making backend for middle-man services like Estuary connecting client and provider. As deal market matures, instead of ineffectively advertising one's storage system in #fil-deal-market, storage middleman services like Estuary and Filswan are taking up the roles for distributing datacap more effectively to storage providers looking for deals. Given venus' unique architecture where multiple providers are sharing same infrastructure (chain services), venus-market is in a good position to provide before mentioned deal making backend for a storage middle-man service.

**platform-to-platform**: venus-market as storage backend for a storage integrator (a storage provider who offers different kinds of storage products to its end user, for example, filecoin, S3, tape and etc).

![image-20210910160837732](https://i.loli.net/2021/09/10/sRY5u6Bw9aj713H.png)

- Taking advantages of Venus' distributed architectural nature, a gateway service backend built on top of current infrastructure
- Compact API: seperation of node and venus-market data enabling local storage of some of the deal related meta data
- Data transfer support for different protocols in addition to `Graphsync` [*](https://docs.google.com/document/d/1XWcTp2MEOVtKLpcpiFeeDvc_gTwQ0Bc6yABCTzDmeP0/edit#heading=h.1oxn84bcd1n1)
- Meta data stored locally in HA database like mySQL by venus-market
- venus-market as deal gateway for storage providers using venus chain services (venus shared modules)
- Deal match making: multiple copies for store and faster retrieval

### Phase 3: Decentrialized market (Dp2p) model (long-term vision)

For phase 3, venus-market will look into ways to automate deal flow between client and provider using a peer-to-peer approach, giving up its role as a gateway in phase 2. Additionally, venus pool can be positioned as a retrieval node which is fully aware of deal meta that chain services helped to record.

![image-20210910171104881](https://i.loli.net/2021/09/10/VE6BLpaARrMck9x.png)

- Goals for phase 3 is not as clear cut; require more iterations as filecoin develops smart contracts and others
- auto-match deal market: a service to provide algorithmically (as opposed to manually verifying data using current fil-plus framework) verified data storing/retrieval from peer to peer
- venus-market as gateway for IPFS: options for paid IPFS node
- In time, a platform model as an easy and quick way for matchmaking might fall out of favour and a faster layer 2 protocol could be built on top of Venus to make true p2p data storage with standardized storage services governed by blockchain 
- New econ market on layer 2

## Design ideas

Design draws inspirations from the original [filecoin component document](https://docs.google.com/document/d/1ukPD8j6plLEbbzUjxfo7eCauIrOeC_tqxqYK_ls9xbc/edit#) and [filecoin storage market module](https://docs.google.com/document/d/1FfMUpW8vanR9FrXsybxBBbba7DzeyuCIN2uAXgE7J8U/edit#heading=h.uq51khvyisgr). 

### Terminology

- module and component: "A **module** means a code library that may be compiled and linked against. A **component** means a distinct process presenting or consuming RPC API." In this document, the distinction is not as clear. Will need revamping of the Venus documentation to redefine all terms.
- **GraphSync**: "The default underlying transfer protocol used by the Scheduler. The full graphsync specification can be found at [here](https://github.com/ipld/specs/blob/master/block-layer/graphsync/graphsync.md)."

### Modules and processes

>  In a multi-process architecture, the storage component would form the miner operator’s entry point for all mining and market operations (but not basic node operations) pertaining to a single storage miner actor. It depends on a node to mediate blockchain interactions. **The storage component drives these interactions**. If viewed as a system of services, the storage component is the consumer of a service provided by a node. Thus, the **storage component will depend on an RPC API be provided by a node**. This API is likely to include streaming operations in order to provide continually changing blockchain state to the component. The [mimblewimble/grin project](https://github.com/mimblewimble) is another example of this multi-process node/miner architecture.

Similar to what is described for storage component above, venus-market will be dependent on a RPC API provided by a node ie. chain services, venus shared modules. Blockchain interactions will be handled by venus chain services which can also be extended to handle authentications among others. 

### Deal flow

In phase 1, louts market deal flow will be mirrored in Venus. Maintainace of the market and evolution with the network.

In phase 2, proposing and accepting a deal will work as following.

- [Provider] Add storage ask bidding policy along with other deal acceptance policy
- [Client] Query asks from venus-market with filters like geo locations, redundancy, deal lifespan and etc
- [venus-market] Aggregate requirements from both providers and clients, matchmaking on demand
- [venus-market] Provider(s) and client go through rounds of real-time biding to match-make
- Once matched, provider proceeds to store data as in the one-to-one model

### Meta data

Platform model implementation of venus-market may store metadata on the deals it distributes to providers under its wings. Like Airbnb, it may include metrics that a repututation system of both client and provider can be built upon. Metrics like storage success rate, retrieval success rate, fast retrieval enabled? and etc. 

### Dependencies

1. `venus` module to provide node services
2. `venus-messager` module to provide data services
3. `venus-gateway` to provide signature services
4. `venus-sealer` to provide sealing and data lookup services
5. `go-fil-market` compatible with lotus one-to-one model (For compatbility with lotus only)
6. piece data from external deals
7. datastore using HA databases for deal meta data

*Note that `go-fil-market` included as dependencies is sololy for the use of compatibility with lotus. venus-market will be bundling other unique features along with compatibility with lotus.*

![模块图](https://i.loli.net/2021/09/08/7UxfVujcNPmszyR.jpg)

### Interactions

<img src="https://i.loli.net/2021/09/08/cK2ZHE4DWmYuvLS.png" width="400" >

## Risks   

- Community's concerns on [infrastructure failure](https://filecoinproject.slack.com/archives/CEHHJNJS3/p1627872429033000?thread_ts=1627864468.030900&cid=CEHHJNJS3), node redundancy can be setup
- Data transfer complexities
- Some of the goals in later phases may be moving target as ecosystem evolve 

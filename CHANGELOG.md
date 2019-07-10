# go-filecoin changelog

## go-filecoin 0.3.2

We're happy to announce go-filecoin 0.3.2. This release is a big step towards completing the filecoin storage protocol. It includes many changes to the miner actor builtin smart contract that will allow the network to securely account for verifiable storage power once fault handling is in place. Many less visible but high impact code and testing improvements ship with this release. 0.3.2 also includes a big UX improvement with the new and improved `go-filecoin deals` command for user friendly management of storage deals. Getting paid as a storage miner is now as simple as a single CLI call.

### Features

#### 🏇 Storage protocol nearing completeness

Our number one goal is a network securely powered by verifiable storage. In order for this to work we need to penalize cheating miners who do not prove their storage on time. This release includes most of the groundwork needed, including fundamental data structures and encoding work for tracking sets of sectors, improved power tracking in the miner actor built-in smart contract, and charging fees for late storage proof (PoSt) submissions. Expect these changes to blossom into the [complete fault reporting mechanism](https://github.com/filecoin-project/specs/blob/master/faults.md#market-faults) in the next release.

#### 👪 Multiple sector sizes

In order for the network to scale gracefully, different miners may choose from a variety of different sector sizes to put data in and prove over: smaller sectors for faster and more nimble storage; larger sectors for slower but efficient storage. This release includes all of the software updates we need to support multiple sector sizes in a single network; however, we plan to properly vet network conditions with much bigger sectors before enabling multiple sectors sizes in the user devnet. Expect 1 GiB sectors on the user devnet in the next release.

#### 🤝 Deal management and payments

Both clients and miners can now easily inspect the fine details of all storage deals they have entered into using `go-filecoin deals list` and `go-filecoin deals show`. Miners can get paid for honoring a deal by running `go-filecoin deals redeem`. Additionally this release ships some improvements in payment channel safety for correct arbitration of deal disputes we want down the road.

### Performance and Reliability

#### 🌳 Upgrade in place

This release drives home previous work on repo migrations. The `go-filecoin-migrate` tool (included in the go-filecoin source repo) is now complete. This release includes a proof of concept migration: upgrading on-disk chain metadata from JSON to the more compact CBOR. Landing this means we are confident that this major technical challenge is behind us, putting us one step closer to a reliable, persistent testnet.

### Refactors and Endeavors

#### 📈Major testing improvements

Testing is the silent champion of reliability and development speed. This release includes [tons of ](https://github.com/filecoin-project/go-filecoin/pull/2972)[behind](https://github.com/filecoin-project/go-filecoin/pull/2700) [the scenes](https://github.com/filecoin-project/go-filecoin/pull/2990) [work](https://github.com/filecoin-project/go-filecoin/pull/2919) improving the quality of existing unit and integration tests as well as adding new tests to existing code. Continued improvements to the [FAST](https://github.com/filecoin-project/go-filecoin/tree/master/tools/fast) framework promise to further accelerate integration testing and devnet deployments. 

#### 💳 Tech debt paydown

This release is not playing around when it comes to paying off technical debt. Fundamental chain refactors include an [improved immutable tipset type](https://github.com/filecoin-project/go-filecoin/pull/2837) and tipset cache sharing are at the top of the list. A major refactor of the [message](https://github.com/filecoin-project/go-filecoin/pull/2798) [handling](https://github.com/filecoin-project/go-filecoin/pull/2796) [system](https://github.com/filecoin-project/go-filecoin/pull/2795) into inbox and outbox queues is also a notable improvement. Don’t forget about a consistent internal attoFIL token type, a sleek new miner deal acceptance codepath, sector builder reliability fixes... the list goes on. We are excited to be shipping higher quality software with each release so that we can move faster towards a robust mainnet. 

### Changelog

A full list of [all 207 PRs in this release](https://github.com/search?p=2&q=is%3Apr+merged%3A2019-05-09..2019-07-05+repo%3Afilecoin-project%2Fgo-filecoin+repo%3Afilecoin-project%2Frust-fil-proofs+repo%3Afilecoin-project%2Fspecs&type=Issues), including many bugfixes not listed here, can be found on Github.

### CLI diff

| go-filecoin command | change       |
| ------------------- | ------------ |
| deals list          | added        |
| deals redeem        | added        |
| deals show          | added        |
| miner pledge        | removed      |
| mining status       | added        |
| show block          | args changed |

### Contributors

❤️ Huge thank you to everyone that made this release possible! By alphabetical order, here are all the humans who contributed to this release:

* [@a8159236](https://github.com/a8159236) (3 issues, 2 comments)
* [@Aboatlai](https://github.com/Aboatlai) (1 issue)
* [@acruikshank](https://github.com/acruikshank) (6 commits, 8 PRs, 21 issues, 23 comments)
* [@AkshitV](https://github.com/AkshitV) (1 issue, 2 comments)
* [@alanshaw](https://github.com/alanshaw) (1 comment)
* [@AndyChen1984](https://github.com/AndyChen1984) (1 issue, 5 comments)
* [@anorth](https://github.com/anorth) (22 commits, 24 PRs, 40 issues, 163 comments)
* [@arielgabizon](https://github.com/arielgabizon) (1 issue, 2 comments)
* [@benrogmans](https://github.com/benrogmans) (1 issue)
* [@bvohaska](https://github.com/bvohaska) (1 PR, 1 comment)
* [@callmez](https://github.com/callmez) (1 comment)
* [@carsonfly](https://github.com/carsonfly) (2 issues, 7 comments)
* [@chengzhigen](https://github.com/chengzhigen) (2 issues, 2 comments)
* [@chenhonghe](https://github.com/chenhonghe) (1 issue, 5 comments)
* [@chenxiaolin0105](https://github.com/chenxiaolin0105) (1 issue)
* [@chenzhi201901](https://github.com/chenzhi201901) (2 issues, 1 comment)
* [@codecov-io](https://github.com/codecov-io) (57 comments)
* [@Cryptovideos](https://github.com/Cryptovideos) (1 issue)
* [@dannyhchan](https://github.com/dannyhchan) (4 comments)
* [@dayu26](https://github.com/dayu26) (1 issue)
* [@decentralion](https://github.com/decentralion) (3 commits, 1 PR, 6 comments)
* [@deltazxm](https://github.com/deltazxm) (2 comments)
* [@dignifiedquire](https://github.com/dignifiedquire) (76 commits, 25 PRs, 14 issues, 139 comments)
* [@DrPeterVanNostrand](https://github.com/DrPeterVanNostrand) (1 commit, 1 PR, 2 comments)
* [@eshon](https://github.com/eshon) (1 issue, 8 comments)
* [@frrist](https://github.com/frrist) (14 commits, 18 PRs, 10 issues, 46 comments)
* [@gnunicorn](https://github.com/gnunicorn) (23 commits, 3 PRs, 1 issue, 17 comments)
* [@grandhelmsman](https://github.com/grandhelmsman) (3 issues, 2 comments)
* [@idotial](https://github.com/idotial) (1 issue)
* [@imrehg](https://github.com/imrehg) (1 PR, 1 comment)
* [@ingar](https://github.com/ingar) (5 commits, 6 PRs, 7 comments)
* [@ipfsmainofficial](https://github.com/ipfsmainofficial) (1 issue)
* [@jscode017](https://github.com/jscode017) (1 comment)
* [@Kentix](https://github.com/Kentix) (1 issue, 2 comments)
* [@kishansagathiya](https://github.com/kishansagathiya) (1 PR, 2 comments)
* [@Kubuxu](https://github.com/Kubuxu) (1 commit, 1 PR, 1 comment)
* [@laser](https://github.com/laser) (45 commits, 41 PRs, 24 issues, 97 comments)
* [@maybeuright](https://github.com/maybeuright) (1 comment)
* [@meiqimichelle](https://github.com/meiqimichelle) (1 comment)
* [@merced](https://github.com/merced) (1 issue, 3 comments)
* [@michellebrous](https://github.com/michellebrous) (1 comment)
* [@mishmosh](https://github.com/mishmosh) (3 commits, 2 PRs, 2 issues, 20 comments)
* [@mslipper](https://github.com/mslipper) (5 commits, 1 PR, 8 comments)
* [@nicola](https://github.com/nicola) (2 commits, 1 PR, 4 issues, 11 comments)
* [@nijynot](https://github.com/nijynot) (1 commit, 1 comment)
* [@no1lcy](https://github.com/no1lcy) (1 issue, 1 comment)
* [@ognots](https://github.com/ognots) (6 commits, 5 PRs, 1 issue, 11 comments)
* [@Peachooo](https://github.com/Peachooo) (2 issues, 1 comment)
* [@pooja](https://github.com/pooja) (12 commits, 5 PRs, 9 issues, 45 comments)
* [@porcuquine](https://github.com/porcuquine) (8 commits, 4 PRs, 7 issues, 42 comments)
* [@R-Niagra](https://github.com/R-Niagra) (1 issue, 1 comment)
* [@ridewindx](https://github.com/ridewindx) (1 commit, 1 PR)
* [@RobQuistNL](https://github.com/RobQuistNL) (2 comments)
* [@rogerlzp](https://github.com/rogerlzp) (1 comment)
* [@rosalinekarr](https://github.com/rosalinekarr) (15 commits, 15 PRs, 3 issues, 36 comments)
* [@schomatis](https://github.com/schomatis) (22 commits, 11 PRs, 3 issues, 28 comments)
* [@shannonwells](https://github.com/shannonwells) (8 commits, 8 PRs, 5 issues, 11 comments)
* [@sidke](https://github.com/sidke) (13 commits, 1 comment)
* [@Stebalien](https://github.com/Stebalien) (1 commit, 1 PR, 1 comment)
* [@sternhenri](https://github.com/sternhenri) (4 PRs, 1 issue, 24 comments)
* [@steven004](https://github.com/steven004) (1 commit, 1 PR, 3 issues, 7 comments)
* [@taoshengshi](https://github.com/taoshengshi) (2 issues, 6 comments)
* [@taylorshuang](https://github.com/taylorshuang) (2 issues, 6 comments)
* [@titilami](https://github.com/titilami) (3 issues, 2 comments)
* [@travisperson](https://github.com/travisperson) (3 commits, 3 PRs, 6 issues, 25 comments)
* [@urugang](https://github.com/urugang) (1 issue)
* [@vhosakot](https://github.com/vhosakot) (1 comment)
* [@vmx](https://github.com/vmx) (3 commits, 4 PRs, 14 comments)
* [@vyzo](https://github.com/vyzo) (1 comment)
* [@warpfork](https://github.com/warpfork) (3 comments)
* [@waynewyang](https://github.com/waynewyang) (3 commits, 4 PRs, 1 issue, 3 comments)
* [@whyrusleeping](https://github.com/whyrusleeping) (72 commits, 15 PRs, 11 issues, 73 comments)
* [@windemut](https://github.com/windemut) (1 issue, 5 comments)
* [@yangjian102621](https://github.com/yangjian102621) (2 issues, 5 comments)
* [@yaohcn](https://github.com/yaohcn) (1 commit, 1 PR, 1 comment)
* [@yusefnapora](https://github.com/yusefnapora) (1 comment)
* [@ZenGround0](https://github.com/ZenGround0) (9 commits, 9 PRs, 23 issues, 37 comments)
* [@zhengboowen](https://github.com/zhengboowen) (3 issues)
* [@zixuanzh](https://github.com/zixuanzh) (1 PR)

### 🙌🏽 Want to contribute?

Would you like to contribute to the Filecoin project and don’t know how? Here are a few places you can get started:

- Check out the [Contributing Guidelines](https://github.com/filecoin-project/go-filecoin/blob/master/CONTRIBUTING.md)

- Look for issues with the `good-first-issue` label in [go-filecoin](https://docs.google.com/document/d/1dfTVASs9cQMo4NPqJmXjEEX-Ju_M9Vw-4AelN1aHOV8/edit#) and [rust-fil-proofs](https://github.com/filecoin-project/rust-fil-proofs/issues?q=is%3Aissue+is%3Aopen+label%3A"good+first+issue")

- Join the [community chat on Matrix/Slack](https://github.com/filecoin-project/community#chat), introduce yourself in #_fil-lobby, and let us know where you would like to contribute

- Join the [user devnet](https://github.com/filecoin-project/go-filecoin/wiki/Getting-Started)

### ⁉️ Do you have questions?

The best place to ask your questions about go-filecoin, how it works, and what you can do with it is at [discuss.filecoin.io](https://discuss.filecoin.io). We are also available at the [community chat on Matrix/Slack](https://github.com/filecoin-project/community#chat).

---

## go-filecoin 0.2.4

We're happy to announce go-filecoin 0.2.4. This is a patch release with block validation improvements. As a placeholder before full implementation of block validation, block time was hardcoded to 30 seconds. It was also possible to manually configure a shorter block time via the CLI — miners who did this gained an unfair block mining advantage. Over the past few weeks, a handful of enterprising devnet participants¹ 😉 increasingly used this undocumented option to the point of severely degrading the devnet for everyone else. To get the devnet running smoothly again, we are releasing partial [block validation](https://github.com/filecoin-project/specs/pull/289).

#### 🌳 Features

- Timestamp block | [go-filecoin #2897](https://github.com/filecoin-project/go-filecoin/pull/2897)
- Partial Block validation | [go-filecoin #2899](https://github.com/filecoin-project/go-filecoin/pull/2899), [go-filecoin #2882](https://github.com/filecoin-project/go-filecoin/pull/2882), [go-filecoin #2914](https://github.com/filecoin-project/go-filecoin/pull/2914)

#### ☝🏽 Upgrade notice

As a reminder, only the latest version of go-filecoin will connect to the user devnet until protocol upgrade work is complete. Users will need to upgrade to 0.2.4 to connect to the user devnet.

[1] If that was you, we’d love to collaborate to see if you can find other ways to break our implementation! Please email us at [mining@filecoin.io](mailto:mining@filecoin.io).

---

## go-filecoin 0.2.2

We're happy to announce go-filecoin 0.2.2. This is a maintenance release with bug fixes and debugging improvements. After the 0.2.1 release, we found a bug in the dht ([#2753](https://github.com/filecoin-project/go-filecoin/issues/2753)) that caused some nodes to panic. This was fixed in [#2754](https://github.com/filecoin-project/go-filecoin/pull/2754) by bumping the [go-libp2p-kad-dht](https://github.com/libp2p/go-libp2p-kad-dht) version from 0.0.4 to 0.0.8.

#### 🐞 Bug fixes

- Update to go-libp2p-kad-dht@v0.0.8 | [go-filecoin #2754](https://github.com/filecoin-project/go-filecoin/pull/2754)
- Fix output for set price | [go-filecoin #2727](https://github.com/filecoin-project/go-filecoin/pull/2727)

#### 🌳 Features

- Add an approval step to user devnet deploy | [go-filecoin #2765](https://github.com/filecoin-project/go-filecoin/pull/2765)
- Log messages proper printing | [go-filecoin #2728](https://github.com/filecoin-project/go-filecoin/pull/2728)
- Add filecoin version command to inspect output | [go-filecoin #2725](https://github.com/filecoin-project/go-filecoin/pull/2725)

#### ☝🏽 Upgrade notice

As a reminder, only the latest version of go-filecoin will connect to the user devnet until model for change work is complete. Users will need to upgrade to 0.2.2 to connect to the user devnet.

---

## go-filecoin 0.2.1

We're happy to announce go-filecoin 0.2.1. This release is heavy on behind-the-scenes upgrades, including support for filesystem repo migrations and storage disputes, a better message pool, proofs improvements, and a bump to libp2p version for more reliable relays. User-facing improvements such as new commands and options, better status messages, and lots of bugfixes are also included. Get pumped! 🎁

### Install and Setup

#### ⌛ Chain syncing status

When a filecoin node is first created, it must download and verify the chain. We call this “chain syncing”. While initial commands (such as tapping the faucet or dashboard streaming) can be run immediately, any other commands (such as mining commands) will return errors until chain syncing is complete. Currently, this can take several hours.

To clarify, we’ve added [wiki updates](https://github.com/filecoin-project/go-filecoin/wiki/Getting-Started#wait-for-chain-sync), better status messages, and cleaner console output for chain syncing. In future releases, we’ll also address the underlying problem of slow chain syncing.

#### 💠 Sector storage configuration

Where would you like the filecoin node to store client data? You can now choose! There are two ways to specify the location of the sector storage directory: the `sectorbase.rootdir` config entry, or the `--sectordir` option to `go-filecoin init`.

If you don’t specify a location, data is stored in `$HOME/.filecoin_sectors` by default.

### Features

#### 🍄 Upgradeable repo

In addition to sealed client data, Filecoin nodes also store other data on-disk such as configuration data, blockchain blocks, deal state, and encryption keys. As development progresses, we need a way to safely change the type and schema of this data. In this release, we include an accepted [design](https://docs.google.com/document/d/1THzh1mrNCKYbdk1zP72xV8pfr1yQBe2n3ptrSAYyVI8/) for filesystem repo migrations, and an initial layout for the migration tool. This paves the way for filecoin nodes to seamlessly update when running in production.

For more information, check out the help text:

```
tools/migration/go-filecoin-migrate --help
```

#### 💎 Storage payments

This release includes work towards storage protocol dispute resolution. Payment channels can now contain conditions that will query another actor before a voucher is redeemed. Payment channels can also be canceled by the payer. This will trigger an early close if the target of the channel does not redeem a payment. These features can be used together with piece inclusion proofs (coming soon) to enforce proof of storage when storage clients pay storage miners.

#### 🐛 New debugging commands 

Three new commands (`inspect`, `protocol`, and `bitswap`) are now available for your debugging and exploring adventures:

\* `go-filecoin inspect all` prints all the necessary information for opening a bug report on GitHub. This includes operating system details, your current go-filecoin config, and a few other commonly needed stats. 

\* `go-filecoin protocol` prints details regarding parameters for a node’s protocol, such as autoseal interval and sector size. These are helpful for debugging some of the internals of Filecoin’s proofs and protocol systems.

\* `go-filecoin bitswap` prints details about a node’s libp2p bitswap system, such as blocks, data, and messages received and sent. These are commonly used in network debugging. 

For more details, run any command followed by the `--help` flag.

### Performance and Reliability

#### 🙌 Upgrade libp2p to 0.0.16

libp2p recently landed a bunch of improvements to relay functionality, addressing heavy resource usage in some production relay nodes. We’ve upgraded to [go-libp2p](http://github.com/libp2p/go-libp2p) 0.0.16 to enjoy the same fixes in filecoin. 

#### 📬 Better message validation

We’ve taken several steps to harden the message pool. The pool now rejects messages that will obviously fail processing due to problems like invalid signature, insufficient funds, no gas, or non-existent actor. It also tracks nonces to ensure that messages are correctly sequenced, and that no account has too many messages in the pool. Finally, the pool now limits the total messages it will accept.

#### 🔗 Proofs integration

Behind the scenes, much groundwork has been laid for more flexible and powerful storage proofs. This release includes more efficient memory utilization when writing large pieces to a sector. It also includes initial support for piece inclusion proofs, [multiple sector sizes](https://github.com/filecoin-project/go-filecoin/issues/2530), and [variable proof lengths](https://github.com/filecoin-project/go-filecoin/pull/2607). 

#### 🔮 Proofs performance

Over in `rust-fil-proofs`, progress is accelerating on more complete and efficient implementations. This includes switching to [mmap for more efficient merkle trees](https://github.com/filecoin-project/rust-fil-proofs/pull/529), [abstractions over the hasher](https://github.com/filecoin-project/rust-fil-proofs/pull/543), [limiting parallelism when generating groth proofs](https://github.com/filecoin-project/rust-fil-proofs/pull/582), and [calculating](https://github.com/filecoin-project/rust-fil-proofs/pull/621) and [aggregating](https://github.com/filecoin-project/rust-fil-proofs/pull/605) challenges across partitions.

### Refactors and Endeavors

#### 🏁 FAST (Filecoin Automation & System Toolkit)

We have significantly improved the FAST testing system for Filecoin since the last release. FAST now automatically includes relevant log data and messages from testing nodes in the event of a test failure. FAST also has an all-new localnet tool to quickly and easily set up local Filecoin node clusters for testing and experimentation. See [the localnet readme](https://github.com/filecoin-project/go-filecoin/blob/master/tools/fast/bin/localnet/README.md) for details.

#### 👾 Go modules

With Go 1.11’s preliminary support for versioned modules, we have switched to [Go modules](https://github.com/golang/go/wiki/Modules) for dependency management. This allows for easier dependency management and faster updates when dealing with updates from upstream dependencies.

#### 😍 Design documents

We regularly write [design docs](https://github.com/filecoin-project/designdocs/blob/master/designdocs.md) before coding begins on important features or components. These short documents are useful in capturing knowledge, formalizing our thinking, and sharing design intent. Going forward, you can find new design docs in the [designdocs](https://github.com/filecoin-project/designdocs/) repo.

### Changelog

A full list of [all 177 PRs in this release](https://github.com/search?q=is%3Apr+merged%3A2019-03-26..2019-05-10+repo%3Afilecoin-project%2Fgo-filecoin+repo%3Afilecoin-project%2Frust-fil-proofs+repo%3Afilecoin-project%2Fspecs&type=Issues), including many bugfixes not listed here, can be found on Github.

### Contributors

❤️ Huge thank you to everyone that made this release possible! By alphabetical order, here are all the humans who contributed to this release via the `go-filecoin`, `rust-fil-proofs`, and `specs` repos:

* [@814556001](https://github.com/814556001) (1 comment)                                                                
* [@a8159236](https://github.com/a8159236) (3 issues, 9 comments)                                                       
* [@aaronhenshaw](https://github.com/aaronhenshaw) (1 issue, 1 comment)                                                 
* [@AbelLaker](https://github.com/AbelLaker) (2 issues, 2 comments)                                                     
* [@acruikshank](https://github.com/acruikshank) (47 commits, 24 PRs, 42 issues, 81 comments)                           
* [@aioloszcy](https://github.com/aioloszcy) (2 issues)                                                                 
* [@alanshaw](https://github.com/alanshaw) (1 commit, 1 PR, 4 comments)                                                 
* [@anacrolix](https://github.com/anacrolix) (2 commits, 2 PRs, 17 comments)                                            
* [@andrewxhill](https://github.com/andrewxhill) (1 issue)                                                              
* [@AndyChen1984](https://github.com/AndyChen1984) (5 issues, 9 comments)                                               
* [@anorth](https://github.com/anorth) (61 commits, 65 PRs, 46 issues, 340 comments)                                    
* [@arcalinea](https://github.com/arcalinea) (1 issue, 4 comments)                                                      
* [@arielgabizon](https://github.com/arielgabizon) (1 issue)                                                            
* [@arsstone](https://github.com/arsstone) (1 PR, 1 issue, 6 comments)                                                  
* [@aschmahmann](https://github.com/aschmahmann) (4 comments)                                                           
* [@bigs](https://github.com/bigs) (1 comment)                                                                          
* [@block2020](https://github.com/block2020) (5 issues, 1 comment)                                                      
* [@btcioner](https://github.com/btcioner) (2 comments)                                                                 
* [@bvohaska](https://github.com/bvohaska) (1 commit, 1 PR, 6 issues, 26 comments)                                      
* [@Byte-Doctor](https://github.com/Byte-Doctor) (1 issue)                                                              
* [@cgwyx](https://github.com/cgwyx) (2 comments)                                                                       
* [@chenminjian](https://github.com/chenminjian) (1 issue, 3 comments)                                                  
* [@comradekingu](https://github.com/comradekingu) (1 commit, 1 PR)                                                     
* [@contrun](https://github.com/contrun) (4 commits, 5 PRs, 1 issue, 7 comments)                                        
* [@craigbranscom](https://github.com/craigbranscom) (1 issue)                                                          
* [@creationix](https://github.com/creationix) (1 comment)                                                              
* [@Cyanglacier](https://github.com/Cyanglacier) (1 issue)                                                              
* [@Daniel-Wang](https://github.com/Daniel-Wang) (1 commit, 1 PR, 1 comment)                                            
* [@danigrant](https://github.com/danigrant) (2 commits, 2 PRs)                                                         
* [@dayou5168](https://github.com/dayou5168) (6 issues, 17 comments)                                                    
* [@dayu26](https://github.com/dayu26) (1 comment)                                                                      
* [@deaswang](https://github.com/deaswang) (1 comment)                                                                  
* [@decentralion](https://github.com/decentralion) (1 issue, 12 comments)                                               
* [@deltazxm](https://github.com/deltazxm) (1 issue, 5 comments)                                                        
* [@dignifiedquire](https://github.com/dignifiedquire) (49 commits, 32 PRs, 16 issues, 151 comments)                    
* [@diwufeiwen](https://github.com/diwufeiwen) (3 issues, 3 comments)                                                   
* [@djdv](https://github.com/djdv) (2 comments)                                                                         
* [@DonaldTsang](https://github.com/DonaldTsang) (1 issue)                                                              
* [@EbonyBelle](https://github.com/EbonyBelle) (1 comment)                                                              
* [@ebuchman](https://github.com/ebuchman) (1 issue)
* [@eefahy](https://github.com/eefahy) (1 comment)
* [@ElecRoastChicken](https://github.com/ElecRoastChicken) (2 comments)
* [@evildido](https://github.com/evildido) (1 issue, 3 comments)
* [@fengchenggang1](https://github.com/fengchenggang1) (1 issue)
* [@firmianavan](https://github.com/firmianavan) (1 commit, 2 PRs, 3 comments)
* [@fjl](https://github.com/fjl) (4 comments)
* [@frrist](https://github.com/frrist) (100 commits, 51 PRs, 44 issues, 111 comments)
* [@gfc-test](https://github.com/gfc-test) (1 PR)
* [@gmas](https://github.com/gmas) (12 commits)
* [@gmasgras](https://github.com/gmasgras) (22 commits, 19 PRs, 14 issues, 35 comments)
* [@gnunicorn](https://github.com/gnunicorn) (1 comment)
* [@haadcode](https://github.com/haadcode) (1 issue)
* [@hango-hango](https://github.com/hango-hango) (1 comment)
* [@haoglehaogle](https://github.com/haoglehaogle) (1 issue)
* [@hsanjuan](https://github.com/hsanjuan) (3 commits, 2 PRs, 7 comments)
* [@ianjdarrow](https://github.com/ianjdarrow) (5 comments)
* [@imrehg](https://github.com/imrehg) (7 issues, 4 comments)
* [@ipfsmainofficial](https://github.com/ipfsmainofficial) (1 issue, 1 comment)
* [@irocnX](https://github.com/irocnX) (1 issue, 1 comment)
* [@jamiew](https://github.com/jamiew) (1 comment)
* [@jaybutera](https://github.com/jaybutera) (1 issue)
* [@jbenet](https://github.com/jbenet) (1 commit, 4 issues, 8 comments)
* [@jcchua](https://github.com/jcchua) (1 issue, 1 comment)
* [@jesseclay](https://github.com/jesseclay) (1 issue, 1 comment)
* [@jhiesey](https://github.com/jhiesey) (1 issue)
* [@jimpick](https://github.com/jimpick) (1 issue, 3 comments)
* [@joshgarde](https://github.com/joshgarde) (4 comments)
* [@jscode017](https://github.com/jscode017) (2 commits, 2 PRs, 4 issues, 17 comments)
* [@karalabe](https://github.com/karalabe) (1 issue, 4 comments)
* [@kishansagathiya](https://github.com/kishansagathiya) (1 issue, 4 comments)
* [@Kostadin](https://github.com/Kostadin) (1 commit, 1 PR)
* [@Kubuxu](https://github.com/Kubuxu) (13 commits, 9 PRs, 8 comments)
* [@lanzafame](https://github.com/lanzafame) (2 commits, 1 PR, 1 issue, 4 comments)
* [@laser](https://github.com/laser) (73 commits, 64 PRs, 77 issues, 178 comments)
* [@leinue](https://github.com/leinue) (1 issue, 1 comment)
* [@lidel](https://github.com/lidel) (3 comments)
* [@life-i](https://github.com/life-i) (1 issue, 3 comments)
* [@lin6461](https://github.com/lin6461) (2 issues, 5 comments)
* [@linsheng9731](https://github.com/linsheng9731) (1 issue)
* [@loulancn](https://github.com/loulancn) (1 issue, 1 comment)
* [@Luca8991](https://github.com/Luca8991) (1 issue)
* [@madper](https://github.com/madper) (1 commit, 1 PR)
* [@magik6k](https://github.com/magik6k) (4 commits, 4 PRs, 9 comments)
* [@MariusVanDerWijden](https://github.com/MariusVanDerWijden) (2 comments)
* [@markwylde](https://github.com/markwylde) (2 issues, 5 comments)
* [@mburns](https://github.com/mburns) (1 PR)
* [@mgoelzer](https://github.com/mgoelzer) (2 issues, 7 comments)
* [@mhammersley](https://github.com/mhammersley) (3 issues, 15 comments)
* [@mikeal](https://github.com/mikeal) (1 PR, 1 issue, 2 comments)
* [@mishmosh](https://github.com/mishmosh) (21 commits, 8 PRs, 35 issues, 159 comments)
* [@mkky-lisheng](https://github.com/mkky-lisheng) (1 issue, 1 comment)
* [@moyid](https://github.com/moyid) (4 comments)
* [@mslipper](https://github.com/mslipper) (9 commits, 11 PRs, 7 issues, 51 comments)
* [@muronglaowang](https://github.com/muronglaowang) (8 issues, 7 comments)
* [@Nanofortress](https://github.com/Nanofortress) (1 issue, 4 comments)
* [@NatoBoram](https://github.com/NatoBoram) (3 issues, 9 comments)
* [@nicola](https://github.com/nicola) (17 commits, 5 PRs, 7 issues, 25 comments)
* [@nijynot](https://github.com/nijynot) (1 PR)
* [@ognots](https://github.com/ognots) (56 commits, 37 PRs, 19 issues, 86 comments)
* [@olizilla](https://github.com/olizilla) (1 commit, 1 PR)
* [@Pacius](https://github.com/Pacius) (1 issue)
* [@ParadiseTaboo](https://github.com/ParadiseTaboo) (1 comment)
* [@pengxiankaikai](https://github.com/pengxiankaikai) (7 issues, 15 comments)
* [@phritz](https://github.com/phritz) (13 commits, 11 PRs, 50 issues, 366 comments)
* [@pkrasam](https://github.com/pkrasam) (1 issue, 1 comment)
* [@pooja](https://github.com/pooja) (5 commits, 1 PR, 11 issues, 95 comments)
* [@porcuquine](https://github.com/porcuquine) (62 commits, 25 PRs, 31 issues, 246 comments)
* [@protocolin](https://github.com/protocolin) (1 issue)
* [@pxrxingrui520](https://github.com/pxrxingrui520) (1 issue)
* [@rafael81](https://github.com/rafael81) (2 commits, 2 PRs, 1 issue, 3 comments)
* [@raulk](https://github.com/raulk) (4 commits, 5 PRs, 22 comments)
* [@redransil](https://github.com/redransil) (1 issue)
* [@RichardLitt](https://github.com/RichardLitt) (1 commit, 1 PR)
* [@ridewindx](https://github.com/ridewindx) (2 commits, 2 PRs)
* [@rjan90](https://github.com/rjan90) (1 comment)
* [@rkowalick](https://github.com/rkowalick) (52 commits, 46 PRs, 17 issues, 106 comments)
* [@RobQuistNL](https://github.com/RobQuistNL) (1 issue, 7 comments)
* [@rosalinekarr](https://github.com/rosalinekarr) (38 commits, 39 PRs, 48 issues, 157 comments)
* [@sanchopansa](https://github.com/sanchopansa) (2 comments)
* [@sandjj](https://github.com/sandjj) (5 issues, 8 comments)
* [@SaveTheAles](https://github.com/SaveTheAles) (1 issue, 3 comments)
* [@schomatis](https://github.com/schomatis) (47 commits, 22 PRs, 12 issues, 173 comments)
* [@scout](https://github.com/scout) (3 comments)
* [@SCUTVincent](https://github.com/SCUTVincent) (1 issue, 2 comments)
* [@shannonwells](https://github.com/shannonwells) (23 commits, 24 PRs, 43 issues, 68 comments)
* [@sidke](https://github.com/sidke) (79 commits, 22 PRs, 18 issues, 12 comments)
* [@SmartMeshFoundation](https://github.com/SmartMeshFoundation) (1 issue)
* [@songjiayang](https://github.com/songjiayang) (1 comment)
* [@Stebalien](https://github.com/Stebalien) (4 commits, 6 PRs, 18 comments)
* [@sternhenri](https://github.com/sternhenri) (38 commits, 10 PRs, 5 issues, 50 comments)
* [@steven004](https://github.com/steven004) (3 commits, 7 PRs, 4 issues, 11 comments)
* [@sywyn219](https://github.com/sywyn219) (3 issues, 13 comments)
* [@Tbaut](https://github.com/Tbaut) (1 issue)
* [@terichadbourne](https://github.com/terichadbourne) (1 issue, 12 comments)
* [@thomas92911](https://github.com/thomas92911) (1 issue, 1 comment)
* [@travisperson](https://github.com/travisperson) (98 commits, 53 PRs, 40 issues, 190 comments)
* [@tycholiu](https://github.com/tycholiu) (1 comment)
* [@urugang](https://github.com/urugang) (1 PR, 1 issue, 1 comment)
* [@vmx](https://github.com/vmx) (8 commits, 5 PRs, 2 issues, 19 comments)
* [@vyzo](https://github.com/vyzo) (8 comments)
* [@warpfork](https://github.com/warpfork) (6 comments)
* [@waynewyang](https://github.com/waynewyang) (3 commits, 5 PRs, 2 issues, 8 comments)
* [@whyrusleeping](https://github.com/whyrusleeping) (157 commits, 42 PRs, 55 issues, 296 comments)
* [@windstore](https://github.com/windstore) (1 issue, 2 comments)
* [@woshihanhaoniao](https://github.com/woshihanhaoniao) (5 issues, 6 comments)
* [@wyblyf](https://github.com/wyblyf) (1 issue, 6 comments)
* [@xcshuan](https://github.com/xcshuan) (1 issue, 1 comment)
* [@yangjian102621](https://github.com/yangjian102621) (1 PR, 4 issues, 16 comments)
* [@yaohcn](https://github.com/yaohcn) (1 PR, 1 issue, 3 comments)
* [@yph152](https://github.com/yph152) (1 issue)
* [@ytQiao](https://github.com/ytQiao) (1 issue, 2 comments)
* [@yusefnapora](https://github.com/yusefnapora) (1 comment)
* [@yyh1102](https://github.com/yyh1102) (2 comments)
* [@zebul](https://github.com/zebul) (1 issue)
* [@ZenGround0](https://github.com/ZenGround0) (35 commits, 29 PRs, 85 issues, 128 comments)
* [@zhangkuicheng](https://github.com/zhangkuicheng) (2 issues, 4 comments)
* [@zixuanzh](https://github.com/zixuanzh) (4 comments)
* [@zjoooooo](https://github.com/zjoooooo) (1 issue, 1 comment)

### 🙌🏽 Want to contribute?

Would you like to contribute to the Filecoin project and don’t know how? Here are a few places you can get started:

- Check out the [Contributing Guidelines](https://github.com/filecoin-project/go-filecoin/blob/master/CONTRIBUTING.md)
- Look for issues with the `good-first-issue` label in [go-filecoin](https://github.com/filecoin-project/go-filecoin/issues?q=is%3Aissue+is%3Aopen+label%3A"good+first+issue") and [rust-fil-proofs](https://github.com/filecoin-project/rust-fil-proofs/issues?q=is%3Aissue+is%3Aopen+label%3A"good+first+issue")
- Join the [community chat on Matrix/Slack](https://github.com/filecoin-project/community#chat), introduce yourself in #_fil-lobby, and let us know where you would like to contribute

### ⁉️ Do you have questions?

The best place to ask your questions about go-filecoin, how it works, and what you can do with it is at [discuss.filecoin.io](https://discuss.filecoin.io). We are also available at the [community chat on Matrix/Slack](https://github.com/filecoin-project/community#chat).

---

## go-filecoin 0.1.4

We're happy to announce go-filecoin 0.1.4. This release contains a better install experience, initial Proof-of-Spacetime integration, more reliable message sending and networking, and many other improvements. Get pumped! 🎁

### Install and Setup

#### 💝 Binary releases

Linux and MacOS binaries for go-filecoin are now available! See [Installing from binary](https://github.com/filecoin-project/go-filecoin/wiki/Getting-Started#installing-from-binary) for instructions.

#### 🍱 Precompiled proofs parameters

Running secure proofs requires parameter files that are several GB in size. Previously, these files were generated at install, an extremely memory-intensive process causing slow or impossible builds for many users. Now, you can download pre-generated files during install by running `paramfetch`. This step is now included in the [Installing from binary](https://github.com/filecoin-project/go-filecoin/wiki/Getting-Started#installing-from-binary) instructions.

#### 🦖 Version checking

go-filecoin now checks that it is running the same version (at the same commit) while connecting to a devnet. This is a temporary fix until a model for change is implemented, allowing different versions to interoperate.

### Features

#### 💎 Proof-of-Spacetime Integration

Miners now call `rust-fil-proofs` to periodically generate proofs of continued storage. With this major integration in place, you can expect some follow-up  (for example, storage mining faults do not yet appear on-chain) and continued optimizations to the underlying Proof-of-Spacetime construction and implementation.

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

Previously, the node object contained both interfaces and internals for much of the core protocol. It was difficult to unit test due to many dependencies and complicated setup. Following the [porcelain and plumbing pattern from Git](https://git-scm.com/book/en/v2/Git-Internals-Plumbing-and-Porcelain), we have now decoupled the node object from many of its dependencies. We have also created a separate API for block, storage, and retrieval mining.

### Changelog

A full list of [all 200 PRs in this release](https://github.com/filecoin-project/go-filecoin/pulls?utf8=%E2%9C%93&q=is%3Apr+merged%3A2019-02-14..2019-03-26) can be found on Github.

### Contributors

❤️ Huge thank you to everyone that made this release possible! By alphabetical order, here are all the humans who contributed issues and commits in `go-filecoin` and `rust-fil-proofs` to date:

- [@aaronhenshaw](http://github.com/aaronhenshaw)
- [@aboodman](http://github.com/aboodman)
- [@AbelLaker](http://github.com/AbelLaker)
- [@alanshaw](http://github.com/alanshaw)
- [@acruikshank](http://github.com/acruikshank)
- [@anacrolix](http://github.com/anacrolix)
- [@andychen1984](http://github.com/andychen1984)
- [@anorth](http://github.com/anorth)
- [@Byte-Doctor](http://github.com/Byte-Doctor)
- [@chenminjuan](http://github.com/chenminjuan)
- [@coderlane](http://github.com/coderlane)
- [@comeradekingu](http://github.com/comeradekingu)
- [@danigrant](http://github.com/danigrant)
- [@dayou5168](http://github.com/dayou5168)
- [@dignifiedquire](http://github.com/dignifiedquire)
- [@diwufeiwen](http://github.com/diwufeiwen)
- [@ebuchman](http://github.com/ebuchman)
- [@eefahy](http://github.com/eefahy)
- [@firmianavan](http://github.com/firmianavan)
- [@frrist](http://github.com/frrist)
- [@gmasgras](http://github.com/gmasgras)
- [@haoglehaogle](http://github.com/haoglehaogle)
- [@hsanjuan](http://github.com/hsanjuan)
- [@imrehg](http://github.com/imrehg)
- [@jaybutera](http://github.com/jaybutera)
- [@jbenet](http://github.com/jbenet)
- [@jimpick](http://github.com/jimpick)
- [@karalabe](http://github.com/karalabe)
- [@kubuxu](http://github.com/kubuxu)
- [@lanzafame](http://github.com/lanzafame)
- [@laser](http://github.com/laser)
- [@leinue](http://github.com/leinue)
- [@life-i](http://github.com/life-i)
- [@luca8991](http://github.com/luca8991)
- [@madper](http://github.com/madper)
- [@magik6k](http://github.com/magik6k)
- [@markwylde](http://github.com/markwylde)
- [@mburns](http://github.com/mburns)
- [@michellebrous](http://github.com/michellebrous)
- [@mikael](http://github.com/mikael)
- [@mishmosh](http://github.com/mishmosh)
- [@mslipper](http://github.com/mslipper)
- [@muronglaowang](http://github.com/muronglaowang)
- [@nanofortress](http://github.com/nanofortress)
- [@natoboram](http://github.com/natoboram)
- [@nicola](http://github.com/nicola)
- [@ognots](http://github.com/ognots)
- [@olizilla](http://github.com/olizilla)
- [@pacius](http://github.com/pacius)
- [@pengxiankaikai](http://github.com/pengxiankaikai)
- [@pooja](http://github.com/pooja)
- [@porcuquine](http://github.com/porcuquine)
- [@phritz](http://github.com/phritz)
- [@pkrasam](http://github.com/pkrasam)
- [@pxrxingrui520](http://github.com/pxrxingrui520)
- [@raulk](http://github.com/raulk)
- [@rafael81](http://github.com/rafael81)
- [@richardlitt](http://github.com/richardlitt)
- [@rkowalick](http://github.com/rkowalick)
- [@rosalinekarr](http://github.com/rosalinekarr)
- [@sandjj](http://github.com/sandjj)
- [@schomatis](http://github.com/schomatis)
- [@shannonwells](http://github.com/shannonwells)
- [@sidka](http://github.com/sidka)
- [@stebalien](http://github.com/stebalien)
- [@steven004](http://github.com/steven004)
- [@sywyn219](http://github.com/sywyn219)
- [@tbaut](http://github.com/tbaut)
- [@thomas92911](http://github.com/thomas92911)
- [@travisperson](http://github.com/travisperson)
- [@vmx](http://github.com/vmx)
- [@waynewyang](http://github.com/waynewyang)
- [@whyrusleeping](http://github.com/whyrusleeping)
- [@windstore](http://github.com/windstore)
- [@woshihanhaoniao](http://github.com/woshihanhaoniao)
- [@xcshuan](http://github.com/xcshuan)
- [@yangjian102621](http://github.com/yangjian102621)
- [@yph152](http://github.com/yph152)
- [@zenground0](http://github.com/zenground0)
- [@zhangkuicheng](http://github.com/zhangkuicheng)
- [@zjoooooo](http://github.com/zjoooooo)

### 🙌🏽 Want to contribute?

Would you like to contribute to the Filecoin project and don’t know how? Here are a few places you can get started:

- Check out the [Contributing Guidelines](https://github.com/filecoin-project/go-filecoin/blob/master/CONTRIBUTING.md)
- Look for issues with the `good-first-issue` label in [go-filecoin](https://github.com/filecoin-project/go-filecoin/issues?utf8=%E2%9C%93&q=is%3Aissue+is%3Aopen+label%3A%22e-good-first-issue%22+) and [rust-fil-proofs](https://github.com/filecoin-project/rust-fil-proofs/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22)
- Join the [community chat on Matrix/Slack](https://github.com/filecoin-project/community#chat), introduce yourself in #_fil-lobby, and let us know where you would like to contribute

### ⁉️ Do you have questions?

The best place to ask your questions about go-filecoin, how it works, and what you can do with it is at [discuss.filecoin.io](https://discuss.filecoin.io). We are also available at the [community chat on Matrix/Slack](https://github.com/filecoin-project/community#chat).

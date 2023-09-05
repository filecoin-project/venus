# Sample code of curl

```bash
# <Inputs> corresponding to the value of Inputs Tag of each API
curl http://<ip>:<port>/rpc/v1 -X POST -H "Content-Type: application/json"  -H "Authorization: Bearer <token>"  -d '{"method": "Filecoin.<method>", "params": <Inputs>, "id": 0}'
```
# Groups

* [Account](#account)
  * [StateAccountKey](#stateaccountkey)
* [Actor](#actor)
  * [ListActor](#listactor)
  * [StateGetActor](#stategetactor)
* [BlockStore](#blockstore)
  * [ChainDeleteObj](#chaindeleteobj)
  * [ChainHasObj](#chainhasobj)
  * [ChainPutObj](#chainputobj)
  * [ChainReadObj](#chainreadobj)
  * [ChainStatObj](#chainstatobj)
* [ChainInfo](#chaininfo)
  * [BlockTime](#blocktime)
  * [ChainExport](#chainexport)
  * [ChainGetBlock](#chaingetblock)
  * [ChainGetBlockMessages](#chaingetblockmessages)
  * [ChainGetEvents](#chaingetevents)
  * [ChainGetGenesis](#chaingetgenesis)
  * [ChainGetMessage](#chaingetmessage)
  * [ChainGetMessagesInTipset](#chaingetmessagesintipset)
  * [ChainGetParentMessages](#chaingetparentmessages)
  * [ChainGetParentReceipts](#chaingetparentreceipts)
  * [ChainGetPath](#chaingetpath)
  * [ChainGetReceipts](#chaingetreceipts)
  * [ChainGetTipSet](#chaingettipset)
  * [ChainGetTipSetAfterHeight](#chaingettipsetafterheight)
  * [ChainGetTipSetByHeight](#chaingettipsetbyheight)
  * [ChainHead](#chainhead)
  * [ChainList](#chainlist)
  * [ChainNotify](#chainnotify)
  * [ChainSetHead](#chainsethead)
  * [GetActor](#getactor)
  * [GetEntry](#getentry)
  * [GetFullBlock](#getfullblock)
  * [GetParentStateRootActor](#getparentstaterootactor)
  * [ProtocolParameters](#protocolparameters)
  * [ResolveToKeyAddr](#resolvetokeyaddr)
  * [StateActorCodeCIDs](#stateactorcodecids)
  * [StateActorManifestCID](#stateactormanifestcid)
  * [StateCall](#statecall)
  * [StateCompute](#statecompute)
  * [StateGetBeaconEntry](#stategetbeaconentry)
  * [StateGetNetworkParams](#stategetnetworkparams)
  * [StateGetRandomnessDigestFromBeacon](#stategetrandomnessdigestfrombeacon)
  * [StateGetRandomnessDigestFromTickets](#stategetrandomnessdigestfromtickets)
  * [StateGetRandomnessFromBeacon](#stategetrandomnessfrombeacon)
  * [StateGetRandomnessFromTickets](#stategetrandomnessfromtickets)
  * [StateNetworkName](#statenetworkname)
  * [StateNetworkVersion](#statenetworkversion)
  * [StateReplay](#statereplay)
  * [StateSearchMsg](#statesearchmsg)
  * [StateVerifiedRegistryRootKey](#stateverifiedregistryrootkey)
  * [StateVerifierStatus](#stateverifierstatus)
  * [StateWaitMsg](#statewaitmsg)
  * [VerifyEntry](#verifyentry)
* [Common](#common)
  * [NodeStatus](#nodestatus)
  * [StartTime](#starttime)
  * [Version](#version)
* [ETH](#eth)
  * [EthAccounts](#ethaccounts)
  * [EthAddressToFilecoinAddress](#ethaddresstofilecoinaddress)
  * [EthBlockNumber](#ethblocknumber)
  * [EthCall](#ethcall)
  * [EthChainId](#ethchainid)
  * [EthEstimateGas](#ethestimategas)
  * [EthFeeHistory](#ethfeehistory)
  * [EthGasPrice](#ethgasprice)
  * [EthGetBalance](#ethgetbalance)
  * [EthGetBlockByHash](#ethgetblockbyhash)
  * [EthGetBlockByNumber](#ethgetblockbynumber)
  * [EthGetBlockTransactionCountByHash](#ethgetblocktransactioncountbyhash)
  * [EthGetBlockTransactionCountByNumber](#ethgetblocktransactioncountbynumber)
  * [EthGetCode](#ethgetcode)
  * [EthGetMessageCidByTransactionHash](#ethgetmessagecidbytransactionhash)
  * [EthGetStorageAt](#ethgetstorageat)
  * [EthGetTransactionByBlockHashAndIndex](#ethgettransactionbyblockhashandindex)
  * [EthGetTransactionByBlockNumberAndIndex](#ethgettransactionbyblocknumberandindex)
  * [EthGetTransactionByHash](#ethgettransactionbyhash)
  * [EthGetTransactionByHashLimited](#ethgettransactionbyhashlimited)
  * [EthGetTransactionCount](#ethgettransactioncount)
  * [EthGetTransactionHashByCid](#ethgettransactionhashbycid)
  * [EthGetTransactionReceipt](#ethgettransactionreceipt)
  * [EthGetTransactionReceiptLimited](#ethgettransactionreceiptlimited)
  * [EthMaxPriorityFeePerGas](#ethmaxpriorityfeepergas)
  * [EthProtocolVersion](#ethprotocolversion)
  * [EthSendRawTransaction](#ethsendrawtransaction)
  * [EthSyncing](#ethsyncing)
  * [EthTraceBlock](#ethtraceblock)
  * [EthTraceReplayBlockTransactions](#ethtracereplayblocktransactions)
  * [FilecoinAddressToEthAddress](#filecoinaddresstoethaddress)
  * [NetListening](#netlistening)
  * [NetVersion](#netversion)
  * [Web3ClientVersion](#web3clientversion)
* [ETHEvent](#ethevent)
  * [EthGetFilterChanges](#ethgetfilterchanges)
  * [EthGetFilterLogs](#ethgetfilterlogs)
  * [EthGetLogs](#ethgetlogs)
  * [EthNewBlockFilter](#ethnewblockfilter)
  * [EthNewFilter](#ethnewfilter)
  * [EthNewPendingTransactionFilter](#ethnewpendingtransactionfilter)
  * [EthSubscribe](#ethsubscribe)
  * [EthUninstallFilter](#ethuninstallfilter)
  * [EthUnsubscribe](#ethunsubscribe)
* [Market](#market)
  * [StateMarketParticipants](#statemarketparticipants)
* [MessagePool](#messagepool)
  * [GasBatchEstimateMessageGas](#gasbatchestimatemessagegas)
  * [GasEstimateFeeCap](#gasestimatefeecap)
  * [GasEstimateGasLimit](#gasestimategaslimit)
  * [GasEstimateGasPremium](#gasestimategaspremium)
  * [GasEstimateMessageGas](#gasestimatemessagegas)
  * [MpoolBatchPush](#mpoolbatchpush)
  * [MpoolBatchPushMessage](#mpoolbatchpushmessage)
  * [MpoolBatchPushUntrusted](#mpoolbatchpushuntrusted)
  * [MpoolCheckMessages](#mpoolcheckmessages)
  * [MpoolCheckPendingMessages](#mpoolcheckpendingmessages)
  * [MpoolCheckReplaceMessages](#mpoolcheckreplacemessages)
  * [MpoolClear](#mpoolclear)
  * [MpoolDeleteByAdress](#mpooldeletebyadress)
  * [MpoolGetConfig](#mpoolgetconfig)
  * [MpoolGetNonce](#mpoolgetnonce)
  * [MpoolPending](#mpoolpending)
  * [MpoolPublishByAddr](#mpoolpublishbyaddr)
  * [MpoolPublishMessage](#mpoolpublishmessage)
  * [MpoolPush](#mpoolpush)
  * [MpoolPushMessage](#mpoolpushmessage)
  * [MpoolPushUntrusted](#mpoolpushuntrusted)
  * [MpoolSelect](#mpoolselect)
  * [MpoolSelects](#mpoolselects)
  * [MpoolSetConfig](#mpoolsetconfig)
  * [MpoolSub](#mpoolsub)
* [MinerState](#minerstate)
  * [StateAllMinerFaults](#stateallminerfaults)
  * [StateChangedActors](#statechangedactors)
  * [StateCirculatingSupply](#statecirculatingsupply)
  * [StateComputeDataCID](#statecomputedatacid)
  * [StateDealProviderCollateralBounds](#statedealprovidercollateralbounds)
  * [StateDecodeParams](#statedecodeparams)
  * [StateEncodeParams](#stateencodeparams)
  * [StateGetAllocation](#stategetallocation)
  * [StateGetAllocationForPendingDeal](#stategetallocationforpendingdeal)
  * [StateGetAllocations](#stategetallocations)
  * [StateGetClaim](#stategetclaim)
  * [StateGetClaims](#stategetclaims)
  * [StateListActors](#statelistactors)
  * [StateListMessages](#statelistmessages)
  * [StateListMiners](#statelistminers)
  * [StateLookupID](#statelookupid)
  * [StateLookupRobustAddress](#statelookuprobustaddress)
  * [StateMarketBalance](#statemarketbalance)
  * [StateMarketDeals](#statemarketdeals)
  * [StateMarketStorageDeal](#statemarketstoragedeal)
  * [StateMinerActiveSectors](#statemineractivesectors)
  * [StateMinerAllocated](#stateminerallocated)
  * [StateMinerAvailableBalance](#statemineravailablebalance)
  * [StateMinerDeadlines](#stateminerdeadlines)
  * [StateMinerFaults](#stateminerfaults)
  * [StateMinerInfo](#stateminerinfo)
  * [StateMinerInitialPledgeCollateral](#stateminerinitialpledgecollateral)
  * [StateMinerPartitions](#stateminerpartitions)
  * [StateMinerPower](#stateminerpower)
  * [StateMinerPreCommitDepositForPower](#stateminerprecommitdepositforpower)
  * [StateMinerProvingDeadline](#stateminerprovingdeadline)
  * [StateMinerRecoveries](#stateminerrecoveries)
  * [StateMinerSectorAllocated](#stateminersectorallocated)
  * [StateMinerSectorCount](#stateminersectorcount)
  * [StateMinerSectorSize](#stateminersectorsize)
  * [StateMinerSectors](#stateminersectors)
  * [StateMinerWorkerAddress](#stateminerworkeraddress)
  * [StateReadState](#statereadstate)
  * [StateSectorExpiration](#statesectorexpiration)
  * [StateSectorGetInfo](#statesectorgetinfo)
  * [StateSectorPartition](#statesectorpartition)
  * [StateSectorPreCommitInfo](#statesectorprecommitinfo)
  * [StateVMCirculatingSupplyInternal](#statevmcirculatingsupplyinternal)
  * [StateVerifiedClientStatus](#stateverifiedclientstatus)
* [Mining](#mining)
  * [MinerCreateBlock](#minercreateblock)
  * [MinerGetBaseInfo](#minergetbaseinfo)
* [Network](#network)
  * [ID](#id)
  * [NetAddrsListen](#netaddrslisten)
  * [NetAgentVersion](#netagentversion)
  * [NetAutoNatStatus](#netautonatstatus)
  * [NetBandwidthStats](#netbandwidthstats)
  * [NetBandwidthStatsByPeer](#netbandwidthstatsbypeer)
  * [NetBandwidthStatsByProtocol](#netbandwidthstatsbyprotocol)
  * [NetConnect](#netconnect)
  * [NetConnectedness](#netconnectedness)
  * [NetDisconnect](#netdisconnect)
  * [NetFindPeer](#netfindpeer)
  * [NetFindProvidersAsync](#netfindprovidersasync)
  * [NetGetClosestPeers](#netgetclosestpeers)
  * [NetPeerInfo](#netpeerinfo)
  * [NetPeers](#netpeers)
  * [NetPing](#netping)
  * [NetProtectAdd](#netprotectadd)
  * [NetProtectList](#netprotectlist)
  * [NetProtectRemove](#netprotectremove)
  * [NetPubsubScores](#netpubsubscores)
* [Paychan](#paychan)
  * [PaychAllocateLane](#paychallocatelane)
  * [PaychAvailableFunds](#paychavailablefunds)
  * [PaychAvailableFundsByFromTo](#paychavailablefundsbyfromto)
  * [PaychCollect](#paychcollect)
  * [PaychFund](#paychfund)
  * [PaychGet](#paychget)
  * [PaychGetWaitReady](#paychgetwaitready)
  * [PaychList](#paychlist)
  * [PaychNewPayment](#paychnewpayment)
  * [PaychSettle](#paychsettle)
  * [PaychStatus](#paychstatus)
  * [PaychVoucherAdd](#paychvoucheradd)
  * [PaychVoucherCheckSpendable](#paychvouchercheckspendable)
  * [PaychVoucherCheckValid](#paychvouchercheckvalid)
  * [PaychVoucherCreate](#paychvouchercreate)
  * [PaychVoucherList](#paychvoucherlist)
  * [PaychVoucherSubmit](#paychvouchersubmit)
* [Syncer](#syncer)
  * [ChainSyncHandleNewTipSet](#chainsynchandlenewtipset)
  * [ChainTipSetWeight](#chaintipsetweight)
  * [Concurrent](#concurrent)
  * [SetConcurrent](#setconcurrent)
  * [SyncIncomingBlocks](#syncincomingblocks)
  * [SyncState](#syncstate)
  * [SyncSubmitBlock](#syncsubmitblock)
  * [SyncerTracker](#syncertracker)
* [Wallet](#wallet)
  * [HasPassword](#haspassword)
  * [LockWallet](#lockwallet)
  * [SetPassword](#setpassword)
  * [UnLockWallet](#unlockwallet)
  * [WalletAddresses](#walletaddresses)
  * [WalletBalance](#walletbalance)
  * [WalletDefaultAddress](#walletdefaultaddress)
  * [WalletDelete](#walletdelete)
  * [WalletExport](#walletexport)
  * [WalletHas](#wallethas)
  * [WalletImport](#walletimport)
  * [WalletNewAddress](#walletnewaddress)
  * [WalletSetDefault](#walletsetdefault)
  * [WalletSign](#walletsign)
  * [WalletSignMessage](#walletsignmessage)
  * [WalletState](#walletstate)

## Account

### StateAccountKey


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `"f01234"`

## Actor

### ListActor


Perms: read

Inputs: `[]`

Response: `{}`

### StateGetActor


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "Code": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Head": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Nonce": 42,
  "Balance": "0",
  "Address": "f01234"
}
```

## BlockStore

### ChainDeleteObj


Perms: admin

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

Response: `{}`

### ChainHasObj


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

Response: `true`

### ChainPutObj
ChainPutObj puts a given object into the block store


Perms: admin

Inputs:
```json
[
  {}
]
```

Response: `{}`

### ChainReadObj


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

Response: `"Ynl0ZSBhcnJheQ=="`

### ChainStatObj


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

Response:
```json
{
  "Size": 42,
  "Links": 42
}
```

## ChainInfo

### BlockTime


Perms: read

Inputs: `[]`

Response: `60000000000`

### ChainExport


Perms: read

Inputs:
```json
[
  10101,
  true,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `"Ynl0ZSBhcnJheQ=="`

### ChainGetBlock


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

Response:
```json
{
  "Miner": "f01234",
  "Ticket": {
    "VRFProof": "Bw=="
  },
  "ElectionProof": {
    "WinCount": 9,
    "VRFProof": "Bw=="
  },
  "BeaconEntries": [
    {
      "Round": 42,
      "Data": "Ynl0ZSBhcnJheQ=="
    }
  ],
  "WinPoStProof": [
    {
      "PoStProof": 8,
      "ProofBytes": "Ynl0ZSBhcnJheQ=="
    }
  ],
  "Parents": [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
  ],
  "ParentWeight": "0",
  "Height": 10101,
  "ParentStateRoot": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "ParentMessageReceipts": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Messages": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "BLSAggregate": {
    "Type": 2,
    "Data": "Ynl0ZSBhcnJheQ=="
  },
  "Timestamp": 42,
  "BlockSig": {
    "Type": 2,
    "Data": "Ynl0ZSBhcnJheQ=="
  },
  "ForkSignaling": 42,
  "ParentBaseFee": "0"
}
```

### ChainGetBlockMessages


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

Response:
```json
{
  "BlsMessages": [
    {
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 9,
      "GasFeeCap": "0",
      "GasPremium": "0",
      "Method": 1,
      "Params": "Ynl0ZSBhcnJheQ=="
    }
  ],
  "SecpkMessages": [
    {
      "Message": {
        "CID": {
          "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
        },
        "Version": 42,
        "To": "f01234",
        "From": "f01234",
        "Nonce": 42,
        "Value": "0",
        "GasLimit": 9,
        "GasFeeCap": "0",
        "GasPremium": "0",
        "Method": 1,
        "Params": "Ynl0ZSBhcnJheQ=="
      },
      "Signature": {
        "Type": 2,
        "Data": "Ynl0ZSBhcnJheQ=="
      },
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      }
    }
  ],
  "Cids": [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
  ]
}
```

### ChainGetEvents
ChainGetEvents returns the events under an event AMT root CID.


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

Response:
```json
[
  {
    "Emitter": 1000,
    "Entries": [
      {
        "Flags": 7,
        "Key": "string value",
        "Codec": 42,
        "Value": "Ynl0ZSBhcnJheQ=="
      }
    ]
  }
]
```

### ChainGetGenesis
ChainGetGenesis returns the genesis tipset.


Perms: read

Inputs: `[]`

Response:
```json
{
  "Cids": null,
  "Blocks": null,
  "Height": 0
}
```

### ChainGetMessage


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

Response:
```json
{
  "CID": {
    "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
  },
  "Version": 42,
  "To": "f01234",
  "From": "f01234",
  "Nonce": 42,
  "Value": "0",
  "GasLimit": 9,
  "GasFeeCap": "0",
  "GasPremium": "0",
  "Method": 1,
  "Params": "Ynl0ZSBhcnJheQ=="
}
```

### ChainGetMessagesInTipset


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
[
  {
    "Cid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "Message": {
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 9,
      "GasFeeCap": "0",
      "GasPremium": "0",
      "Method": 1,
      "Params": "Ynl0ZSBhcnJheQ=="
    }
  }
]
```

### ChainGetParentMessages


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

Response:
```json
[
  {
    "Cid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "Message": {
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 9,
      "GasFeeCap": "0",
      "GasPremium": "0",
      "Method": 1,
      "Params": "Ynl0ZSBhcnJheQ=="
    }
  }
]
```

### ChainGetParentReceipts


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

Response:
```json
[
  {
    "ExitCode": 0,
    "Return": "Ynl0ZSBhcnJheQ==",
    "GasUsed": 9,
    "EventsRoot": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
  }
]
```

### ChainGetPath


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ],
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
[
  {
    "Type": "apply",
    "Val": {
      "Cids": null,
      "Blocks": null,
      "Height": 0
    }
  }
]
```

### ChainGetReceipts


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

Response:
```json
[
  {
    "ExitCode": 0,
    "Return": "Ynl0ZSBhcnJheQ==",
    "GasUsed": 9,
    "EventsRoot": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
  }
]
```

### ChainGetTipSet


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "Cids": null,
  "Blocks": null,
  "Height": 0
}
```

### ChainGetTipSetAfterHeight


Perms: read

Inputs:
```json
[
  10101,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "Cids": null,
  "Blocks": null,
  "Height": 0
}
```

### ChainGetTipSetByHeight


Perms: read

Inputs:
```json
[
  10101,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "Cids": null,
  "Blocks": null,
  "Height": 0
}
```

### ChainHead


Perms: read

Inputs: `[]`

Response:
```json
{
  "Cids": null,
  "Blocks": null,
  "Height": 0
}
```

### ChainList


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ],
  123
]
```

Response:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

### ChainNotify


Perms: read

Inputs: `[]`

Response:
```json
[
  {
    "Type": "apply",
    "Val": {
      "Cids": null,
      "Blocks": null,
      "Height": 0
    }
  }
]
```

### ChainSetHead


Perms: admin

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `{}`

### GetActor


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response:
```json
{
  "Code": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Head": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Nonce": 42,
  "Balance": "0",
  "Address": "f01234"
}
```

### GetEntry


Perms: read

Inputs:
```json
[
  10101,
  42
]
```

Response:
```json
{
  "Round": 42,
  "Data": "Ynl0ZSBhcnJheQ=="
}
```

### GetFullBlock


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

Response:
```json
{
  "Header": {
    "Miner": "f01234",
    "Ticket": {
      "VRFProof": "Bw=="
    },
    "ElectionProof": {
      "WinCount": 9,
      "VRFProof": "Bw=="
    },
    "BeaconEntries": [
      {
        "Round": 42,
        "Data": "Ynl0ZSBhcnJheQ=="
      }
    ],
    "WinPoStProof": [
      {
        "PoStProof": 8,
        "ProofBytes": "Ynl0ZSBhcnJheQ=="
      }
    ],
    "Parents": [
      {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      }
    ],
    "ParentWeight": "0",
    "Height": 10101,
    "ParentStateRoot": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "ParentMessageReceipts": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "Messages": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "BLSAggregate": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "Timestamp": 42,
    "BlockSig": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "ForkSignaling": 42,
    "ParentBaseFee": "0"
  },
  "BLSMessages": [
    {
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 9,
      "GasFeeCap": "0",
      "GasPremium": "0",
      "Method": 1,
      "Params": "Ynl0ZSBhcnJheQ=="
    }
  ],
  "SECPMessages": [
    {
      "Message": {
        "CID": {
          "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
        },
        "Version": 42,
        "To": "f01234",
        "From": "f01234",
        "Nonce": 42,
        "Value": "0",
        "GasLimit": 9,
        "GasFeeCap": "0",
        "GasPremium": "0",
        "Method": 1,
        "Params": "Ynl0ZSBhcnJheQ=="
      },
      "Signature": {
        "Type": 2,
        "Data": "Ynl0ZSBhcnJheQ=="
      },
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      }
    }
  ]
}
```

### GetParentStateRootActor


Perms: read

Inputs:
```json
[
  {
    "Cids": null,
    "Blocks": null,
    "Height": 0
  },
  "f01234"
]
```

Response:
```json
{
  "Code": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Head": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Nonce": 42,
  "Balance": "0",
  "Address": "f01234"
}
```

### ProtocolParameters


Perms: read

Inputs: `[]`

Response:
```json
{
  "Network": "string value",
  "BlockTime": 60000000000,
  "SupportedSectors": [
    {
      "Size": 34359738368,
      "MaxPieceSize": 1024
    }
  ]
}
```

### ResolveToKeyAddr


Perms: read

Inputs:
```json
[
  "f01234",
  {
    "Cids": null,
    "Blocks": null,
    "Height": 0
  }
]
```

Response: `"f01234"`

### StateActorCodeCIDs
StateActorCodeCIDs returns the CIDs of all the builtin actors for the given network version


Perms: read

Inputs:
```json
[
  21
]
```

Response: `{}`

### StateActorManifestCID
StateActorManifestCID returns the CID of the builtin actors manifest for the given network version


Perms: read

Inputs:
```json
[
  21
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### StateCall


Perms: read

Inputs:
```json
[
  {
    "CID": {
      "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 9,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ=="
  },
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "MsgCid": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Msg": {
    "CID": {
      "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 9,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ=="
  },
  "MsgRct": {
    "ExitCode": 0,
    "Return": "Ynl0ZSBhcnJheQ==",
    "GasUsed": 9,
    "EventsRoot": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
  },
  "GasCost": {
    "Message": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "GasUsed": "0",
    "BaseFeeBurn": "0",
    "OverEstimationBurn": "0",
    "MinerPenalty": "0",
    "MinerTip": "0",
    "Refund": "0",
    "TotalCost": "0"
  },
  "ExecutionTrace": {
    "Msg": {
      "From": "f01234",
      "To": "f01234",
      "Value": "0",
      "Method": 1,
      "Params": "Ynl0ZSBhcnJheQ==",
      "ParamsCodec": 42,
      "GasLimit": 42,
      "ReadOnly": true,
      "CodeCid": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      }
    },
    "MsgRct": {
      "ExitCode": 0,
      "Return": "Ynl0ZSBhcnJheQ==",
      "ReturnCodec": 42
    },
    "GasCharges": [
      {
        "Name": "string value",
        "tg": 9,
        "cg": 9,
        "sg": 9,
        "tt": 60000000000
      }
    ],
    "Subcalls": [
      {
        "Msg": {
          "From": "f01234",
          "To": "f01234",
          "Value": "0",
          "Method": 1,
          "Params": "Ynl0ZSBhcnJheQ==",
          "ParamsCodec": 42,
          "GasLimit": 42,
          "ReadOnly": true,
          "CodeCid": {
            "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
          }
        },
        "MsgRct": {
          "ExitCode": 0,
          "Return": "Ynl0ZSBhcnJheQ==",
          "ReturnCodec": 42
        },
        "GasCharges": [
          {
            "Name": "string value",
            "tg": 9,
            "cg": 9,
            "sg": 9,
            "tt": 60000000000
          }
        ],
        "Subcalls": null
      }
    ]
  },
  "Error": "string value",
  "Duration": 60000000000
}
```

### StateCompute
StateCompute is a flexible command that applies the given messages on the given tipset.
The messages are run as though the VM were at the provided height.

When called, StateCompute will:
- Load the provided tipset, or use the current chain head if not provided
- Compute the tipset state of the provided tipset on top of the parent state
- (note that this step runs before vmheight is applied to the execution)
- Execute state upgrade if any were scheduled at the epoch, or in null
blocks preceding the tipset
- Call the cron actor on null blocks preceding the tipset
- For each block in the tipset
- Apply messages in blocks in the specified
- Award block reward by calling the reward actor
- Call the cron actor for the current epoch
- If the specified vmheight is higher than the current epoch, apply any
needed state upgrades to the state
- Apply the specified messages to the state

The vmheight parameter sets VM execution epoch, and can be used to simulate
message execution in different network versions. If the specified vmheight
epoch is higher than the epoch of the specified tipset, any state upgrades
until the vmheight will be executed on the state before applying messages
specified by the user.

Note that the initial tipset state computation is not affected by the
vmheight parameter - only the messages in the `apply` set are

If the caller wants to simply compute the state, vmheight should be set to
the epoch of the specified tipset.

Messages in the `apply` parameter must have the correct nonces, and gas
values set.


Perms: read

Inputs:
```json
[
  10101,
  [
    {
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 9,
      "GasFeeCap": "0",
      "GasPremium": "0",
      "Method": 1,
      "Params": "Ynl0ZSBhcnJheQ=="
    }
  ],
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "Root": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Trace": [
    {
      "MsgCid": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      "Msg": {
        "CID": {
          "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
        },
        "Version": 42,
        "To": "f01234",
        "From": "f01234",
        "Nonce": 42,
        "Value": "0",
        "GasLimit": 9,
        "GasFeeCap": "0",
        "GasPremium": "0",
        "Method": 1,
        "Params": "Ynl0ZSBhcnJheQ=="
      },
      "MsgRct": {
        "ExitCode": 0,
        "Return": "Ynl0ZSBhcnJheQ==",
        "GasUsed": 9,
        "EventsRoot": {
          "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
        }
      },
      "GasCost": {
        "Message": {
          "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
        },
        "GasUsed": "0",
        "BaseFeeBurn": "0",
        "OverEstimationBurn": "0",
        "MinerPenalty": "0",
        "MinerTip": "0",
        "Refund": "0",
        "TotalCost": "0"
      },
      "ExecutionTrace": {
        "Msg": {
          "From": "f01234",
          "To": "f01234",
          "Value": "0",
          "Method": 1,
          "Params": "Ynl0ZSBhcnJheQ==",
          "ParamsCodec": 42,
          "GasLimit": 42,
          "ReadOnly": true,
          "CodeCid": {
            "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
          }
        },
        "MsgRct": {
          "ExitCode": 0,
          "Return": "Ynl0ZSBhcnJheQ==",
          "ReturnCodec": 42
        },
        "GasCharges": [
          {
            "Name": "string value",
            "tg": 9,
            "cg": 9,
            "sg": 9,
            "tt": 60000000000
          }
        ],
        "Subcalls": [
          {
            "Msg": {
              "From": "f01234",
              "To": "f01234",
              "Value": "0",
              "Method": 1,
              "Params": "Ynl0ZSBhcnJheQ==",
              "ParamsCodec": 42,
              "GasLimit": 42,
              "ReadOnly": true,
              "CodeCid": {
                "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
              }
            },
            "MsgRct": {
              "ExitCode": 0,
              "Return": "Ynl0ZSBhcnJheQ==",
              "ReturnCodec": 42
            },
            "GasCharges": [
              {
                "Name": "string value",
                "tg": 9,
                "cg": 9,
                "sg": 9,
                "tt": 60000000000
              }
            ],
            "Subcalls": null
          }
        ]
      },
      "Error": "string value",
      "Duration": 60000000000
    }
  ]
}
```

### StateGetBeaconEntry
StateGetBeaconEntry returns the beacon entry for the given filecoin epoch. If
the entry has not yet been produced, the call will block until the entry
becomes available


Perms: read

Inputs:
```json
[
  10101
]
```

Response:
```json
{
  "Round": 42,
  "Data": "Ynl0ZSBhcnJheQ=="
}
```

### StateGetNetworkParams
StateGetNetworkParams return current network params


Perms: read

Inputs: `[]`

Response:
```json
{
  "NetworkName": "mainnet",
  "BlockDelaySecs": 42,
  "ConsensusMinerMinPower": "0",
  "SupportedProofTypes": [
    8
  ],
  "PreCommitChallengeDelay": 10101,
  "ForkUpgradeParams": {
    "UpgradeSmokeHeight": 10101,
    "UpgradeBreezeHeight": 10101,
    "UpgradeIgnitionHeight": 10101,
    "UpgradeLiftoffHeight": 10101,
    "UpgradeAssemblyHeight": 10101,
    "UpgradeRefuelHeight": 10101,
    "UpgradeTapeHeight": 10101,
    "UpgradeKumquatHeight": 10101,
    "BreezeGasTampingDuration": 10101,
    "UpgradeCalicoHeight": 10101,
    "UpgradePersianHeight": 10101,
    "UpgradeOrangeHeight": 10101,
    "UpgradeClausHeight": 10101,
    "UpgradeTrustHeight": 10101,
    "UpgradeNorwegianHeight": 10101,
    "UpgradeTurboHeight": 10101,
    "UpgradeHyperdriveHeight": 10101,
    "UpgradeChocolateHeight": 10101,
    "UpgradeOhSnapHeight": 10101,
    "UpgradeSkyrHeight": 10101,
    "UpgradeSharkHeight": 10101,
    "UpgradeHyggeHeight": 10101,
    "UpgradeLightningHeight": 10101,
    "UpgradeThunderHeight": 10101,
    "UpgradeWatermelonHeight": 10101
  },
  "Eip155ChainID": 123
}
```

### StateGetRandomnessDigestFromBeacon
StateGetRandomnessDigestFromBeacon is used to sample the beacon for randomness.


Perms: read

Inputs:
```json
[
  10101,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `"Bw=="`

### StateGetRandomnessDigestFromTickets
StateGetRandomnessDigestFromTickets is used to sample the chain for randomness.


Perms: read

Inputs:
```json
[
  10101,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `"Bw=="`

### StateGetRandomnessFromBeacon


Perms: read

Inputs:
```json
[
  2,
  10101,
  "Ynl0ZSBhcnJheQ==",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `"Bw=="`

### StateGetRandomnessFromTickets


Perms: read

Inputs:
```json
[
  2,
  10101,
  "Ynl0ZSBhcnJheQ==",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `"Bw=="`

### StateNetworkName


Perms: read

Inputs: `[]`

Response: `"mainnet"`

### StateNetworkVersion


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `21`

### StateReplay


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ],
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

Response:
```json
{
  "MsgCid": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Msg": {
    "CID": {
      "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 9,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ=="
  },
  "MsgRct": {
    "ExitCode": 0,
    "Return": "Ynl0ZSBhcnJheQ==",
    "GasUsed": 9,
    "EventsRoot": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
  },
  "GasCost": {
    "Message": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "GasUsed": "0",
    "BaseFeeBurn": "0",
    "OverEstimationBurn": "0",
    "MinerPenalty": "0",
    "MinerTip": "0",
    "Refund": "0",
    "TotalCost": "0"
  },
  "ExecutionTrace": {
    "Msg": {
      "From": "f01234",
      "To": "f01234",
      "Value": "0",
      "Method": 1,
      "Params": "Ynl0ZSBhcnJheQ==",
      "ParamsCodec": 42,
      "GasLimit": 42,
      "ReadOnly": true,
      "CodeCid": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      }
    },
    "MsgRct": {
      "ExitCode": 0,
      "Return": "Ynl0ZSBhcnJheQ==",
      "ReturnCodec": 42
    },
    "GasCharges": [
      {
        "Name": "string value",
        "tg": 9,
        "cg": 9,
        "sg": 9,
        "tt": 60000000000
      }
    ],
    "Subcalls": [
      {
        "Msg": {
          "From": "f01234",
          "To": "f01234",
          "Value": "0",
          "Method": 1,
          "Params": "Ynl0ZSBhcnJheQ==",
          "ParamsCodec": 42,
          "GasLimit": 42,
          "ReadOnly": true,
          "CodeCid": {
            "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
          }
        },
        "MsgRct": {
          "ExitCode": 0,
          "Return": "Ynl0ZSBhcnJheQ==",
          "ReturnCodec": 42
        },
        "GasCharges": [
          {
            "Name": "string value",
            "tg": 9,
            "cg": 9,
            "sg": 9,
            "tt": 60000000000
          }
        ],
        "Subcalls": null
      }
    ]
  },
  "Error": "string value",
  "Duration": 60000000000
}
```

### StateSearchMsg
StateSearchMsg looks back up to limit epochs in the chain for a message, and returns its receipt and the tipset where it was executed

NOTE: If a replacing message is found on chain, this method will return
a MsgLookup for the replacing message - the MsgLookup.Message will be a different
CID than the one provided in the 'cid' param, MsgLookup.Receipt will contain the
result of the execution of the replacing message.

If the caller wants to ensure that exactly the requested message was executed,
they must check that MsgLookup.Message is equal to the provided 'cid', or set the
`allowReplaced` parameter to false. Without this check, and with `allowReplaced`
set to true, both the requested and original message may appear as
successfully executed on-chain, which may look like a double-spend.

A replacing message is a message with a different CID, any of Gas values, and
different signature, but with all other parameters matching (source/destination,
nonce, params, etc.)


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ],
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  10101,
  true
]
```

Response:
```json
{
  "Message": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Receipt": {
    "ExitCode": 0,
    "Return": "Ynl0ZSBhcnJheQ==",
    "GasUsed": 9,
    "EventsRoot": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
  },
  "ReturnDec": {},
  "TipSet": [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ],
  "Height": 10101
}
```

### StateVerifiedRegistryRootKey


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `"f01234"`

### StateVerifierStatus


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `"0"`

### StateWaitMsg
StateWaitMsg looks back up to limit epochs in the chain for a message.
If not found, it blocks until the message arrives on chain, and gets to the
indicated confidence depth.

NOTE: If a replacing message is found on chain, this method will return
a MsgLookup for the replacing message - the MsgLookup.Message will be a different
CID than the one provided in the 'cid' param, MsgLookup.Receipt will contain the
result of the execution of the replacing message.

If the caller wants to ensure that exactly the requested message was executed,
they must check that MsgLookup.Message is equal to the provided 'cid', or set the
`allowReplaced` parameter to false. Without this check, and with `allowReplaced`
set to true, both the requested and original message may appear as
successfully executed on-chain, which may look like a double-spend.

A replacing message is a message with a different CID, any of Gas values, and
different signature, but with all other parameters matching (source/destination,
nonce, params, etc.)


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  42,
  10101,
  true
]
```

Response:
```json
{
  "Message": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Receipt": {
    "ExitCode": 0,
    "Return": "Ynl0ZSBhcnJheQ==",
    "GasUsed": 9,
    "EventsRoot": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
  },
  "ReturnDec": {},
  "TipSet": [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ],
  "Height": 10101
}
```

### VerifyEntry


Perms: read

Inputs:
```json
[
  {
    "Round": 42,
    "Data": "Ynl0ZSBhcnJheQ=="
  },
  {
    "Round": 42,
    "Data": "Ynl0ZSBhcnJheQ=="
  },
  10101
]
```

Response: `true`

## Common

### NodeStatus


Perms: read

Inputs:
```json
[
  true
]
```

Response:
```json
{
  "SyncStatus": {
    "Epoch": 42,
    "Behind": 42
  },
  "PeerStatus": {
    "PeersToPublishMsgs": 123,
    "PeersToPublishBlocks": 123
  },
  "ChainStatus": {
    "BlocksPerTipsetLast100": 12.3,
    "BlocksPerTipsetLastFinality": 12.3
  }
}
```

### StartTime
StartTime returns node start time


Perms: read

Inputs: `[]`

Response: `"0001-01-01T00:00:00Z"`

### Version
Version provides information about API provider


Perms: read

Inputs: `[]`

Response:
```json
{
  "Version": "string value",
  "APIVersion": 131840
}
```

## ETH

### EthAccounts
These methods are used for Ethereum-compatible JSON-RPC calls

EthAccounts will always return [] since we don't expect Lotus to manage private keys


Perms: read

Inputs: `[]`

Response:
```json
[
  "0x0707070707070707070707070707070707070707"
]
```

### EthAddressToFilecoinAddress
EthAddressToFilecoinAddress converts an EthAddress into an f410 Filecoin Address


Perms: read

Inputs:
```json
[
  "0x0707070707070707070707070707070707070707"
]
```

Response: `"f01234"`

### EthBlockNumber
EthBlockNumber returns the height of the latest (heaviest) TipSet


Perms: read

Inputs: `[]`

Response: `"0x5"`

### EthCall


Perms: read

Inputs:
```json
[
  {
    "from": "0x5cbeecf99d3fdb3f25e309cc264f240bb0664031",
    "to": "0x5cbeecf99d3fdb3f25e309cc264f240bb0664031",
    "gas": "0x5",
    "gasPrice": "0x0",
    "value": "0x0",
    "data": "0x07"
  },
  {
    "blockNumber": "0x5",
    "blockHash": "0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e",
    "requireCanonical": true
  }
]
```

Response: `"0x07"`

### EthChainId


Perms: read

Inputs: `[]`

Response: `"0x5"`

### EthEstimateGas


Perms: read

Inputs:
```json
[
  {
    "from": "0x5cbeecf99d3fdb3f25e309cc264f240bb0664031",
    "to": "0x5cbeecf99d3fdb3f25e309cc264f240bb0664031",
    "gas": "0x5",
    "gasPrice": "0x0",
    "value": "0x0",
    "data": "0x07"
  }
]
```

Response: `"0x5"`

### EthFeeHistory


Perms: read

Inputs:
```json
[
  "Bw=="
]
```

Response:
```json
{
  "oldestBlock": "0x5",
  "baseFeePerGas": [
    "0x0"
  ],
  "gasUsedRatio": [
    12.3
  ],
  "reward": []
}
```

### EthGasPrice


Perms: read

Inputs: `[]`

Response: `"0x0"`

### EthGetBalance


Perms: read

Inputs:
```json
[
  "0x0707070707070707070707070707070707070707",
  {
    "blockNumber": "0x5",
    "blockHash": "0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e",
    "requireCanonical": true
  }
]
```

Response: `"0x0"`

### EthGetBlockByHash


Perms: read

Inputs:
```json
[
  "0x0707070707070707070707070707070707070707070707070707070707070707",
  true
]
```

Response:
```json
{
  "hash": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "parentHash": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "sha3Uncles": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "miner": "0x0707070707070707070707070707070707070707",
  "stateRoot": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "transactionsRoot": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "receiptsRoot": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "logsBloom": "0x07",
  "difficulty": "0x5",
  "totalDifficulty": "0x5",
  "number": "0x5",
  "gasLimit": "0x5",
  "gasUsed": "0x5",
  "timestamp": "0x5",
  "extraData": "0x07",
  "mixHash": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "nonce": "0x0707070707070707",
  "baseFeePerGas": "0x0",
  "size": "0x5",
  "transactions": [
    {}
  ],
  "uncles": [
    "0x0707070707070707070707070707070707070707070707070707070707070707"
  ]
}
```

### EthGetBlockByNumber


Perms: read

Inputs:
```json
[
  "string value",
  true
]
```

Response:
```json
{
  "hash": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "parentHash": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "sha3Uncles": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "miner": "0x0707070707070707070707070707070707070707",
  "stateRoot": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "transactionsRoot": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "receiptsRoot": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "logsBloom": "0x07",
  "difficulty": "0x5",
  "totalDifficulty": "0x5",
  "number": "0x5",
  "gasLimit": "0x5",
  "gasUsed": "0x5",
  "timestamp": "0x5",
  "extraData": "0x07",
  "mixHash": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "nonce": "0x0707070707070707",
  "baseFeePerGas": "0x0",
  "size": "0x5",
  "transactions": [
    {}
  ],
  "uncles": [
    "0x0707070707070707070707070707070707070707070707070707070707070707"
  ]
}
```

### EthGetBlockTransactionCountByHash
EthGetBlockTransactionCountByHash returns the number of messages in the TipSet


Perms: read

Inputs:
```json
[
  "0x0707070707070707070707070707070707070707070707070707070707070707"
]
```

Response: `"0x5"`

### EthGetBlockTransactionCountByNumber
EthGetBlockTransactionCountByNumber returns the number of messages in the TipSet


Perms: read

Inputs:
```json
[
  "0x5"
]
```

Response: `"0x5"`

### EthGetCode


Perms: read

Inputs:
```json
[
  "0x0707070707070707070707070707070707070707",
  {
    "blockNumber": "0x5",
    "blockHash": "0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e",
    "requireCanonical": true
  }
]
```

Response: `"0x07"`

### EthGetMessageCidByTransactionHash


Perms: read

Inputs:
```json
[
  "0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e"
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### EthGetStorageAt


Perms: read

Inputs:
```json
[
  "0x0707070707070707070707070707070707070707",
  "0x07",
  {
    "blockNumber": "0x5",
    "blockHash": "0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e",
    "requireCanonical": true
  }
]
```

Response: `"0x07"`

### EthGetTransactionByBlockHashAndIndex


Perms: read

Inputs:
```json
[
  "0x0707070707070707070707070707070707070707070707070707070707070707",
  "0x5"
]
```

Response:
```json
{
  "chainId": "0x5",
  "nonce": "0x5",
  "hash": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "blockHash": "0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e",
  "blockNumber": "0x5",
  "transactionIndex": "0x5",
  "from": "0x0707070707070707070707070707070707070707",
  "to": "0x5cbeecf99d3fdb3f25e309cc264f240bb0664031",
  "value": "0x0",
  "type": "0x5",
  "input": "0x07",
  "gas": "0x5",
  "maxFeePerGas": "0x0",
  "maxPriorityFeePerGas": "0x0",
  "accessList": [
    "0x0707070707070707070707070707070707070707070707070707070707070707"
  ],
  "v": "0x0",
  "r": "0x0",
  "s": "0x0"
}
```

### EthGetTransactionByBlockNumberAndIndex


Perms: read

Inputs:
```json
[
  "0x5",
  "0x5"
]
```

Response:
```json
{
  "chainId": "0x5",
  "nonce": "0x5",
  "hash": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "blockHash": "0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e",
  "blockNumber": "0x5",
  "transactionIndex": "0x5",
  "from": "0x0707070707070707070707070707070707070707",
  "to": "0x5cbeecf99d3fdb3f25e309cc264f240bb0664031",
  "value": "0x0",
  "type": "0x5",
  "input": "0x07",
  "gas": "0x5",
  "maxFeePerGas": "0x0",
  "maxPriorityFeePerGas": "0x0",
  "accessList": [
    "0x0707070707070707070707070707070707070707070707070707070707070707"
  ],
  "v": "0x0",
  "r": "0x0",
  "s": "0x0"
}
```

### EthGetTransactionByHash


Perms: read

Inputs:
```json
[
  "0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e"
]
```

Response:
```json
{
  "chainId": "0x5",
  "nonce": "0x5",
  "hash": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "blockHash": "0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e",
  "blockNumber": "0x5",
  "transactionIndex": "0x5",
  "from": "0x0707070707070707070707070707070707070707",
  "to": "0x5cbeecf99d3fdb3f25e309cc264f240bb0664031",
  "value": "0x0",
  "type": "0x5",
  "input": "0x07",
  "gas": "0x5",
  "maxFeePerGas": "0x0",
  "maxPriorityFeePerGas": "0x0",
  "accessList": [
    "0x0707070707070707070707070707070707070707070707070707070707070707"
  ],
  "v": "0x0",
  "r": "0x0",
  "s": "0x0"
}
```

### EthGetTransactionByHashLimited


Perms: read

Inputs:
```json
[
  "0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e",
  10101
]
```

Response:
```json
{
  "chainId": "0x5",
  "nonce": "0x5",
  "hash": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "blockHash": "0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e",
  "blockNumber": "0x5",
  "transactionIndex": "0x5",
  "from": "0x0707070707070707070707070707070707070707",
  "to": "0x5cbeecf99d3fdb3f25e309cc264f240bb0664031",
  "value": "0x0",
  "type": "0x5",
  "input": "0x07",
  "gas": "0x5",
  "maxFeePerGas": "0x0",
  "maxPriorityFeePerGas": "0x0",
  "accessList": [
    "0x0707070707070707070707070707070707070707070707070707070707070707"
  ],
  "v": "0x0",
  "r": "0x0",
  "s": "0x0"
}
```

### EthGetTransactionCount


Perms: read

Inputs:
```json
[
  "0x0707070707070707070707070707070707070707",
  {
    "blockNumber": "0x5",
    "blockHash": "0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e",
    "requireCanonical": true
  }
]
```

Response: `"0x5"`

### EthGetTransactionHashByCid


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

Response: `"0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e"`

### EthGetTransactionReceipt


Perms: read

Inputs:
```json
[
  "0x0707070707070707070707070707070707070707070707070707070707070707"
]
```

Response:
```json
{
  "transactionHash": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "transactionIndex": "0x5",
  "blockHash": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "blockNumber": "0x5",
  "from": "0x0707070707070707070707070707070707070707",
  "to": "0x5cbeecf99d3fdb3f25e309cc264f240bb0664031",
  "root": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "status": "0x5",
  "contractAddress": "0x5cbeecf99d3fdb3f25e309cc264f240bb0664031",
  "cumulativeGasUsed": "0x5",
  "gasUsed": "0x5",
  "effectiveGasPrice": "0x0",
  "logsBloom": "0x07",
  "logs": [
    {
      "address": "0x0707070707070707070707070707070707070707",
      "data": "0x07",
      "topics": [
        "0x0707070707070707070707070707070707070707070707070707070707070707"
      ],
      "removed": true,
      "logIndex": "0x5",
      "transactionIndex": "0x5",
      "transactionHash": "0x0707070707070707070707070707070707070707070707070707070707070707",
      "blockHash": "0x0707070707070707070707070707070707070707070707070707070707070707",
      "blockNumber": "0x5"
    }
  ],
  "type": "0x5"
}
```

### EthGetTransactionReceiptLimited


Perms: read

Inputs:
```json
[
  "0x0707070707070707070707070707070707070707070707070707070707070707",
  10101
]
```

Response:
```json
{
  "transactionHash": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "transactionIndex": "0x5",
  "blockHash": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "blockNumber": "0x5",
  "from": "0x0707070707070707070707070707070707070707",
  "to": "0x5cbeecf99d3fdb3f25e309cc264f240bb0664031",
  "root": "0x0707070707070707070707070707070707070707070707070707070707070707",
  "status": "0x5",
  "contractAddress": "0x5cbeecf99d3fdb3f25e309cc264f240bb0664031",
  "cumulativeGasUsed": "0x5",
  "gasUsed": "0x5",
  "effectiveGasPrice": "0x0",
  "logsBloom": "0x07",
  "logs": [
    {
      "address": "0x0707070707070707070707070707070707070707",
      "data": "0x07",
      "topics": [
        "0x0707070707070707070707070707070707070707070707070707070707070707"
      ],
      "removed": true,
      "logIndex": "0x5",
      "transactionIndex": "0x5",
      "transactionHash": "0x0707070707070707070707070707070707070707070707070707070707070707",
      "blockHash": "0x0707070707070707070707070707070707070707070707070707070707070707",
      "blockNumber": "0x5"
    }
  ],
  "type": "0x5"
}
```

### EthMaxPriorityFeePerGas


Perms: read

Inputs: `[]`

Response: `"0x0"`

### EthProtocolVersion


Perms: read

Inputs: `[]`

Response: `"0x5"`

### EthSendRawTransaction


Perms: read

Inputs:
```json
[
  "0x07"
]
```

Response: `"0x0707070707070707070707070707070707070707070707070707070707070707"`

### EthSyncing


Perms: read

Inputs: `[]`

Response: `false`

### EthTraceBlock
TraceAPI related methods

Returns traces created at given block


Perms: read

Inputs:
```json
[
  "string value"
]
```

Response:
```json
[
  {
    "action": {
      "callType": "string value",
      "from": "0x0707070707070707070707070707070707070707",
      "to": "0x0707070707070707070707070707070707070707",
      "gas": "0x5",
      "input": "0x07",
      "value": "0x0"
    },
    "result": {
      "gasUsed": "0x5",
      "output": "0x07"
    },
    "subtraces": 123,
    "traceAddress": [
      123
    ],
    "Type": "string value",
    "blockHash": "0x0707070707070707070707070707070707070707070707070707070707070707",
    "blockNumber": 9,
    "transactionHash": "0x0707070707070707070707070707070707070707070707070707070707070707",
    "transactionPosition": 123
  }
]
```

### EthTraceReplayBlockTransactions
Replays all transactions in a block returning the requested traces for each transaction


Perms: read

Inputs:
```json
[
  "string value",
  [
    "string value"
  ]
]
```

Response:
```json
[
  {
    "output": "0x07",
    "stateDiff": "string value",
    "trace": [
      {
        "action": {
          "callType": "string value",
          "from": "0x0707070707070707070707070707070707070707",
          "to": "0x0707070707070707070707070707070707070707",
          "gas": "0x5",
          "input": "0x07",
          "value": "0x0"
        },
        "result": {
          "gasUsed": "0x5",
          "output": "0x07"
        },
        "subtraces": 123,
        "traceAddress": [
          123
        ],
        "Type": "string value"
      }
    ],
    "transactionHash": "0x0707070707070707070707070707070707070707070707070707070707070707",
    "vmTrace": "string value"
  }
]
```

### FilecoinAddressToEthAddress
FilecoinAddressToEthAddress converts an f410 or f0 Filecoin Address to an EthAddress


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `"0x0707070707070707070707070707070707070707"`

### NetListening


Perms: read

Inputs: `[]`

Response: `true`

### NetVersion


Perms: read

Inputs: `[]`

Response: `"string value"`

### Web3ClientVersion
Returns the client version


Perms: read

Inputs: `[]`

Response: `"string value"`

## ETHEvent

### EthGetFilterChanges
Polling method for a filter, returns event logs which occurred since last poll.
(requires write perm since timestamp of last filter execution will be written)


Perms: read

Inputs:
```json
[
  "0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e"
]
```

Response:
```json
[
  {}
]
```

### EthGetFilterLogs
Returns event logs matching filter with given id.
(requires write perm since timestamp of last filter execution will be written)


Perms: read

Inputs:
```json
[
  "0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e"
]
```

Response:
```json
[
  {}
]
```

### EthGetLogs
Returns event logs matching given filter spec.


Perms: read

Inputs:
```json
[
  {
    "fromBlock": "2301220",
    "address": [
      "0x5cbeecf99d3fdb3f25e309cc264f240bb0664031"
    ],
    "topics": null
  }
]
```

Response:
```json
[
  {}
]
```

### EthNewBlockFilter
Installs a persistent filter to notify when a new block arrives.


Perms: read

Inputs: `[]`

Response: `"0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e"`

### EthNewFilter
Installs a persistent filter based on given filter spec.


Perms: read

Inputs:
```json
[
  {
    "fromBlock": "2301220",
    "address": [
      "0x5cbeecf99d3fdb3f25e309cc264f240bb0664031"
    ],
    "topics": null
  }
]
```

Response: `"0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e"`

### EthNewPendingTransactionFilter
Installs a persistent filter to notify when new messages arrive in the message pool.


Perms: read

Inputs: `[]`

Response: `"0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e"`

### EthSubscribe
Subscribe to different event types using websockets
eventTypes is one or more of:
- newHeads: notify when new blocks arrive.
- pendingTransactions: notify when new messages arrive in the message pool.
- logs: notify new event logs that match a criteria
params contains additional parameters used with the log event type
The client will receive a stream of EthSubscriptionResponse values until EthUnsubscribe is called.


Perms: read

Inputs:
```json
[
  "Bw=="
]
```

Response: `"0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e"`

### EthUninstallFilter
Uninstalls a filter with given id.


Perms: read

Inputs:
```json
[
  "0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e"
]
```

Response: `true`

### EthUnsubscribe
Unsubscribe from a websocket subscription


Perms: read

Inputs:
```json
[
  "0x37690cfec6c1bf4c3b9288c7a5d783e98731e90b0a4c177c2a374c7a9427355e"
]
```

Response: `true`

## Market

### StateMarketParticipants


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "t026363": {
    "Escrow": "0",
    "Locked": "0"
  }
}
```

## MessagePool

### GasBatchEstimateMessageGas


Perms: read

Inputs:
```json
[
  [
    {
      "Msg": {
        "CID": {
          "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
        },
        "Version": 42,
        "To": "f01234",
        "From": "f01234",
        "Nonce": 42,
        "Value": "0",
        "GasLimit": 9,
        "GasFeeCap": "0",
        "GasPremium": "0",
        "Method": 1,
        "Params": "Ynl0ZSBhcnJheQ=="
      },
      "Spec": {
        "MaxFee": "0",
        "GasOverEstimation": 12.3,
        "GasOverPremium": 12.3
      }
    }
  ],
  42,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
[
  {
    "Msg": {
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 9,
      "GasFeeCap": "0",
      "GasPremium": "0",
      "Method": 1,
      "Params": "Ynl0ZSBhcnJheQ=="
    },
    "Err": "string value"
  }
]
```

### GasEstimateFeeCap


Perms: read

Inputs:
```json
[
  {
    "CID": {
      "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 9,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ=="
  },
  9,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `"0"`

### GasEstimateGasLimit


Perms: read

Inputs:
```json
[
  {
    "CID": {
      "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 9,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ=="
  },
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `9`

### GasEstimateGasPremium


Perms: read

Inputs:
```json
[
  42,
  "f01234",
  9,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `"0"`

### GasEstimateMessageGas


Perms: read

Inputs:
```json
[
  {
    "CID": {
      "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 9,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ=="
  },
  {
    "MaxFee": "0",
    "GasOverEstimation": 12.3,
    "GasOverPremium": 12.3
  },
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "CID": {
    "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
  },
  "Version": 42,
  "To": "f01234",
  "From": "f01234",
  "Nonce": 42,
  "Value": "0",
  "GasLimit": 9,
  "GasFeeCap": "0",
  "GasPremium": "0",
  "Method": 1,
  "Params": "Ynl0ZSBhcnJheQ=="
}
```

### MpoolBatchPush


Perms: write

Inputs:
```json
[
  [
    {
      "Message": {
        "CID": {
          "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
        },
        "Version": 42,
        "To": "f01234",
        "From": "f01234",
        "Nonce": 42,
        "Value": "0",
        "GasLimit": 9,
        "GasFeeCap": "0",
        "GasPremium": "0",
        "Method": 1,
        "Params": "Ynl0ZSBhcnJheQ=="
      },
      "Signature": {
        "Type": 2,
        "Data": "Ynl0ZSBhcnJheQ=="
      },
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      }
    }
  ]
]
```

Response:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

### MpoolBatchPushMessage


Perms: sign

Inputs:
```json
[
  [
    {
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 9,
      "GasFeeCap": "0",
      "GasPremium": "0",
      "Method": 1,
      "Params": "Ynl0ZSBhcnJheQ=="
    }
  ],
  {
    "MaxFee": "0",
    "GasOverEstimation": 12.3,
    "GasOverPremium": 12.3
  }
]
```

Response:
```json
[
  {
    "Message": {
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 9,
      "GasFeeCap": "0",
      "GasPremium": "0",
      "Method": 1,
      "Params": "Ynl0ZSBhcnJheQ=="
    },
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "CID": {
      "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
    }
  }
]
```

### MpoolBatchPushUntrusted


Perms: write

Inputs:
```json
[
  [
    {
      "Message": {
        "CID": {
          "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
        },
        "Version": 42,
        "To": "f01234",
        "From": "f01234",
        "Nonce": 42,
        "Value": "0",
        "GasLimit": 9,
        "GasFeeCap": "0",
        "GasPremium": "0",
        "Method": 1,
        "Params": "Ynl0ZSBhcnJheQ=="
      },
      "Signature": {
        "Type": 2,
        "Data": "Ynl0ZSBhcnJheQ=="
      },
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      }
    }
  ]
]
```

Response:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

### MpoolCheckMessages
MpoolCheckMessages performs logical checks on a batch of messages


Perms: read

Inputs:
```json
[
  [
    {
      "Message": {
        "CID": {
          "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
        },
        "Version": 42,
        "To": "f01234",
        "From": "f01234",
        "Nonce": 42,
        "Value": "0",
        "GasLimit": 9,
        "GasFeeCap": "0",
        "GasPremium": "0",
        "Method": 1,
        "Params": "Ynl0ZSBhcnJheQ=="
      },
      "ValidNonce": true
    }
  ]
]
```

Response:
```json
[
  [
    {
      "Cid": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      "Code": 0,
      "OK": true,
      "Err": "string value",
      "Hint": {
        "abc": 123
      }
    }
  ]
]
```

### MpoolCheckPendingMessages
MpoolCheckPendingMessages performs logical checks for all pending messages from a given address


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response:
```json
[
  [
    {
      "Cid": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      "Code": 0,
      "OK": true,
      "Err": "string value",
      "Hint": {
        "abc": 123
      }
    }
  ]
]
```

### MpoolCheckReplaceMessages
MpoolCheckReplaceMessages performs logical checks on pending messages with replacement


Perms: read

Inputs:
```json
[
  [
    {
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 9,
      "GasFeeCap": "0",
      "GasPremium": "0",
      "Method": 1,
      "Params": "Ynl0ZSBhcnJheQ=="
    }
  ]
]
```

Response:
```json
[
  [
    {
      "Cid": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      "Code": 0,
      "OK": true,
      "Err": "string value",
      "Hint": {
        "abc": 123
      }
    }
  ]
]
```

### MpoolClear


Perms: write

Inputs:
```json
[
  true
]
```

Response: `{}`

### MpoolDeleteByAdress


Perms: admin

Inputs:
```json
[
  "f01234"
]
```

Response: `{}`

### MpoolGetConfig


Perms: read

Inputs: `[]`

Response:
```json
{
  "PriorityAddrs": [
    "f01234"
  ],
  "SizeLimitHigh": 123,
  "SizeLimitLow": 123,
  "ReplaceByFeeRatio": 1.23,
  "PruneCooldown": 60000000000,
  "GasLimitOverestimation": 12.3
}
```

### MpoolGetNonce


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `42`

### MpoolPending


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
[
  {
    "Message": {
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 9,
      "GasFeeCap": "0",
      "GasPremium": "0",
      "Method": 1,
      "Params": "Ynl0ZSBhcnJheQ=="
    },
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "CID": {
      "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
    }
  }
]
```

### MpoolPublishByAddr


Perms: write

Inputs:
```json
[
  "f01234"
]
```

Response: `{}`

### MpoolPublishMessage


Perms: write

Inputs:
```json
[
  {
    "Message": {
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 9,
      "GasFeeCap": "0",
      "GasPremium": "0",
      "Method": 1,
      "Params": "Ynl0ZSBhcnJheQ=="
    },
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "CID": {
      "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
    }
  }
]
```

Response: `{}`

### MpoolPush


Perms: write

Inputs:
```json
[
  {
    "Message": {
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 9,
      "GasFeeCap": "0",
      "GasPremium": "0",
      "Method": 1,
      "Params": "Ynl0ZSBhcnJheQ=="
    },
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "CID": {
      "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
    }
  }
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### MpoolPushMessage


Perms: sign

Inputs:
```json
[
  {
    "CID": {
      "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 9,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ=="
  },
  {
    "MaxFee": "0",
    "GasOverEstimation": 12.3,
    "GasOverPremium": 12.3
  }
]
```

Response:
```json
{
  "Message": {
    "CID": {
      "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 9,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ=="
  },
  "Signature": {
    "Type": 2,
    "Data": "Ynl0ZSBhcnJheQ=="
  },
  "CID": {
    "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
  }
}
```

### MpoolPushUntrusted


Perms: write

Inputs:
```json
[
  {
    "Message": {
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 9,
      "GasFeeCap": "0",
      "GasPremium": "0",
      "Method": 1,
      "Params": "Ynl0ZSBhcnJheQ=="
    },
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "CID": {
      "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
    }
  }
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### MpoolSelect


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ],
  12.3
]
```

Response:
```json
[
  {
    "Message": {
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 9,
      "GasFeeCap": "0",
      "GasPremium": "0",
      "Method": 1,
      "Params": "Ynl0ZSBhcnJheQ=="
    },
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "CID": {
      "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
    }
  }
]
```

### MpoolSelects


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ],
  [
    12.3
  ]
]
```

Response:
```json
[
  [
    {
      "Message": {
        "CID": {
          "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
        },
        "Version": 42,
        "To": "f01234",
        "From": "f01234",
        "Nonce": 42,
        "Value": "0",
        "GasLimit": 9,
        "GasFeeCap": "0",
        "GasPremium": "0",
        "Method": 1,
        "Params": "Ynl0ZSBhcnJheQ=="
      },
      "Signature": {
        "Type": 2,
        "Data": "Ynl0ZSBhcnJheQ=="
      },
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      }
    }
  ]
]
```

### MpoolSetConfig


Perms: admin

Inputs:
```json
[
  {
    "PriorityAddrs": [
      "f01234"
    ],
    "SizeLimitHigh": 123,
    "SizeLimitLow": 123,
    "ReplaceByFeeRatio": 1.23,
    "PruneCooldown": 60000000000,
    "GasLimitOverestimation": 12.3
  }
]
```

Response: `{}`

### MpoolSub


Perms: read

Inputs: `[]`

Response:
```json
{
  "Type": 0,
  "Message": {
    "Message": {
      "CID": {
        "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 9,
      "GasFeeCap": "0",
      "GasPremium": "0",
      "Method": 1,
      "Params": "Ynl0ZSBhcnJheQ=="
    },
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "CID": {
      "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
    }
  }
}
```

## MinerState

### StateAllMinerFaults


Perms: read

Inputs:
```json
[
  10101,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
[
  {
    "Miner": "f01234",
    "Epoch": 10101
  }
]
```

### StateChangedActors


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

Response:
```json
{
  "t01236": {
    "Code": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "Head": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "Nonce": 42,
    "Balance": "0",
    "Address": "f01234"
  }
}
```

### StateCirculatingSupply


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `"0"`

### StateComputeDataCID
StateComputeDataCID computes DataCID from a set of on-chain deals


Perms: read

Inputs:
```json
[
  "f01234",
  8,
  [
    5432
  ],
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### StateDealProviderCollateralBounds


Perms: read

Inputs:
```json
[
  1032,
  true,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "Min": "0",
  "Max": "0"
}
```

### StateDecodeParams


Perms: read

Inputs:
```json
[
  "f01234",
  1,
  "Ynl0ZSBhcnJheQ==",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `{}`

### StateEncodeParams


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  1,
  "json raw message"
]
```

Response: `"Ynl0ZSBhcnJheQ=="`

### StateGetAllocation
StateGetAllocation returns the allocation for a given address and allocation ID.


Perms: read

Inputs:
```json
[
  "f01234",
  0,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "Client": 1000,
  "Provider": 1000,
  "Data": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Size": 1032,
  "TermMin": 10101,
  "TermMax": 10101,
  "Expiration": 10101
}
```

### StateGetAllocationForPendingDeal
StateGetAllocationForPendingDeal returns the allocation for a given deal ID of a pending deal. Returns nil if
pending allocation is not found.


Perms: read

Inputs:
```json
[
  5432,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "Client": 1000,
  "Provider": 1000,
  "Data": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Size": 1032,
  "TermMin": 10101,
  "TermMax": 10101,
  "Expiration": 10101
}
```

### StateGetAllocations
StateGetAllocations returns the all the allocations for a given client.


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `{}`

### StateGetClaim
StateGetClaim returns the claim for a given address and claim ID.


Perms: read

Inputs:
```json
[
  "f01234",
  0,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "Provider": 1000,
  "Client": 1000,
  "Data": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Size": 1032,
  "TermMin": 10101,
  "TermMax": 10101,
  "TermStart": 10101,
  "Sector": 9
}
```

### StateGetClaims
StateGetClaims returns the all the claims for a given provider.


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `{}`

### StateListActors


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
[
  "f01234"
]
```

### StateListMessages


Perms: read

Inputs:
```json
[
  {
    "To": "f01234",
    "From": "f01234"
  },
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ],
  10101
]
```

Response:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

### StateListMiners


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
[
  "f01234"
]
```

### StateLookupID


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `"f01234"`

### StateLookupRobustAddress
StateLookupRobustAddress returns the public key address of the given ID address for non-account addresses (multisig, miners etc)


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `"f01234"`

### StateMarketBalance


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "Escrow": "0",
  "Locked": "0"
}
```

### StateMarketDeals


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "t026363": {
    "Proposal": {
      "PieceCID": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      "PieceSize": 1032,
      "VerifiedDeal": true,
      "Client": "f01234",
      "Provider": "f01234",
      "Label": "",
      "StartEpoch": 10101,
      "EndEpoch": 10101,
      "StoragePricePerEpoch": "0",
      "ProviderCollateral": "0",
      "ClientCollateral": "0"
    },
    "State": {
      "SectorStartEpoch": 10101,
      "LastUpdatedEpoch": 10101,
      "SlashEpoch": 10101,
      "VerifiedClaim": 0
    }
  }
}
```

### StateMarketStorageDeal


Perms: read

Inputs:
```json
[
  5432,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "Proposal": {
    "PieceCID": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "PieceSize": 1032,
    "VerifiedDeal": true,
    "Client": "f01234",
    "Provider": "f01234",
    "Label": "",
    "StartEpoch": 10101,
    "EndEpoch": 10101,
    "StoragePricePerEpoch": "0",
    "ProviderCollateral": "0",
    "ClientCollateral": "0"
  },
  "State": {
    "SectorStartEpoch": 10101,
    "LastUpdatedEpoch": 10101,
    "SlashEpoch": 10101,
    "VerifiedClaim": 0
  }
}
```

### StateMinerActiveSectors


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
[
  {
    "SectorNumber": 9,
    "SealProof": 8,
    "SealedCID": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "DealIDs": [
      5432
    ],
    "Activation": 10101,
    "Expiration": 10101,
    "DealWeight": "0",
    "VerifiedDealWeight": "0",
    "InitialPledge": "0",
    "ExpectedDayReward": "0",
    "ExpectedStoragePledge": "0",
    "ReplacedSectorAge": 10101,
    "ReplacedDayReward": "0",
    "SectorKeyCID": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "SimpleQAPower": true
  }
]
```

### StateMinerAllocated
StateMinerAllocated returns a bitfield containing all sector numbers marked as allocated in miner state


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
[
  5,
  1
]
```

### StateMinerAvailableBalance


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `"0"`

### StateMinerDeadlines


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
[
  {
    "PostSubmissions": [
      5,
      1
    ],
    "DisputableProofCount": 42
  }
]
```

### StateMinerFaults


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
[
  5,
  1
]
```

### StateMinerInfo


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "Owner": "f01234",
  "Worker": "f01234",
  "NewWorker": "f01234",
  "ControlAddresses": [
    "f01234"
  ],
  "WorkerChangeEpoch": 10101,
  "PeerId": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
  "Multiaddrs": [
    "Ynl0ZSBhcnJheQ=="
  ],
  "WindowPoStProofType": 8,
  "SectorSize": 34359738368,
  "WindowPoStPartitionSectors": 42,
  "ConsensusFaultElapsed": 10101,
  "PendingOwnerAddress": "f01234",
  "Beneficiary": "f01234",
  "BeneficiaryTerm": {
    "Quota": "0",
    "UsedQuota": "0",
    "Expiration": 10101
  },
  "PendingBeneficiaryTerm": {
    "NewBeneficiary": "f01234",
    "NewQuota": "0",
    "NewExpiration": 10101,
    "ApprovedByBeneficiary": true,
    "ApprovedByNominee": true
  }
}
```

### StateMinerInitialPledgeCollateral


Perms: read

Inputs:
```json
[
  "f01234",
  {
    "SealProof": 8,
    "SectorNumber": 9,
    "SealedCID": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "SealRandEpoch": 10101,
    "DealIDs": [
      5432
    ],
    "Expiration": 10101,
    "UnsealedCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
  },
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `"0"`

### StateMinerPartitions


Perms: read

Inputs:
```json
[
  "f01234",
  42,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
[
  {
    "AllSectors": [
      5,
      1
    ],
    "FaultySectors": [
      5,
      1
    ],
    "RecoveringSectors": [
      5,
      1
    ],
    "LiveSectors": [
      5,
      1
    ],
    "ActiveSectors": [
      5,
      1
    ]
  }
]
```

### StateMinerPower


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "MinerPower": {
    "RawBytePower": "0",
    "QualityAdjPower": "0"
  },
  "TotalPower": {
    "RawBytePower": "0",
    "QualityAdjPower": "0"
  },
  "HasMinPower": true
}
```

### StateMinerPreCommitDepositForPower


Perms: read

Inputs:
```json
[
  "f01234",
  {
    "SealProof": 8,
    "SectorNumber": 9,
    "SealedCID": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "SealRandEpoch": 10101,
    "DealIDs": [
      5432
    ],
    "Expiration": 10101,
    "UnsealedCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
  },
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `"0"`

### StateMinerProvingDeadline


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "CurrentEpoch": 10101,
  "PeriodStart": 10101,
  "Index": 42,
  "Open": 10101,
  "Close": 10101,
  "Challenge": 10101,
  "FaultCutoff": 10101,
  "WPoStPeriodDeadlines": 42,
  "WPoStProvingPeriod": 10101,
  "WPoStChallengeWindow": 10101,
  "WPoStChallengeLookback": 10101,
  "FaultDeclarationCutoff": 10101
}
```

### StateMinerRecoveries


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
[
  5,
  1
]
```

### StateMinerSectorAllocated


Perms: read

Inputs:
```json
[
  "f01234",
  9,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `true`

### StateMinerSectorCount


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "Live": 42,
  "Active": 42,
  "Faulty": 42
}
```

### StateMinerSectorSize


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `34359738368`

### StateMinerSectors


Perms: read

Inputs:
```json
[
  "f01234",
  [
    5,
    1
  ],
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
[
  {
    "SectorNumber": 9,
    "SealProof": 8,
    "SealedCID": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "DealIDs": [
      5432
    ],
    "Activation": 10101,
    "Expiration": 10101,
    "DealWeight": "0",
    "VerifiedDealWeight": "0",
    "InitialPledge": "0",
    "ExpectedDayReward": "0",
    "ExpectedStoragePledge": "0",
    "ReplacedSectorAge": 10101,
    "ReplacedDayReward": "0",
    "SectorKeyCID": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "SimpleQAPower": true
  }
]
```

### StateMinerWorkerAddress


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `"f01234"`

### StateReadState


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "Balance": "0",
  "Code": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "State": {}
}
```

### StateSectorExpiration


Perms: read

Inputs:
```json
[
  "f01234",
  9,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "OnTime": 10101,
  "Early": 10101
}
```

### StateSectorGetInfo


Perms: read

Inputs:
```json
[
  "f01234",
  9,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "SectorNumber": 9,
  "SealProof": 8,
  "SealedCID": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "DealIDs": [
    5432
  ],
  "Activation": 10101,
  "Expiration": 10101,
  "DealWeight": "0",
  "VerifiedDealWeight": "0",
  "InitialPledge": "0",
  "ExpectedDayReward": "0",
  "ExpectedStoragePledge": "0",
  "ReplacedSectorAge": 10101,
  "ReplacedDayReward": "0",
  "SectorKeyCID": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "SimpleQAPower": true
}
```

### StateSectorPartition


Perms: read

Inputs:
```json
[
  "f01234",
  9,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "Deadline": 42,
  "Partition": 42
}
```

### StateSectorPreCommitInfo
StateSectorPreCommitInfo returns the PreCommit info for the specified miner's sector.
Returns nil and no error if the sector isn't precommitted.

Note that the sector number may be allocated while PreCommitInfo is nil. This means that either allocated sector
numbers were compacted, and the sector number was marked as allocated in order to reduce size of the allocated
sectors bitfield, or that the sector was precommitted, but the precommit has expired.


Perms: read

Inputs:
```json
[
  "f01234",
  9,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "Info": {
    "SealProof": 8,
    "SectorNumber": 9,
    "SealedCID": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "SealRandEpoch": 10101,
    "DealIDs": [
      5432
    ],
    "Expiration": 10101,
    "UnsealedCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
  },
  "PreCommitDeposit": "0",
  "PreCommitEpoch": 10101
}
```

### StateVMCirculatingSupplyInternal


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "FilVested": "0",
  "FilMined": "0",
  "FilBurnt": "0",
  "FilLocked": "0",
  "FilCirculating": "0",
  "FilReserveDisbursed": "0"
}
```

### StateVerifiedClientStatus


Perms: read

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `"0"`

## Mining

### MinerCreateBlock


Perms: write

Inputs:
```json
[
  {
    "Miner": "f01234",
    "Parents": [
      {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      {
        "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
      }
    ],
    "Ticket": {
      "VRFProof": "Bw=="
    },
    "Eproof": {
      "WinCount": 9,
      "VRFProof": "Bw=="
    },
    "BeaconValues": [
      {
        "Round": 42,
        "Data": "Ynl0ZSBhcnJheQ=="
      }
    ],
    "Messages": [
      {
        "Message": {
          "CID": {
            "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
          },
          "Version": 42,
          "To": "f01234",
          "From": "f01234",
          "Nonce": 42,
          "Value": "0",
          "GasLimit": 9,
          "GasFeeCap": "0",
          "GasPremium": "0",
          "Method": 1,
          "Params": "Ynl0ZSBhcnJheQ=="
        },
        "Signature": {
          "Type": 2,
          "Data": "Ynl0ZSBhcnJheQ=="
        },
        "CID": {
          "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
        }
      }
    ],
    "Epoch": 10101,
    "Timestamp": 42,
    "WinningPoStProof": [
      {
        "PoStProof": 8,
        "ProofBytes": "Ynl0ZSBhcnJheQ=="
      }
    ]
  }
]
```

Response:
```json
{
  "Header": {
    "Miner": "f01234",
    "Ticket": {
      "VRFProof": "Bw=="
    },
    "ElectionProof": {
      "WinCount": 9,
      "VRFProof": "Bw=="
    },
    "BeaconEntries": [
      {
        "Round": 42,
        "Data": "Ynl0ZSBhcnJheQ=="
      }
    ],
    "WinPoStProof": [
      {
        "PoStProof": 8,
        "ProofBytes": "Ynl0ZSBhcnJheQ=="
      }
    ],
    "Parents": [
      {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      }
    ],
    "ParentWeight": "0",
    "Height": 10101,
    "ParentStateRoot": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "ParentMessageReceipts": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "Messages": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "BLSAggregate": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "Timestamp": 42,
    "BlockSig": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "ForkSignaling": 42,
    "ParentBaseFee": "0"
  },
  "BlsMessages": [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
  ],
  "SecpkMessages": [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
  ]
}
```

### MinerGetBaseInfo


Perms: read

Inputs:
```json
[
  "f01234",
  10101,
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
{
  "MinerPower": "0",
  "NetworkPower": "0",
  "Sectors": [
    {
      "SealProof": 8,
      "SectorNumber": 9,
      "SectorKey": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      "SealedCID": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      }
    }
  ],
  "WorkerKey": "f01234",
  "SectorSize": 34359738368,
  "PrevBeaconEntry": {
    "Round": 42,
    "Data": "Ynl0ZSBhcnJheQ=="
  },
  "BeaconEntries": [
    {
      "Round": 42,
      "Data": "Ynl0ZSBhcnJheQ=="
    }
  ],
  "EligibleForMining": true
}
```

## Network

### ID


Perms: read

Inputs: `[]`

Response: `"12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf"`

### NetAddrsListen


Perms: read

Inputs: `[]`

Response:
```json
{
  "ID": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
  "Addrs": [
    "/ip4/52.36.61.156/tcp/1347/p2p/12D3KooWFETiESTf1v4PGUvtnxMAcEFMzLZbJGg4tjWfGEimYior"
  ]
}
```

### NetAgentVersion


Perms: read

Inputs:
```json
[
  "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf"
]
```

Response: `"string value"`

### NetAutoNatStatus


Perms: read

Inputs: `[]`

Response:
```json
{
  "Reachability": 1,
  "PublicAddrs": [
    "string value"
  ]
}
```

### NetBandwidthStats
NetBandwidthStats returns statistics about the nodes total bandwidth
usage and current rate across all peers and protocols.


Perms: read

Inputs: `[]`

Response:
```json
{
  "TotalIn": 9,
  "TotalOut": 9,
  "RateIn": 12.3,
  "RateOut": 12.3
}
```

### NetBandwidthStatsByPeer
NetBandwidthStatsByPeer returns statistics about the nodes bandwidth
usage and current rate per peer


Perms: read

Inputs: `[]`

Response:
```json
{
  "12D3KooWSXmXLJmBR1M7i9RW9GQPNUhZSzXKzxDHWtAgNuJAbyEJ": {
    "TotalIn": 174000,
    "TotalOut": 12500,
    "RateIn": 100,
    "RateOut": 50
  }
}
```

### NetBandwidthStatsByProtocol
NetBandwidthStatsByProtocol returns statistics about the nodes bandwidth
usage and current rate per protocol


Perms: read

Inputs: `[]`

Response:
```json
{
  "/fil/hello/1.0.0": {
    "TotalIn": 174000,
    "TotalOut": 12500,
    "RateIn": 100,
    "RateOut": 50
  }
}
```

### NetConnect


Perms: admin

Inputs:
```json
[
  {
    "ID": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "Addrs": [
      "/ip4/52.36.61.156/tcp/1347/p2p/12D3KooWFETiESTf1v4PGUvtnxMAcEFMzLZbJGg4tjWfGEimYior"
    ]
  }
]
```

Response: `{}`

### NetConnectedness


Perms: read

Inputs:
```json
[
  "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf"
]
```

Response: `1`

### NetDisconnect


Perms: admin

Inputs:
```json
[
  "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf"
]
```

Response: `{}`

### NetFindPeer


Perms: read

Inputs:
```json
[
  "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf"
]
```

Response:
```json
{
  "ID": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
  "Addrs": [
    "/ip4/52.36.61.156/tcp/1347/p2p/12D3KooWFETiESTf1v4PGUvtnxMAcEFMzLZbJGg4tjWfGEimYior"
  ]
}
```

### NetFindProvidersAsync


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  123
]
```

Response:
```json
{
  "ID": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
  "Addrs": [
    "/ip4/52.36.61.156/tcp/1347/p2p/12D3KooWFETiESTf1v4PGUvtnxMAcEFMzLZbJGg4tjWfGEimYior"
  ]
}
```

### NetGetClosestPeers


Perms: read

Inputs:
```json
[
  "string value"
]
```

Response:
```json
[
  "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf"
]
```

### NetPeerInfo


Perms: read

Inputs:
```json
[
  "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf"
]
```

Response:
```json
{
  "ID": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
  "Agent": "string value",
  "Addrs": [
    "string value"
  ],
  "Protocols": [
    "string value"
  ],
  "ConnMgrMeta": {
    "FirstSeen": "0001-01-01T00:00:00Z",
    "Value": 123,
    "Tags": {
      "name": 42
    },
    "Conns": {
      "name": "2021-03-08T22:52:18Z"
    }
  }
}
```

### NetPeers


Perms: read

Inputs: `[]`

Response:
```json
[
  {
    "ID": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "Addrs": [
      "/ip4/52.36.61.156/tcp/1347/p2p/12D3KooWFETiESTf1v4PGUvtnxMAcEFMzLZbJGg4tjWfGEimYior"
    ]
  }
]
```

### NetPing


Perms: read

Inputs:
```json
[
  "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf"
]
```

Response: `60000000000`

### NetProtectAdd


Perms: admin

Inputs:
```json
[
  [
    "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf"
  ]
]
```

Response: `{}`

### NetProtectList


Perms: read

Inputs: `[]`

Response:
```json
[
  "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf"
]
```

### NetProtectRemove


Perms: admin

Inputs:
```json
[
  [
    "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf"
  ]
]
```

Response: `{}`

### NetPubsubScores


Perms: read

Inputs: `[]`

Response:
```json
[
  {
    "ID": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "Score": {
      "Score": 12.3,
      "Topics": {
        "/blocks": {
          "TimeInMesh": 60000000000,
          "FirstMessageDeliveries": 122,
          "MeshMessageDeliveries": 1234,
          "InvalidMessageDeliveries": 3
        }
      },
      "AppSpecificScore": 12.3,
      "IPColocationFactor": 12.3,
      "BehaviourPenalty": 12.3
    }
  }
]
```

## Paychan

### PaychAllocateLane
PaychAllocateLane Allocate late creates a lane within a payment channel so that calls to
CreatePaymentVoucher will automatically make vouchers only for the difference in total


Perms: sign

Inputs:
```json
[
  "f01234"
]
```

Response: `42`

### PaychAvailableFunds
PaychAvailableFunds get the status of an outbound payment channel
@pch: payment channel address


Perms: sign

Inputs:
```json
[
  "f01234"
]
```

Response:
```json
{
  "Channel": "f01234",
  "From": "f01234",
  "To": "f01234",
  "ConfirmedAmt": "0",
  "PendingAmt": "0",
  "NonReservedAmt": "0",
  "PendingAvailableAmt": "0",
  "PendingWaitSentinel": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "QueuedAmt": "0",
  "VoucherReedeemedAmt": "0"
}
```

### PaychAvailableFundsByFromTo
PaychAvailableFundsByFromTo  get the status of an outbound payment channel
@from: the payment channel sender
@to: he payment channel recipient


Perms: sign

Inputs:
```json
[
  "f01234",
  "f01234"
]
```

Response:
```json
{
  "Channel": "f01234",
  "From": "f01234",
  "To": "f01234",
  "ConfirmedAmt": "0",
  "PendingAmt": "0",
  "NonReservedAmt": "0",
  "PendingAvailableAmt": "0",
  "PendingWaitSentinel": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "QueuedAmt": "0",
  "VoucherReedeemedAmt": "0"
}
```

### PaychCollect
PaychCollect update payment channel status to collect
Collect sends the value of submitted vouchers to the channel recipient (the provider),
and refunds the remaining channel balance to the channel creator (the client).
@pch: payment channel address


Perms: sign

Inputs:
```json
[
  "f01234"
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### PaychFund
PaychFund gets or creates a payment channel between address pair.
The specified amount will be added to the channel through on-chain send for future use


Perms: sign

Inputs:
```json
[
  "f01234",
  "f01234",
  "0"
]
```

Response:
```json
{
  "Channel": "f01234",
  "WaitSentinel": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
}
```

### PaychGet
PaychGet gets or creates a payment channel between address pair
The specified amount will be reserved for use. If there aren't enough non-reserved funds
available, funds will be added through an on-chain message.
- When opts.OffChain is true, this call will not cause any messages to be sent to the chain (no automatic
channel creation/funds adding). If the operation can't be performed without sending a message an error will be
returned. Note that even when this option is specified, this call can be blocked by previous operations on the
channel waiting for on-chain operations.


Perms: sign

Inputs:
```json
[
  "f01234",
  "f01234",
  "0",
  {
    "OffChain": true
  }
]
```

Response:
```json
{
  "Channel": "f01234",
  "WaitSentinel": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
}
```

### PaychGetWaitReady
PaychGetWaitReady waits until the create channel / add funds message with the sentinel
@sentinel: given message CID arrives.
@ch: the returned channel address can safely be used against the Manager methods.


Perms: sign

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

Response: `"f01234"`

### PaychList
PaychList list the addresses of all channels that have been created


Perms: read

Inputs: `[]`

Response:
```json
[
  "f01234"
]
```

### PaychNewPayment
PaychNewPayment aggregate vouchers into a new lane
@from: the payment channel sender
@to: the payment channel recipient
@vouchers: the outstanding (non-redeemed) vouchers


Perms: sign

Inputs:
```json
[
  "f01234",
  "f01234",
  [
    {
      "Amount": "0",
      "TimeLockMin": 10101,
      "TimeLockMax": 10101,
      "MinSettle": 10101,
      "Extra": {
        "Actor": "f01234",
        "Method": 1,
        "Data": "Ynl0ZSBhcnJheQ=="
      }
    }
  ]
]
```

Response:
```json
{
  "Channel": "f01234",
  "WaitSentinel": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Vouchers": [
    {
      "ChannelAddr": "f01234",
      "TimeLockMin": 10101,
      "TimeLockMax": 10101,
      "SecretHash": "Ynl0ZSBhcnJheQ==",
      "Extra": {
        "Actor": "f01234",
        "Method": 1,
        "Data": "Ynl0ZSBhcnJheQ=="
      },
      "Lane": 42,
      "Nonce": 42,
      "Amount": "0",
      "MinSettleHeight": 10101,
      "Merges": [
        {
          "Lane": 42,
          "Nonce": 42
        }
      ],
      "Signature": {
        "Type": 2,
        "Data": "Ynl0ZSBhcnJheQ=="
      }
    }
  ]
}
```

### PaychSettle
PaychSettle update payment channel status to settle
After a settlement period (currently 12 hours) either party to the payment channel can call collect on chain
@pch: payment channel address


Perms: sign

Inputs:
```json
[
  "f01234"
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### PaychStatus
PaychStatus get the payment channel status
@pch: payment channel address


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response:
```json
{
  "ControlAddr": "f01234",
  "Direction": 1
}
```

### PaychVoucherAdd
PaychVoucherAdd adds a voucher for an inbound channel.
If the channel is not in the store, fetches the channel from state (and checks that
the channel To address is owned by the wallet).


Perms: write

Inputs:
```json
[
  "f01234",
  {
    "ChannelAddr": "f01234",
    "TimeLockMin": 10101,
    "TimeLockMax": 10101,
    "SecretHash": "Ynl0ZSBhcnJheQ==",
    "Extra": {
      "Actor": "f01234",
      "Method": 1,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "Lane": 42,
    "Nonce": 42,
    "Amount": "0",
    "MinSettleHeight": 10101,
    "Merges": [
      {
        "Lane": 42,
        "Nonce": 42
      }
    ],
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    }
  },
  "Ynl0ZSBhcnJheQ==",
  "0"
]
```

Response: `"0"`

### PaychVoucherCheckSpendable
PaychVoucherCheckSpendable checks if the given voucher is currently spendable
@pch: payment channel address
@sv: voucher


Perms: read

Inputs:
```json
[
  "f01234",
  {
    "ChannelAddr": "f01234",
    "TimeLockMin": 10101,
    "TimeLockMax": 10101,
    "SecretHash": "Ynl0ZSBhcnJheQ==",
    "Extra": {
      "Actor": "f01234",
      "Method": 1,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "Lane": 42,
    "Nonce": 42,
    "Amount": "0",
    "MinSettleHeight": 10101,
    "Merges": [
      {
        "Lane": 42,
        "Nonce": 42
      }
    ],
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    }
  },
  "Ynl0ZSBhcnJheQ==",
  "Ynl0ZSBhcnJheQ=="
]
```

Response: `true`

### PaychVoucherCheckValid
PaychVoucherCheckValid checks if the given voucher is valid (is or could become spendable at some point).
If the channel is not in the store, fetches the channel from state (and checks that
the channel To address is owned by the wallet).
@pch: payment channel address
@sv: voucher


Perms: read

Inputs:
```json
[
  "f01234",
  {
    "ChannelAddr": "f01234",
    "TimeLockMin": 10101,
    "TimeLockMax": 10101,
    "SecretHash": "Ynl0ZSBhcnJheQ==",
    "Extra": {
      "Actor": "f01234",
      "Method": 1,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "Lane": 42,
    "Nonce": 42,
    "Amount": "0",
    "MinSettleHeight": 10101,
    "Merges": [
      {
        "Lane": 42,
        "Nonce": 42
      }
    ],
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    }
  }
]
```

Response: `{}`

### PaychVoucherCreate
PaychVoucherCreate creates a new signed voucher on the given payment channel
with the given lane and amount.  The value passed in is exactly the value
that will be used to create the voucher, so if previous vouchers exist, the
actual additional value of this voucher will only be the difference between
the two.
If there are insufficient funds in the channel to create the voucher,
returns a nil voucher and the shortfall.


Perms: sign

Inputs:
```json
[
  "f01234",
  "0",
  42
]
```

Response:
```json
{
  "Voucher": {
    "ChannelAddr": "f01234",
    "TimeLockMin": 10101,
    "TimeLockMax": 10101,
    "SecretHash": "Ynl0ZSBhcnJheQ==",
    "Extra": {
      "Actor": "f01234",
      "Method": 1,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "Lane": 42,
    "Nonce": 42,
    "Amount": "0",
    "MinSettleHeight": 10101,
    "Merges": [
      {
        "Lane": 42,
        "Nonce": 42
      }
    ],
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    }
  },
  "Shortfall": "0"
}
```

### PaychVoucherList
PaychVoucherList list vouchers in payment channel
@pch: payment channel address


Perms: write

Inputs:
```json
[
  "f01234"
]
```

Response:
```json
[
  {
    "ChannelAddr": "f01234",
    "TimeLockMin": 10101,
    "TimeLockMax": 10101,
    "SecretHash": "Ynl0ZSBhcnJheQ==",
    "Extra": {
      "Actor": "f01234",
      "Method": 1,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "Lane": 42,
    "Nonce": 42,
    "Amount": "0",
    "MinSettleHeight": 10101,
    "Merges": [
      {
        "Lane": 42,
        "Nonce": 42
      }
    ],
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    }
  }
]
```

### PaychVoucherSubmit
PaychVoucherSubmit Submit voucher to chain to update payment channel state
@pch: payment channel address
@sv: voucher in payment channel


Perms: sign

Inputs:
```json
[
  "f01234",
  {
    "ChannelAddr": "f01234",
    "TimeLockMin": 10101,
    "TimeLockMax": 10101,
    "SecretHash": "Ynl0ZSBhcnJheQ==",
    "Extra": {
      "Actor": "f01234",
      "Method": 1,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "Lane": 42,
    "Nonce": 42,
    "Amount": "0",
    "MinSettleHeight": 10101,
    "Merges": [
      {
        "Lane": 42,
        "Nonce": 42
      }
    ],
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    }
  },
  "Ynl0ZSBhcnJheQ==",
  "Ynl0ZSBhcnJheQ=="
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

## Syncer

### ChainSyncHandleNewTipSet


Perms: write

Inputs:
```json
[
  {
    "Source": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "Sender": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "FullTipSet": {
      "Blocks": [
        {
          "Header": {
            "Miner": "f01234",
            "Ticket": {
              "VRFProof": "Bw=="
            },
            "ElectionProof": {
              "WinCount": 9,
              "VRFProof": "Bw=="
            },
            "BeaconEntries": [
              {
                "Round": 42,
                "Data": "Ynl0ZSBhcnJheQ=="
              }
            ],
            "WinPoStProof": [
              {
                "PoStProof": 8,
                "ProofBytes": "Ynl0ZSBhcnJheQ=="
              }
            ],
            "Parents": [
              {
                "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
              }
            ],
            "ParentWeight": "0",
            "Height": 10101,
            "ParentStateRoot": {
              "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
            },
            "ParentMessageReceipts": {
              "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
            },
            "Messages": {
              "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
            },
            "BLSAggregate": {
              "Type": 2,
              "Data": "Ynl0ZSBhcnJheQ=="
            },
            "Timestamp": 42,
            "BlockSig": {
              "Type": 2,
              "Data": "Ynl0ZSBhcnJheQ=="
            },
            "ForkSignaling": 42,
            "ParentBaseFee": "0"
          },
          "BLSMessages": [
            {
              "CID": {
                "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
              },
              "Version": 42,
              "To": "f01234",
              "From": "f01234",
              "Nonce": 42,
              "Value": "0",
              "GasLimit": 9,
              "GasFeeCap": "0",
              "GasPremium": "0",
              "Method": 1,
              "Params": "Ynl0ZSBhcnJheQ=="
            }
          ],
          "SECPMessages": [
            {
              "Message": {
                "CID": {
                  "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
                },
                "Version": 42,
                "To": "f01234",
                "From": "f01234",
                "Nonce": 42,
                "Value": "0",
                "GasLimit": 9,
                "GasFeeCap": "0",
                "GasPremium": "0",
                "Method": 1,
                "Params": "Ynl0ZSBhcnJheQ=="
              },
              "Signature": {
                "Type": 2,
                "Data": "Ynl0ZSBhcnJheQ=="
              },
              "CID": {
                "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
              }
            }
          ]
        }
      ]
    }
  }
]
```

Response: `{}`

### ChainTipSetWeight


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response: `"0"`

### Concurrent


Perms: read

Inputs: `[]`

Response: `9`

### SetConcurrent


Perms: admin

Inputs:
```json
[
  9
]
```

Response: `{}`

### SyncIncomingBlocks
SyncIncomingBlocks returns a channel streaming incoming, potentially not
yet synced block headers.


Perms: read

Inputs: `[]`

Response:
```json
{
  "Miner": "f01234",
  "Ticket": {
    "VRFProof": "Bw=="
  },
  "ElectionProof": {
    "WinCount": 9,
    "VRFProof": "Bw=="
  },
  "BeaconEntries": [
    {
      "Round": 42,
      "Data": "Ynl0ZSBhcnJheQ=="
    }
  ],
  "WinPoStProof": [
    {
      "PoStProof": 8,
      "ProofBytes": "Ynl0ZSBhcnJheQ=="
    }
  ],
  "Parents": [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
  ],
  "ParentWeight": "0",
  "Height": 10101,
  "ParentStateRoot": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "ParentMessageReceipts": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Messages": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "BLSAggregate": {
    "Type": 2,
    "Data": "Ynl0ZSBhcnJheQ=="
  },
  "Timestamp": 42,
  "BlockSig": {
    "Type": 2,
    "Data": "Ynl0ZSBhcnJheQ=="
  },
  "ForkSignaling": 42,
  "ParentBaseFee": "0"
}
```

### SyncState


Perms: read

Inputs: `[]`

Response:
```json
{
  "ActiveSyncs": [
    {
      "WorkerID": 42,
      "Base": {
        "Cids": null,
        "Blocks": null,
        "Height": 0
      },
      "Target": {
        "Cids": null,
        "Blocks": null,
        "Height": 0
      },
      "Stage": 1,
      "Height": 10101,
      "Start": "0001-01-01T00:00:00Z",
      "End": "0001-01-01T00:00:00Z",
      "Message": "string value"
    }
  ],
  "VMApplied": 42
}
```

### SyncSubmitBlock


Perms: write

Inputs:
```json
[
  {
    "Header": {
      "Miner": "f01234",
      "Ticket": {
        "VRFProof": "Bw=="
      },
      "ElectionProof": {
        "WinCount": 9,
        "VRFProof": "Bw=="
      },
      "BeaconEntries": [
        {
          "Round": 42,
          "Data": "Ynl0ZSBhcnJheQ=="
        }
      ],
      "WinPoStProof": [
        {
          "PoStProof": 8,
          "ProofBytes": "Ynl0ZSBhcnJheQ=="
        }
      ],
      "Parents": [
        {
          "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
        }
      ],
      "ParentWeight": "0",
      "Height": 10101,
      "ParentStateRoot": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      "ParentMessageReceipts": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      "Messages": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      "BLSAggregate": {
        "Type": 2,
        "Data": "Ynl0ZSBhcnJheQ=="
      },
      "Timestamp": 42,
      "BlockSig": {
        "Type": 2,
        "Data": "Ynl0ZSBhcnJheQ=="
      },
      "ForkSignaling": 42,
      "ParentBaseFee": "0"
    },
    "BlsMessages": [
      {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      }
    ],
    "SecpkMessages": [
      {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      }
    ]
  }
]
```

Response: `{}`

### SyncerTracker


Perms: read

Inputs: `[]`

Response:
```json
{
  "History": [
    {
      "State": 1,
      "Base": {
        "Cids": null,
        "Blocks": null,
        "Height": 0
      },
      "Current": {
        "Cids": null,
        "Blocks": null,
        "Height": 0
      },
      "Start": "0001-01-01T00:00:00Z",
      "End": "0001-01-01T00:00:00Z",
      "Err": {},
      "Head": {
        "Cids": null,
        "Blocks": null,
        "Height": 0
      },
      "Sender": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf"
    }
  ],
  "Buckets": [
    {
      "State": 1,
      "Base": {
        "Cids": null,
        "Blocks": null,
        "Height": 0
      },
      "Current": {
        "Cids": null,
        "Blocks": null,
        "Height": 0
      },
      "Start": "0001-01-01T00:00:00Z",
      "End": "0001-01-01T00:00:00Z",
      "Err": {},
      "Head": {
        "Cids": null,
        "Blocks": null,
        "Height": 0
      },
      "Sender": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf"
    }
  ]
}
```

## Wallet

### HasPassword


Perms: admin

Inputs: `[]`

Response: `true`

### LockWallet


Perms: admin

Inputs: `[]`

Response: `{}`

### SetPassword


Perms: admin

Inputs:
```json
[
  "Ynl0ZSBhcnJheQ=="
]
```

Response: `{}`

### UnLockWallet


Perms: admin

Inputs:
```json
[
  "Ynl0ZSBhcnJheQ=="
]
```

Response: `{}`

### WalletAddresses


Perms: admin

Inputs: `[]`

Response:
```json
[
  "f01234"
]
```

### WalletBalance


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `"0"`

### WalletDefaultAddress


Perms: write

Inputs: `[]`

Response: `"f01234"`

### WalletDelete


Perms: admin

Inputs:
```json
[
  "f01234"
]
```

Response: `{}`

### WalletExport


Perms: admin

Inputs:
```json
[
  "f01234",
  "string value"
]
```

Response:
```json
{
  "Type": "bls",
  "PrivateKey": "Ynl0ZSBhcnJheQ=="
}
```

### WalletHas


Perms: write

Inputs:
```json
[
  "f01234"
]
```

Response: `true`

### WalletImport


Perms: admin

Inputs:
```json
[
  {
    "Type": "bls",
    "PrivateKey": "Ynl0ZSBhcnJheQ=="
  }
]
```

Response: `"f01234"`

### WalletNewAddress


Perms: write

Inputs:
```json
[
  7
]
```

Response: `"f01234"`

### WalletSetDefault


Perms: write

Inputs:
```json
[
  "f01234"
]
```

Response: `{}`

### WalletSign


Perms: sign

Inputs:
```json
[
  "f01234",
  "Ynl0ZSBhcnJheQ==",
  {
    "Type": "message",
    "Extra": "Ynl0ZSBhcnJheQ=="
  }
]
```

Response:
```json
{
  "Type": 2,
  "Data": "Ynl0ZSBhcnJheQ=="
}
```

### WalletSignMessage


Perms: sign

Inputs:
```json
[
  "f01234",
  {
    "CID": {
      "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 9,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ=="
  }
]
```

Response:
```json
{
  "Message": {
    "CID": {
      "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 9,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ=="
  },
  "Signature": {
    "Type": 2,
    "Data": "Ynl0ZSBhcnJheQ=="
  },
  "CID": {
    "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
  }
}
```

### WalletState


Perms: admin

Inputs: `[]`

Response: `123`


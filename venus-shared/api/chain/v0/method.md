# Groups

* [Account](#account)
  * [StateAccountKey](#stateaccountkey)
* [Actor](#actor)
  * [ListActor](#listactor)
  * [StateGetActor](#stategetactor)
* [Beacon](#beacon)
  * [BeaconGetEntry](#beacongetentry)
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
  * [ChainGetGenesis](#chaingetgenesis)
  * [ChainGetMessage](#chaingetmessage)
  * [ChainGetMessagesInTipset](#chaingetmessagesintipset)
  * [ChainGetParentMessages](#chaingetparentmessages)
  * [ChainGetParentReceipts](#chaingetparentreceipts)
  * [ChainGetPath](#chaingetpath)
  * [ChainGetRandomnessFromBeacon](#chaingetrandomnessfrombeacon)
  * [ChainGetRandomnessFromTickets](#chaingetrandomnessfromtickets)
  * [ChainGetReceipts](#chaingetreceipts)
  * [ChainGetTipSet](#chaingettipset)
  * [ChainGetTipSetByHeight](#chaingettipsetbyheight)
  * [ChainHead](#chainhead)
  * [ChainList](#chainlist)
  * [ChainNotify](#chainnotify)
  * [ChainSetHead](#chainsethead)
  * [GetActor](#getactor)
  * [GetEntry](#getentry)
  * [GetFullBlock](#getfullblock)
  * [GetParentStateRootActor](#getparentstaterootactor)
  * [MessageWait](#messagewait)
  * [ProtocolParameters](#protocolparameters)
  * [ResolveToKeyAddr](#resolvetokeyaddr)
  * [StateActorCodeCIDs](#stateactorcodecids)
  * [StateActorManifestCID](#stateactormanifestcid)
  * [StateCall](#statecall)
  * [StateGetNetworkParams](#stategetnetworkparams)
  * [StateGetReceipt](#stategetreceipt)
  * [StateNetworkName](#statenetworkname)
  * [StateNetworkVersion](#statenetworkversion)
  * [StateSearchMsg](#statesearchmsg)
  * [StateSearchMsgLimited](#statesearchmsglimited)
  * [StateVerifiedRegistryRootKey](#stateverifiedregistryrootkey)
  * [StateVerifierStatus](#stateverifierstatus)
  * [StateWaitMsg](#statewaitmsg)
  * [StateWaitMsgLimited](#statewaitmsglimited)
  * [VerifyEntry](#verifyentry)
* [Common](#common)
  * [StartTime](#starttime)
  * [Version](#version)
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
  * [StateCirculatingSupply](#statecirculatingsupply)
  * [StateDealProviderCollateralBounds](#statedealprovidercollateralbounds)
  * [StateGetAllocation](#stategetallocation)
  * [StateGetAllocationForPendingDeal](#stategetallocationforpendingdeal)
  * [StateGetAllocations](#stategetallocations)
  * [StateGetClaim](#stategetclaim)
  * [StateGetClaims](#stategetclaims)
  * [StateListActors](#statelistactors)
  * [StateListMiners](#statelistminers)
  * [StateLookupID](#statelookupid)
  * [StateMarketBalance](#statemarketbalance)
  * [StateMarketDeals](#statemarketdeals)
  * [StateMarketStorageDeal](#statemarketstoragedeal)
  * [StateMinerActiveSectors](#statemineractivesectors)
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
  * [StateSectorExpiration](#statesectorexpiration)
  * [StateSectorGetInfo](#statesectorgetinfo)
  * [StateSectorPartition](#statesectorpartition)
  * [StateSectorPreCommitInfo](#statesectorprecommitinfo)
  * [StateVMCirculatingSupplyInternal](#statevmcirculatingsupplyinternal)
  * [StateVerifiedClientStatus](#stateverifiedclientstatus)
* [Mining](#mining)
  * [MinerCreateBlock](#minercreateblock)
  * [MinerGetBaseInfo](#minergetbaseinfo)
* [MultiSig](#multisig)
  * [MsigAddApprove](#msigaddapprove)
  * [MsigAddCancel](#msigaddcancel)
  * [MsigAddPropose](#msigaddpropose)
  * [MsigApprove](#msigapprove)
  * [MsigApproveTxnHash](#msigapprovetxnhash)
  * [MsigCancel](#msigcancel)
  * [MsigCancelTxnHash](#msigcanceltxnhash)
  * [MsigCreate](#msigcreate)
  * [MsigGetVested](#msiggetvested)
  * [MsigPropose](#msigpropose)
  * [MsigRemoveSigner](#msigremovesigner)
  * [MsigSwapApprove](#msigswapapprove)
  * [MsigSwapCancel](#msigswapcancel)
  * [MsigSwapPropose](#msigswappropose)
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
  "Balance": "0"
}
```

## Beacon

### BeaconGetEntry


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
    "VRFProof": null
  },
  "ElectionProof": null,
  "BeaconEntries": null,
  "WinPoStProof": null,
  "Parents": [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
  ],
  "ParentWeight": "0",
  "Height": 10101,
  "ParentStateRoot": null,
  "ParentMessageReceipts": null,
  "Messages": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "BLSAggregate": null,
  "Timestamp": 42,
  "BlockSig": null,
  "ForkSignaling": 0,
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
  "BlsMessages": null,
  "SecpkMessages": null,
  "Cids": [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
  ]
}
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
    "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
  },
  "Version": 42,
  "To": "f01234",
  "From": "f01234",
  "Nonce": 42,
  "Value": "0",
  "GasLimit": 0,
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
        "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 0,
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
        "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 0,
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
    "GasUsed": 0
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

### ChainGetRandomnessFromBeacon


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
  2,
  10101,
  "Ynl0ZSBhcnJheQ=="
]
```

Response: `"Bw=="`

### ChainGetRandomnessFromTickets


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
  2,
  10101,
  "Ynl0ZSBhcnJheQ=="
]
```

Response: `"Bw=="`

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
    "GasUsed": 0
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
  "Balance": "0"
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
      "VRFProof": null
    },
    "ElectionProof": null,
    "BeaconEntries": null,
    "WinPoStProof": null,
    "Parents": [
      {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      }
    ],
    "ParentWeight": "0",
    "Height": 10101,
    "ParentStateRoot": null,
    "ParentMessageReceipts": null,
    "Messages": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "BLSAggregate": null,
    "Timestamp": 42,
    "BlockSig": null,
    "ForkSignaling": 0,
    "ParentBaseFee": "0"
  },
  "BLSMessages": null,
  "SECPMessages": null
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
  "Balance": "0"
}
```

### MessageWait


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  10101,
  10101
]
```

Response:
```json
{
  "TS": null,
  "Message": {
    "CID": {
      "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 0,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ=="
  },
  "Block": {
    "Miner": "f01234",
    "Ticket": {
      "VRFProof": null
    },
    "ElectionProof": null,
    "BeaconEntries": null,
    "WinPoStProof": null,
    "Parents": [
      {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      }
    ],
    "ParentWeight": "0",
    "Height": 10101,
    "ParentStateRoot": null,
    "ParentMessageReceipts": null,
    "Messages": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "BLSAggregate": null,
    "Timestamp": 42,
    "BlockSig": null,
    "ForkSignaling": 0,
    "ParentBaseFee": "0"
  },
  "Receipt": {
    "ExitCode": 0,
    "Return": "Ynl0ZSBhcnJheQ==",
    "GasUsed": 0
  }
}
```

### ProtocolParameters


Perms: read

Inputs: `[]`

Response:
```json
{
  "Network": "string value",
  "BlockTime": 0,
  "SupportedSectors": null
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
  17
]
```

Response: `{}`

### StateActorManifestCID
StateActorManifestCID returns the CID of the builtin actors manifest for the given network version


Perms: read

Inputs:
```json
[
  17
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
      "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 0,
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
  "MsgCid": null,
  "Msg": {
    "CID": {
      "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 0,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ=="
  },
  "MsgRct": null,
  "GasCost": {
    "Message": null,
    "GasUsed": "0",
    "BaseFeeBurn": "0",
    "OverEstimationBurn": "0",
    "MinerPenalty": "0",
    "MinerTip": "0",
    "Refund": "0",
    "TotalCost": "0"
  },
  "ExecutionTrace": {
    "Msg": null,
    "MsgRct": null,
    "Error": "",
    "Duration": 0,
    "GasCharges": null,
    "Subcalls": null
  },
  "Error": "string value",
  "Duration": 60000000000
}
```

### StateGetNetworkParams
StateGetNetworkParams return current network params


Perms: read

Inputs: `[]`

Response:
```json
{
  "NetworkName": "",
  "BlockDelaySecs": 0,
  "ConsensusMinerMinPower": "0",
  "SupportedProofTypes": null,
  "PreCommitChallengeDelay": 0,
  "ForkUpgradeParams": {
    "UpgradeSmokeHeight": 0,
    "UpgradeBreezeHeight": 0,
    "UpgradeIgnitionHeight": 0,
    "UpgradeLiftoffHeight": 0,
    "UpgradeAssemblyHeight": 0,
    "UpgradeRefuelHeight": 0,
    "UpgradeTapeHeight": 0,
    "UpgradeKumquatHeight": 0,
    "BreezeGasTampingDuration": 0,
    "UpgradeCalicoHeight": 0,
    "UpgradePersianHeight": 0,
    "UpgradeOrangeHeight": 0,
    "UpgradeClausHeight": 0,
    "UpgradeTrustHeight": 0,
    "UpgradeNorwegianHeight": 0,
    "UpgradeTurboHeight": 0,
    "UpgradeHyperdriveHeight": 0,
    "UpgradeChocolateHeight": 0,
    "UpgradeOhSnapHeight": 0,
    "UpgradeSkyrHeight": 0,
    "UpgradeSharkHeight": 0
  }
}
```

### StateGetReceipt


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
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
  "ExitCode": 0,
  "Return": "Ynl0ZSBhcnJheQ==",
  "GasUsed": 0
}
```

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

Response: `17`

### StateSearchMsg


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
  "Message": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Receipt": {
    "ExitCode": 0,
    "Return": "Ynl0ZSBhcnJheQ==",
    "GasUsed": 0
  },
  "ReturnDec": null,
  "TipSet": [],
  "Height": 10101
}
```

### StateSearchMsgLimited


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  10101
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
    "GasUsed": 0
  },
  "ReturnDec": null,
  "TipSet": [],
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


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  42
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
    "GasUsed": 0
  },
  "ReturnDec": null,
  "TipSet": [],
  "Height": 10101
}
```

### StateWaitMsgLimited


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  42,
  10101
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
    "GasUsed": 0
  },
  "ReturnDec": null,
  "TipSet": [],
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
  "APIVersion": 0
}
```

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
          "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
        },
        "Version": 42,
        "To": "f01234",
        "From": "f01234",
        "Nonce": 42,
        "Value": "0",
        "GasLimit": 0,
        "GasFeeCap": "0",
        "GasPremium": "0",
        "Method": 1,
        "Params": "Ynl0ZSBhcnJheQ=="
      },
      "Spec": {
        "MaxFee": "0",
        "GasOverEstimation": 0,
        "GasOverPremium": 0
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
        "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 0,
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
      "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 0,
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
      "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 0,
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
      "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 0,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ=="
  },
  {
    "MaxFee": "0",
    "GasOverEstimation": 0,
    "GasOverPremium": 0
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
    "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
  },
  "Version": 42,
  "To": "f01234",
  "From": "f01234",
  "Nonce": 42,
  "Value": "0",
  "GasLimit": 0,
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
          "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
        },
        "Version": 42,
        "To": "f01234",
        "From": "f01234",
        "Nonce": 42,
        "Value": "0",
        "GasLimit": 0,
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
        "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
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
        "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 0,
      "GasFeeCap": "0",
      "GasPremium": "0",
      "Method": 1,
      "Params": "Ynl0ZSBhcnJheQ=="
    }
  ],
  {
    "MaxFee": "0",
    "GasOverEstimation": 0,
    "GasOverPremium": 0
  }
]
```

Response:
```json
[
  {
    "Message": {
      "CID": {
        "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 0,
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
      "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
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
          "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
        },
        "Version": 42,
        "To": "f01234",
        "From": "f01234",
        "Nonce": 42,
        "Value": "0",
        "GasLimit": 0,
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
        "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
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
  "PriorityAddrs": null,
  "SizeLimitHigh": 0,
  "SizeLimitLow": 0,
  "ReplaceByFeeRatio": 0,
  "PruneCooldown": 0,
  "GasLimitOverestimation": 0
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
        "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 0,
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
      "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
    }
  }
]
```

### MpoolPublishByAddr


Perms: admin

Inputs:
```json
[
  "f01234"
]
```

Response: `{}`

### MpoolPublishMessage


Perms: admin

Inputs:
```json
[
  {
    "Message": {
      "CID": {
        "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 0,
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
      "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
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
        "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 0,
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
      "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
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
      "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 0,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ=="
  },
  {
    "MaxFee": "0",
    "GasOverEstimation": 0,
    "GasOverPremium": 0
  }
]
```

Response:
```json
{
  "Message": {
    "CID": {
      "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 0,
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
    "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
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
        "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 0,
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
      "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
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
        "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 0,
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
      "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
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
          "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
        },
        "Version": 42,
        "To": "f01234",
        "From": "f01234",
        "Nonce": 42,
        "Value": "0",
        "GasLimit": 0,
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
        "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
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
    "PriorityAddrs": null,
    "SizeLimitHigh": 0,
    "SizeLimitLow": 0,
    "ReplaceByFeeRatio": 0,
    "PruneCooldown": 0,
    "GasLimitOverestimation": 0
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
        "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
      },
      "Version": 42,
      "To": "f01234",
      "From": "f01234",
      "Nonce": 42,
      "Value": "0",
      "GasLimit": 0,
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
      "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
    }
  }
}
```

## MinerState

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
  "TermMin": 0,
  "TermMax": 0,
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
  "TermMin": 0,
  "TermMax": 0,
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
  "TermMin": 0,
  "TermMax": 0,
  "TermStart": 0,
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
      "PieceCID": null,
      "PieceSize": 0,
      "VerifiedDeal": false,
      "Client": "f01234",
      "Provider": "f01234",
      "Label": "",
      "StartEpoch": 0,
      "EndEpoch": 0,
      "StoragePricePerEpoch": "0",
      "ProviderCollateral": "0",
      "ClientCollateral": "0"
    },
    "State": {
      "SectorStartEpoch": 0,
      "LastUpdatedEpoch": 0,
      "SlashEpoch": 0,
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
    "PieceCID": null,
    "PieceSize": 0,
    "VerifiedDeal": false,
    "Client": "f01234",
    "Provider": "f01234",
    "Label": "",
    "StartEpoch": 0,
    "EndEpoch": 0,
    "StoragePricePerEpoch": "0",
    "ProviderCollateral": "0",
    "ClientCollateral": "0"
  },
  "State": {
    "SectorStartEpoch": 0,
    "LastUpdatedEpoch": 0,
    "SlashEpoch": 0,
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
    "SectorNumber": 0,
    "SealProof": 0,
    "SealedCID": null,
    "DealIDs": null,
    "Activation": 10101,
    "Expiration": 10101,
    "DealWeight": "0",
    "VerifiedDealWeight": "0",
    "InitialPledge": "0",
    "ExpectedDayReward": "0",
    "ExpectedStoragePledge": "0",
    "ReplacedSectorAge": 0,
    "ReplacedDayReward": "0",
    "SectorKeyCID": null,
    "SimpleQAPower": false
  }
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
      0
    ],
    "DisputableProofCount": 0
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
  "NewWorker": "\u003cempty\u003e",
  "ControlAddresses": null,
  "WorkerChangeEpoch": 0,
  "PeerId": null,
  "Multiaddrs": [
    "Ynl0ZSBhcnJheQ=="
  ],
  "WindowPoStProofType": 0,
  "SectorSize": 0,
  "WindowPoStPartitionSectors": 0,
  "ConsensusFaultElapsed": 0,
  "Beneficiary": "f01234",
  "BeneficiaryTerm": null,
  "PendingBeneficiaryTerm": null
}
```

### StateMinerInitialPledgeCollateral


Perms: read

Inputs:
```json
[
  "f01234",
  {
    "SealProof": 0,
    "SectorNumber": 0,
    "SealedCID": null,
    "SealRandEpoch": 0,
    "DealIDs": null,
    "Expiration": 10101,
    "UnsealedCid": null
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
      0
    ],
    "FaultySectors": [
      0
    ],
    "RecoveringSectors": [
      0
    ],
    "LiveSectors": [
      0
    ],
    "ActiveSectors": [
      0
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
  "HasMinPower": false
}
```

### StateMinerPreCommitDepositForPower


Perms: read

Inputs:
```json
[
  "f01234",
  {
    "SealProof": 0,
    "SectorNumber": 0,
    "SealedCID": null,
    "SealRandEpoch": 0,
    "DealIDs": null,
    "Expiration": 10101,
    "UnsealedCid": null
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
  "CurrentEpoch": 0,
  "PeriodStart": 0,
  "Index": 42,
  "Open": 10101,
  "Close": 10101,
  "Challenge": 10101,
  "FaultCutoff": 0,
  "WPoStPeriodDeadlines": 0,
  "WPoStProvingPeriod": 0,
  "WPoStChallengeWindow": 0,
  "WPoStChallengeLookback": 0,
  "FaultDeclarationCutoff": 0
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
    "SectorNumber": 0,
    "SealProof": 0,
    "SealedCID": null,
    "DealIDs": null,
    "Activation": 10101,
    "Expiration": 10101,
    "DealWeight": "0",
    "VerifiedDealWeight": "0",
    "InitialPledge": "0",
    "ExpectedDayReward": "0",
    "ExpectedStoragePledge": "0",
    "ReplacedSectorAge": 0,
    "ReplacedDayReward": "0",
    "SectorKeyCID": null,
    "SimpleQAPower": false
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
  "OnTime": 0,
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
  "SectorNumber": 0,
  "SealProof": 0,
  "SealedCID": null,
  "DealIDs": null,
  "Activation": 10101,
  "Expiration": 10101,
  "DealWeight": "0",
  "VerifiedDealWeight": "0",
  "InitialPledge": "0",
  "ExpectedDayReward": "0",
  "ExpectedStoragePledge": "0",
  "ReplacedSectorAge": 0,
  "ReplacedDayReward": "0",
  "SectorKeyCID": null,
  "SimpleQAPower": false
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
    "SealProof": 0,
    "SectorNumber": 0,
    "SealedCID": null,
    "SealRandEpoch": 0,
    "DealIDs": null,
    "Expiration": 10101,
    "UnsealedCid": null
  },
  "PreCommitDeposit": "\u003cnil\u003e",
  "PreCommitEpoch": 0
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
  "FilVested": "\u003cnil\u003e",
  "FilMined": "\u003cnil\u003e",
  "FilBurnt": "\u003cnil\u003e",
  "FilLocked": "\u003cnil\u003e",
  "FilCirculating": "\u003cnil\u003e",
  "FilReserveDisbursed": "\u003cnil\u003e"
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
      "VRFProof": null
    },
    "Eproof": {
      "WinCount": 0,
      "VRFProof": null
    },
    "BeaconValues": null,
    "Messages": [
      {
        "Message": {
          "CID": {
            "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
          },
          "Version": 42,
          "To": "f01234",
          "From": "f01234",
          "Nonce": 42,
          "Value": "0",
          "GasLimit": 0,
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
          "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
        }
      }
    ],
    "Epoch": 10101,
    "Timestamp": 42,
    "WinningPoStProof": null
  }
]
```

Response:
```json
{
  "Header": {
    "Miner": "f01234",
    "Ticket": {
      "VRFProof": null
    },
    "ElectionProof": null,
    "BeaconEntries": null,
    "WinPoStProof": null,
    "Parents": [
      {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      }
    ],
    "ParentWeight": "0",
    "Height": 10101,
    "ParentStateRoot": null,
    "ParentMessageReceipts": null,
    "Messages": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "BLSAggregate": null,
    "Timestamp": 42,
    "BlockSig": null,
    "ForkSignaling": 0,
    "ParentBaseFee": "0"
  },
  "BlsMessages": null,
  "SecpkMessages": null
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
      "SealProof": 0,
      "SectorNumber": 0,
      "SectorKey": null,
      "SealedCID": null
    }
  ],
  "WorkerKey": "\u003cempty\u003e",
  "SectorSize": 0,
  "PrevBeaconEntry": {
    "Round": 0,
    "Data": null
  },
  "BeaconEntries": null,
  "EligibleForMining": false
}
```

## MultiSig

### MsigAddApprove


Perms: sign

Inputs:
```json
[
  "f01234",
  "f01234",
  42,
  "f01234",
  "f01234",
  true
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### MsigAddCancel


Perms: sign

Inputs:
```json
[
  "f01234",
  "f01234",
  42,
  "f01234",
  true
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### MsigAddPropose


Perms: sign

Inputs:
```json
[
  "f01234",
  "f01234",
  "f01234",
  true
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### MsigApprove


Perms: sign

Inputs:
```json
[
  "f01234",
  42,
  "f01234"
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### MsigApproveTxnHash


Perms: sign

Inputs:
```json
[
  "f01234",
  42,
  "f01234",
  "f01234",
  "0",
  "f01234",
  42,
  "Ynl0ZSBhcnJheQ=="
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### MsigCancel


Perms: sign

Inputs:
```json
[
  "f01234",
  42,
  "f01234"
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### MsigCancelTxnHash
MsigCancel cancels a previously-proposed multisig message
It takes the following params: \<multisig address>, \<proposed transaction ID>, \<recipient address>, \<value to transfer>,
\<sender address of the cancel msg>, \<method to call in the proposed message>, \<params to include in the proposed message>


Perms: sign

Inputs:
```json
[
  "f01234",
  42,
  "f01234",
  "0",
  "f01234",
  42,
  "Ynl0ZSBhcnJheQ=="
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### MsigCreate
MsigCreate creates a multisig wallet
It takes the following params: \<required number of senders>, \<approving addresses>, \<unlock duration>
\<initial balance>, \<sender address of the create msg>, \<gas price>


Perms: sign

Inputs:
```json
[
  42,
  [
    "f01234"
  ],
  10101,
  "0",
  "f01234",
  "0"
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### MsigGetVested


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

Response: `"0"`

### MsigPropose


Perms: sign

Inputs:
```json
[
  "f01234",
  "f01234",
  "0",
  "f01234",
  42,
  "Ynl0ZSBhcnJheQ=="
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### MsigRemoveSigner


Perms: sign

Inputs:
```json
[
  "f01234",
  "f01234",
  "f01234",
  true
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### MsigSwapApprove


Perms: sign

Inputs:
```json
[
  "f01234",
  "f01234",
  42,
  "f01234",
  "f01234",
  "f01234"
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### MsigSwapCancel


Perms: sign

Inputs:
```json
[
  "f01234",
  "f01234",
  42,
  "f01234",
  "f01234"
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### MsigSwapPropose


Perms: sign

Inputs:
```json
[
  "f01234",
  "f01234",
  "f01234",
  "f01234"
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
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
  "ID": "",
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
  "PublicAddr": ""
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
  "TotalIn": 0,
  "TotalOut": 0,
  "RateIn": 0,
  "RateOut": 0
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
    "ID": "",
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
  "ID": "",
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
  "ID": "",
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
  "ID": "",
  "Agent": "string value",
  "Addrs": [
    "string value"
  ],
  "Protocols": [
    "string value"
  ],
  "ConnMgrMeta": null
}
```

### NetPeers


Perms: read

Inputs: `[]`

Response:
```json
[
  {
    "ID": "",
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
    "ID": "",
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
      "AppSpecificScore": 0,
      "IPColocationFactor": 0,
      "BehaviourPenalty": 0
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
  "PendingWaitSentinel": null,
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
  "PendingWaitSentinel": null,
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

### PaychGet
PaychGet creates a payment channel to a provider with a amount of FIL
@from: the payment channel sender
@to: the payment channel recipient
@amt: the deposits funds in the payment channel


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
  "WaitSentinel": null
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
      "TimeLockMin": 0,
      "TimeLockMax": 0,
      "MinSettle": 0,
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
  "WaitSentinel": null,
  "Vouchers": [
    {
      "ChannelAddr": "\u003cempty\u003e",
      "TimeLockMin": 0,
      "TimeLockMax": 0,
      "SecretHash": null,
      "Extra": {
        "Actor": "f01234",
        "Method": 1,
        "Data": "Ynl0ZSBhcnJheQ=="
      },
      "Lane": 42,
      "Nonce": 42,
      "Amount": "0",
      "MinSettleHeight": 0,
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
  "ControlAddr": "\u003cempty\u003e",
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
    "ChannelAddr": "\u003cempty\u003e",
    "TimeLockMin": 0,
    "TimeLockMax": 0,
    "SecretHash": null,
    "Extra": {
      "Actor": "f01234",
      "Method": 1,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "Lane": 42,
    "Nonce": 42,
    "Amount": "0",
    "MinSettleHeight": 0,
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
    "ChannelAddr": "\u003cempty\u003e",
    "TimeLockMin": 0,
    "TimeLockMax": 0,
    "SecretHash": null,
    "Extra": {
      "Actor": "f01234",
      "Method": 1,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "Lane": 42,
    "Nonce": 42,
    "Amount": "0",
    "MinSettleHeight": 0,
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
    "ChannelAddr": "\u003cempty\u003e",
    "TimeLockMin": 0,
    "TimeLockMax": 0,
    "SecretHash": null,
    "Extra": {
      "Actor": "f01234",
      "Method": 1,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "Lane": 42,
    "Nonce": 42,
    "Amount": "0",
    "MinSettleHeight": 0,
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
    "ChannelAddr": "\u003cempty\u003e",
    "TimeLockMin": 0,
    "TimeLockMax": 0,
    "SecretHash": null,
    "Extra": {
      "Actor": "f01234",
      "Method": 1,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "Lane": 42,
    "Nonce": 42,
    "Amount": "0",
    "MinSettleHeight": 0,
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
    "ChannelAddr": "\u003cempty\u003e",
    "TimeLockMin": 0,
    "TimeLockMax": 0,
    "SecretHash": null,
    "Extra": {
      "Actor": "f01234",
      "Method": 1,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "Lane": 42,
    "Nonce": 42,
    "Amount": "0",
    "MinSettleHeight": 0,
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
    "ChannelAddr": "\u003cempty\u003e",
    "TimeLockMin": 0,
    "TimeLockMax": 0,
    "SecretHash": null,
    "Extra": {
      "Actor": "f01234",
      "Method": 1,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "Lane": 42,
    "Nonce": 42,
    "Amount": "0",
    "MinSettleHeight": 0,
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
    "Head": {
      "Cids": null,
      "Blocks": null,
      "Height": 0
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

### SyncState


Perms: read

Inputs: `[]`

Response:
```json
{
  "ActiveSyncs": null,
  "VMApplied": 0
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
        "VRFProof": null
      },
      "ElectionProof": null,
      "BeaconEntries": null,
      "WinPoStProof": null,
      "Parents": [
        {
          "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
        }
      ],
      "ParentWeight": "0",
      "Height": 10101,
      "ParentStateRoot": null,
      "ParentMessageReceipts": null,
      "Messages": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      "BLSAggregate": null,
      "Timestamp": 42,
      "BlockSig": null,
      "ForkSignaling": 0,
      "ParentBaseFee": "0"
    },
    "BlsMessages": null,
    "SecpkMessages": null
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
      "Source": "",
      "Sender": "",
      "Head": null
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
      "Source": "",
      "Sender": "",
      "Head": null
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
  "PrivateKey": null
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
    "PrivateKey": null
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
      "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 0,
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
      "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 0,
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
    "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
  }
}
```

### WalletState


Perms: admin

Inputs: `[]`

Response: `123`


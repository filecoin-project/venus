# Groups

* [Market](#market)
  * [ActorExist](#actorexist)
  * [ActorList](#actorlist)
  * [ActorSectorSize](#actorsectorsize)
  * [AddFsPieceStorage](#addfspiecestorage)
  * [AddS3PieceStorage](#adds3piecestorage)
  * [AssignUnPackedDeals](#assignunpackeddeals)
  * [DagstoreGC](#dagstoregc)
  * [DagstoreInitializeAll](#dagstoreinitializeall)
  * [DagstoreInitializeShard](#dagstoreinitializeshard)
  * [DagstoreInitializeStorage](#dagstoreinitializestorage)
  * [DagstoreListShards](#dagstorelistshards)
  * [DagstoreRecoverShard](#dagstorerecovershard)
  * [DealsConsiderOfflineRetrievalDeals](#dealsconsiderofflineretrievaldeals)
  * [DealsConsiderOfflineStorageDeals](#dealsconsiderofflinestoragedeals)
  * [DealsConsiderOnlineRetrievalDeals](#dealsconsideronlineretrievaldeals)
  * [DealsConsiderOnlineStorageDeals](#dealsconsideronlinestoragedeals)
  * [DealsConsiderUnverifiedStorageDeals](#dealsconsiderunverifiedstoragedeals)
  * [DealsConsiderVerifiedStorageDeals](#dealsconsiderverifiedstoragedeals)
  * [DealsImportData](#dealsimportdata)
  * [DealsMaxProviderCollateralMultiplier](#dealsmaxprovidercollateralmultiplier)
  * [DealsMaxPublishFee](#dealsmaxpublishfee)
  * [DealsMaxStartDelay](#dealsmaxstartdelay)
  * [DealsPieceCidBlocklist](#dealspiececidblocklist)
  * [DealsPublishMsgPeriod](#dealspublishmsgperiod)
  * [DealsSetConsiderOfflineRetrievalDeals](#dealssetconsiderofflineretrievaldeals)
  * [DealsSetConsiderOfflineStorageDeals](#dealssetconsiderofflinestoragedeals)
  * [DealsSetConsiderOnlineRetrievalDeals](#dealssetconsideronlineretrievaldeals)
  * [DealsSetConsiderOnlineStorageDeals](#dealssetconsideronlinestoragedeals)
  * [DealsSetConsiderUnverifiedStorageDeals](#dealssetconsiderunverifiedstoragedeals)
  * [DealsSetConsiderVerifiedStorageDeals](#dealssetconsiderverifiedstoragedeals)
  * [DealsSetMaxProviderCollateralMultiplier](#dealssetmaxprovidercollateralmultiplier)
  * [DealsSetMaxPublishFee](#dealssetmaxpublishfee)
  * [DealsSetMaxStartDelay](#dealssetmaxstartdelay)
  * [DealsSetPieceCidBlocklist](#dealssetpiececidblocklist)
  * [DealsSetPublishMsgPeriod](#dealssetpublishmsgperiod)
  * [GetDeals](#getdeals)
  * [GetRetrievalDealStatistic](#getretrievaldealstatistic)
  * [GetStorageDealStatistic](#getstoragedealstatistic)
  * [GetUnPackedDeals](#getunpackeddeals)
  * [ID](#id)
  * [ImportV1Data](#importv1data)
  * [ListPieceStorageInfos](#listpiecestorageinfos)
  * [ListenMarketEvent](#listenmarketevent)
  * [MarkDealsAsPacking](#markdealsaspacking)
  * [MarketAddBalance](#marketaddbalance)
  * [MarketCancelDataTransfer](#marketcanceldatatransfer)
  * [MarketDataTransferPath](#marketdatatransferpath)
  * [MarketDataTransferUpdates](#marketdatatransferupdates)
  * [MarketGetAsk](#marketgetask)
  * [MarketGetDealUpdates](#marketgetdealupdates)
  * [MarketGetReserved](#marketgetreserved)
  * [MarketGetRetrievalAsk](#marketgetretrievalask)
  * [MarketImportDealData](#marketimportdealdata)
  * [MarketImportPublishedDeal](#marketimportpublisheddeal)
  * [MarketListAsk](#marketlistask)
  * [MarketListDataTransfers](#marketlistdatatransfers)
  * [MarketListDeals](#marketlistdeals)
  * [MarketListIncompleteDeals](#marketlistincompletedeals)
  * [MarketListRetrievalAsk](#marketlistretrievalask)
  * [MarketListRetrievalDeals](#marketlistretrievaldeals)
  * [MarketMaxBalanceAddFee](#marketmaxbalanceaddfee)
  * [MarketMaxDealsPerPublishMsg](#marketmaxdealsperpublishmsg)
  * [MarketPendingDeals](#marketpendingdeals)
  * [MarketPublishPendingDeals](#marketpublishpendingdeals)
  * [MarketReleaseFunds](#marketreleasefunds)
  * [MarketReserveFunds](#marketreservefunds)
  * [MarketRestartDataTransfer](#marketrestartdatatransfer)
  * [MarketSetAsk](#marketsetask)
  * [MarketSetDataTransferPath](#marketsetdatatransferpath)
  * [MarketSetMaxBalanceAddFee](#marketsetmaxbalanceaddfee)
  * [MarketSetMaxDealsPerPublishMsg](#marketsetmaxdealsperpublishmsg)
  * [MarketSetRetrievalAsk](#marketsetretrievalask)
  * [MarketWithdraw](#marketwithdraw)
  * [MessagerGetMessage](#messagergetmessage)
  * [MessagerPushMessage](#messagerpushmessage)
  * [MessagerWaitMessage](#messagerwaitmessage)
  * [NetAddrsListen](#netaddrslisten)
  * [OfflineDealImport](#offlinedealimport)
  * [PaychVoucherList](#paychvoucherlist)
  * [PiecesGetCIDInfo](#piecesgetcidinfo)
  * [PiecesGetPieceInfo](#piecesgetpieceinfo)
  * [PiecesListCidInfos](#pieceslistcidinfos)
  * [PiecesListPieces](#pieceslistpieces)
  * [RemovePieceStorage](#removepiecestorage)
  * [ResponseMarketEvent](#responsemarketevent)
  * [SectorGetExpectedSealDuration](#sectorgetexpectedsealduration)
  * [SectorSetExpectedSealDuration](#sectorsetexpectedsealduration)
  * [UpdateDealOnPacking](#updatedealonpacking)
  * [UpdateDealStatus](#updatedealstatus)
  * [UpdateStorageDealStatus](#updatestoragedealstatus)
  * [Version](#version)

## Market

### ActorExist


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `true`

### ActorList


Perms: read

Inputs: `[]`

Response:
```json
[
  {
    "Addr": "f01234",
    "Account": "string value"
  }
]
```

### ActorSectorSize


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `34359738368`

### AddFsPieceStorage


Perms: admin

Inputs:
```json
[
  "string value",
  "string value",
  true
]
```

Response: `{}`

### AddS3PieceStorage


Perms: admin

Inputs:
```json
[
  "string value",
  "string value",
  "string value",
  "string value",
  "string value",
  "string value",
  "string value",
  true
]
```

Response: `{}`

### AssignUnPackedDeals


Perms: write

Inputs:
```json
[
  {
    "Miner": 1000,
    "Number": 9
  },
  34359738368,
  {
    "MaxPiece": 0,
    "MaxPieceSize": 0,
    "MinPiece": 0,
    "MinPieceSize": 0,
    "MinUsedSpace": 0,
    "StartEpoch": 0,
    "EndEpoch": 0
  }
]
```

Response:
```json
[
  {
    "PieceCID": null,
    "PieceSize": 0,
    "VerifiedDeal": false,
    "Client": "\u003cempty\u003e",
    "Provider": "\u003cempty\u003e",
    "Label": "",
    "StartEpoch": 0,
    "EndEpoch": 0,
    "StoragePricePerEpoch": "0",
    "ProviderCollateral": "0",
    "ClientCollateral": "0",
    "Offset": 1032,
    "Length": 1032,
    "PayloadSize": 0,
    "DealID": 0,
    "TotalStorageFee": "0",
    "FastRetrieval": false,
    "PublishCid": null
  }
]
```

### DagstoreGC
DagstoreGC runs garbage collection on the DAG store.


Perms: admin

Inputs: `[]`

Response:
```json
[
  {
    "Key": "string value",
    "Success": true,
    "Error": "string value"
  }
]
```

### DagstoreInitializeAll
DagstoreInitializeAll initializes all uninitialized shards in bulk,
according to the policy passed in the parameters.

It is recommended to set a maximum concurrency to avoid extreme
IO pressure if the storage subsystem has a large amount of deals.

It returns a stream of events to report progress.


Perms: admin

Inputs:
```json
[
  {
    "MaxConcurrency": 0,
    "IncludeSealed": false
  }
]
```

Response:
```json
{
  "Key": "string value",
  "Event": "string value",
  "Success": true,
  "Error": "string value",
  "Total": 123,
  "Current": 123
}
```

### DagstoreInitializeShard
DagstoreInitializeShard initializes an uninitialized shard.

Initialization consists of fetching the shard's data (deal payload) from
the storage subsystem, generating an index, and persisting the index
to facilitate later retrievals, and/or to publish to external sources.

This operation is intended to complement the initial migration. The
migration registers a shard for every unique piece CID, with lazy
initialization. Thus, shards are not initialized immediately to avoid
IO activity competing with proving. Instead, shard are initialized
when first accessed. This method forces the initialization of a shard by
accessing it and immediately releasing it. This is useful to warm up the
cache to facilitate subsequent retrievals, and to generate the indexes
to publish them externally.

This operation fails if the shard is not in ShardStateNew state.
It blocks until initialization finishes.


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response: `{}`

### DagstoreInitializeStorage
DagstoreInitializeStorage initializes all pieces in specify storage


Perms: admin

Inputs:
```json
[
  "string value",
  {
    "MaxConcurrency": 0,
    "IncludeSealed": false
  }
]
```

Response:
```json
{
  "Key": "string value",
  "Event": "string value",
  "Success": true,
  "Error": "string value",
  "Total": 123,
  "Current": 123
}
```

### DagstoreListShards
DagstoreListShards returns information about all shards known to the
DAG store. Only available on nodes running the markets subsystem.


Perms: admin

Inputs: `[]`

Response:
```json
[
  {
    "Key": "string value",
    "State": "string value",
    "Error": "string value"
  }
]
```

### DagstoreRecoverShard
DagstoreRecoverShard attempts to recover a failed shard.

This operation fails if the shard is not in ShardStateErrored state.
It blocks until recovery finishes. If recovery failed, it returns the
error.


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response: `{}`

### DealsConsiderOfflineRetrievalDeals


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `true`

### DealsConsiderOfflineStorageDeals


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `true`

### DealsConsiderOnlineRetrievalDeals


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `true`

### DealsConsiderOnlineStorageDeals


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `true`

### DealsConsiderUnverifiedStorageDeals


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `true`

### DealsConsiderVerifiedStorageDeals


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `true`

### DealsImportData


Perms: admin

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "string value"
]
```

Response: `{}`

### DealsMaxProviderCollateralMultiplier


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `42`

### DealsMaxPublishFee


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `"0 FIL"`

### DealsMaxStartDelay


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `60000000000`

### DealsPieceCidBlocklist


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
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

### DealsPublishMsgPeriod


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `60000000000`

### DealsSetConsiderOfflineRetrievalDeals


Perms: write

Inputs:
```json
[
  "f01234",
  true
]
```

Response: `{}`

### DealsSetConsiderOfflineStorageDeals


Perms: write

Inputs:
```json
[
  "f01234",
  true
]
```

Response: `{}`

### DealsSetConsiderOnlineRetrievalDeals


Perms: write

Inputs:
```json
[
  "f01234",
  true
]
```

Response: `{}`

### DealsSetConsiderOnlineStorageDeals


Perms: write

Inputs:
```json
[
  "f01234",
  true
]
```

Response: `{}`

### DealsSetConsiderUnverifiedStorageDeals


Perms: write

Inputs:
```json
[
  "f01234",
  true
]
```

Response: `{}`

### DealsSetConsiderVerifiedStorageDeals


Perms: write

Inputs:
```json
[
  "f01234",
  true
]
```

Response: `{}`

### DealsSetMaxProviderCollateralMultiplier


Perms: write

Inputs:
```json
[
  "f01234",
  42
]
```

Response: `{}`

### DealsSetMaxPublishFee


Perms: write

Inputs:
```json
[
  "f01234",
  "0 FIL"
]
```

Response: `{}`

### DealsSetMaxStartDelay


Perms: write

Inputs:
```json
[
  "f01234",
  60000000000
]
```

Response: `{}`

### DealsSetPieceCidBlocklist


Perms: write

Inputs:
```json
[
  "f01234",
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
  ]
]
```

Response: `{}`

### DealsSetPublishMsgPeriod


Perms: write

Inputs:
```json
[
  "f01234",
  60000000000
]
```

Response: `{}`

### GetDeals


Perms: read

Inputs:
```json
[
  "f01234",
  123,
  123
]
```

Response:
```json
[
  {
    "DealID": 0,
    "SectorID": 0,
    "Offset": 0,
    "Length": 0,
    "Proposal": {
      "PieceCID": null,
      "PieceSize": 0,
      "VerifiedDeal": false,
      "Client": "\u003cempty\u003e",
      "Provider": "\u003cempty\u003e",
      "Label": "",
      "StartEpoch": 0,
      "EndEpoch": 0,
      "StoragePricePerEpoch": "0",
      "ProviderCollateral": "0",
      "ClientCollateral": "0"
    },
    "ClientSignature": {
      "Type": 0,
      "Data": null
    },
    "TransferType": "",
    "Root": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "PublishCid": null,
    "FastRetrieval": false,
    "Status": "Undefine"
  }
]
```

### GetRetrievalDealStatistic
GetRetrievalDealStatistic get retrieval deal statistic information
todo address undefined is invalid, it is currently not possible to directly associate an order with a miner


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
  "DealsStatus": null
}
```

### GetStorageDealStatistic
GetStorageDealStatistic get storage deal statistic information
if set miner address to address.Undef, return all storage deal info


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
  "DealsStatus": null
}
```

### GetUnPackedDeals


Perms: read

Inputs:
```json
[
  "f01234",
  {
    "MaxPiece": 0,
    "MaxPieceSize": 0,
    "MinPiece": 0,
    "MinPieceSize": 0,
    "MinUsedSpace": 0,
    "StartEpoch": 0,
    "EndEpoch": 0
  }
]
```

Response:
```json
[
  {
    "PieceCID": null,
    "PieceSize": 0,
    "VerifiedDeal": false,
    "Client": "\u003cempty\u003e",
    "Provider": "\u003cempty\u003e",
    "Label": "",
    "StartEpoch": 0,
    "EndEpoch": 0,
    "StoragePricePerEpoch": "0",
    "ProviderCollateral": "0",
    "ClientCollateral": "0",
    "Offset": 1032,
    "Length": 1032,
    "PayloadSize": 0,
    "DealID": 0,
    "TotalStorageFee": "0",
    "FastRetrieval": false,
    "PublishCid": null
  }
]
```

### ID


Perms: read

Inputs: `[]`

Response: `"12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf"`

### ImportV1Data


Perms: write

Inputs:
```json
[
  "string value"
]
```

Response: `{}`

### ListPieceStorageInfos


Perms: read

Inputs: `[]`

Response:
```json
{
  "FsStorage": null,
  "S3Storage": null
}
```

### ListenMarketEvent


Perms: read

Inputs:
```json
[
  {
    "Miner": "f01234"
  }
]
```

Response:
```json
{
  "Id": "00000000-0000-0000-0000-000000000000",
  "Method": "string value",
  "Payload": "Ynl0ZSBhcnJheQ=="
}
```

### MarkDealsAsPacking


Perms: write

Inputs:
```json
[
  "f01234",
  [
    5432
  ]
]
```

Response: `{}`

### MarketAddBalance


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
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### MarketCancelDataTransfer
MarketCancelDataTransfer cancels a data transfer with the given transfer ID and other peer


Perms: write

Inputs:
```json
[
  3,
  "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
  true
]
```

Response: `{}`

### MarketDataTransferPath


Perms: admin

Inputs:
```json
[
  "f01234"
]
```

Response: `"string value"`

### MarketDataTransferUpdates


Perms: write

Inputs: `[]`

Response:
```json
{
  "TransferID": 0,
  "Status": 1,
  "BaseCID": null,
  "IsInitiator": false,
  "IsSender": false,
  "Voucher": "string value",
  "Message": "string value",
  "OtherPeer": "",
  "Transferred": 42,
  "Stages": {
    "Stages": [
      {
        "Name": "string value",
        "Description": "string value",
        "CreatedTime": "0001-01-01T00:00:00Z",
        "UpdatedTime": "0001-01-01T00:00:00Z",
        "Logs": [
          {
            "Log": "string value",
            "UpdatedTime": "0001-01-01T00:00:00Z"
          }
        ]
      }
    ]
  }
}
```

### MarketGetAsk


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
  "Ask": {
    "Price": "0",
    "VerifiedPrice": "0",
    "MinPieceSize": 0,
    "MaxPieceSize": 0,
    "Miner": "f01234",
    "Timestamp": 10101,
    "Expiry": 10101,
    "SeqNo": 0
  },
  "Signature": {
    "Type": 2,
    "Data": "Ynl0ZSBhcnJheQ=="
  },
  "CreatedAt": 0,
  "UpdatedAt": 0
}
```

### MarketGetDealUpdates


Perms: read

Inputs: `[]`

Response:
```json
{
  "Proposal": {
    "PieceCID": null,
    "PieceSize": 0,
    "VerifiedDeal": false,
    "Client": "\u003cempty\u003e",
    "Provider": "\u003cempty\u003e",
    "Label": "",
    "StartEpoch": 0,
    "EndEpoch": 0,
    "StoragePricePerEpoch": "\u003cnil\u003e",
    "ProviderCollateral": "\u003cnil\u003e",
    "ClientCollateral": "\u003cnil\u003e"
  },
  "ClientSignature": {
    "Type": 0,
    "Data": null
  },
  "ProposalCid": null,
  "AddFundsCid": null,
  "PublishCid": null,
  "Miner": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
  "Client": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
  "State": 42,
  "PiecePath": "",
  "PayloadSize": 0,
  "MetadataPath": "",
  "SlashEpoch": 0,
  "FastRetrieval": false,
  "Message": "string value",
  "FundsReserved": "\u003cnil\u003e",
  "Ref": {
    "TransferType": "",
    "Root": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "PieceCid": null,
    "PieceSize": 0,
    "RawBlockSize": 0
  },
  "AvailableForRetrieval": false,
  "DealID": 0,
  "CreationTime": "0001-01-01T00:00:00Z",
  "TransferChannelId": null,
  "SectorNumber": 0,
  "Offset": 1032,
  "PieceStatus": "",
  "InboundCAR": "",
  "CreatedAt": 0,
  "UpdatedAt": 0
}
```

### MarketGetReserved


Perms: sign

Inputs:
```json
[
  "f01234"
]
```

Response: `"0"`

### MarketGetRetrievalAsk


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
  "PricePerByte": "0",
  "UnsealPrice": "0",
  "PaymentInterval": 0,
  "PaymentIntervalIncrease": 0
}
```

### MarketImportDealData


Perms: write

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "string value"
]
```

Response: `{}`

### MarketImportPublishedDeal


Perms: write

Inputs:
```json
[
  {
    "Proposal": {
      "PieceCID": null,
      "PieceSize": 0,
      "VerifiedDeal": false,
      "Client": "\u003cempty\u003e",
      "Provider": "\u003cempty\u003e",
      "Label": "",
      "StartEpoch": 0,
      "EndEpoch": 0,
      "StoragePricePerEpoch": "\u003cnil\u003e",
      "ProviderCollateral": "\u003cnil\u003e",
      "ClientCollateral": "\u003cnil\u003e"
    },
    "ClientSignature": {
      "Type": 0,
      "Data": null
    },
    "ProposalCid": null,
    "AddFundsCid": null,
    "PublishCid": null,
    "Miner": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "Client": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "State": 42,
    "PiecePath": "",
    "PayloadSize": 0,
    "MetadataPath": "",
    "SlashEpoch": 0,
    "FastRetrieval": false,
    "Message": "string value",
    "FundsReserved": "\u003cnil\u003e",
    "Ref": {
      "TransferType": "",
      "Root": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      "PieceCid": null,
      "PieceSize": 0,
      "RawBlockSize": 0
    },
    "AvailableForRetrieval": false,
    "DealID": 0,
    "CreationTime": "0001-01-01T00:00:00Z",
    "TransferChannelId": null,
    "SectorNumber": 0,
    "Offset": 1032,
    "PieceStatus": "",
    "InboundCAR": "",
    "CreatedAt": 0,
    "UpdatedAt": 0
  }
]
```

Response: `{}`

### MarketListAsk


Perms: read

Inputs: `[]`

Response:
```json
[
  {
    "Ask": {
      "Price": "0",
      "VerifiedPrice": "0",
      "MinPieceSize": 0,
      "MaxPieceSize": 0,
      "Miner": "f01234",
      "Timestamp": 10101,
      "Expiry": 10101,
      "SeqNo": 0
    },
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "CreatedAt": 0,
    "UpdatedAt": 0
  }
]
```

### MarketListDataTransfers


Perms: write

Inputs: `[]`

Response:
```json
[
  {
    "TransferID": 0,
    "Status": 1,
    "BaseCID": null,
    "IsInitiator": false,
    "IsSender": false,
    "Voucher": "string value",
    "Message": "string value",
    "OtherPeer": "",
    "Transferred": 42,
    "Stages": {
      "Stages": [
        {
          "Name": "string value",
          "Description": "string value",
          "CreatedTime": "0001-01-01T00:00:00Z",
          "UpdatedTime": "0001-01-01T00:00:00Z",
          "Logs": [
            {
              "Log": "string value",
              "UpdatedTime": "0001-01-01T00:00:00Z"
            }
          ]
        }
      ]
    }
  }
]
```

### MarketListDeals


Perms: read

Inputs:
```json
[
  [
    "f01234"
  ]
]
```

Response:
```json
[
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
]
```

### MarketListIncompleteDeals


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
  {
    "Proposal": {
      "PieceCID": null,
      "PieceSize": 0,
      "VerifiedDeal": false,
      "Client": "\u003cempty\u003e",
      "Provider": "\u003cempty\u003e",
      "Label": "",
      "StartEpoch": 0,
      "EndEpoch": 0,
      "StoragePricePerEpoch": "0",
      "ProviderCollateral": "0",
      "ClientCollateral": "0"
    },
    "ClientSignature": {
      "Type": 0,
      "Data": null
    },
    "ProposalCid": null,
    "AddFundsCid": null,
    "PublishCid": null,
    "Miner": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "Client": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "State": 42,
    "PiecePath": "",
    "PayloadSize": 0,
    "MetadataPath": "",
    "SlashEpoch": 0,
    "FastRetrieval": false,
    "Message": "string value",
    "FundsReserved": "0",
    "Ref": {
      "TransferType": "",
      "Root": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      "PieceCid": null,
      "PieceSize": 0,
      "RawBlockSize": 0
    },
    "AvailableForRetrieval": false,
    "DealID": 0,
    "CreationTime": "0001-01-01T00:00:00Z",
    "TransferChannelId": null,
    "SectorNumber": 0,
    "Offset": 1032,
    "PieceStatus": "",
    "InboundCAR": "",
    "CreatedAt": 0,
    "UpdatedAt": 0
  }
]
```

### MarketListRetrievalAsk


Perms: read

Inputs: `[]`

Response:
```json
[
  {
    "Miner": "f01234",
    "PricePerByte": "0",
    "UnsealPrice": "0",
    "PaymentInterval": 0,
    "PaymentIntervalIncrease": 0,
    "CreatedAt": 0,
    "UpdatedAt": 0
  }
]
```

### MarketListRetrievalDeals


Perms: read

Inputs: `[]`

Response:
```json
[
  {
    "PayloadCID": null,
    "ID": 0,
    "Selector": null,
    "PieceCID": null,
    "PricePerByte": "0",
    "PaymentInterval": 0,
    "PaymentIntervalIncrease": 0,
    "UnsealPrice": "0",
    "StoreID": 0,
    "SelStorageProposalCid": null,
    "ChannelID": null,
    "Status": 0,
    "Receiver": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "TotalSent": 0,
    "FundsReceived": "0",
    "Message": "string value",
    "CurrentInterval": 0,
    "LegacyProtocol": false,
    "CreatedAt": 0,
    "UpdatedAt": 0
  }
]
```

### MarketMaxBalanceAddFee


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `"0 FIL"`

### MarketMaxDealsPerPublishMsg


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `42`

### MarketPendingDeals


Perms: write

Inputs: `[]`

Response:
```json
[
  {
    "Deals": [
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
        "ClientSignature": {
          "Type": 0,
          "Data": null
        }
      }
    ],
    "PublishPeriodStart": "0001-01-01T00:00:00Z",
    "PublishPeriod": 0
  }
]
```

### MarketPublishPendingDeals


Perms: admin

Inputs: `[]`

Response: `{}`

### MarketReleaseFunds


Perms: sign

Inputs:
```json
[
  "f01234",
  "0"
]
```

Response: `{}`

### MarketReserveFunds


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
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### MarketRestartDataTransfer
MarketRestartDataTransfer attempts to restart a data transfer with the given transfer ID and other peer


Perms: write

Inputs:
```json
[
  3,
  "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
  true
]
```

Response: `{}`

### MarketSetAsk


Perms: admin

Inputs:
```json
[
  "f01234",
  "0",
  "0",
  10101,
  1032,
  1032
]
```

Response: `{}`

### MarketSetDataTransferPath


Perms: admin

Inputs:
```json
[
  "f01234",
  "string value"
]
```

Response: `{}`

### MarketSetMaxBalanceAddFee


Perms: write

Inputs:
```json
[
  "f01234",
  "0 FIL"
]
```

Response: `{}`

### MarketSetMaxDealsPerPublishMsg


Perms: write

Inputs:
```json
[
  "f01234",
  42
]
```

Response: `{}`

### MarketSetRetrievalAsk


Perms: admin

Inputs:
```json
[
  "f01234",
  {
    "PricePerByte": "0",
    "UnsealPrice": "0",
    "PaymentInterval": 0,
    "PaymentIntervalIncrease": 0
  }
]
```

Response: `{}`

### MarketWithdraw


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
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### MessagerGetMessage


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

### MessagerPushMessage


Perms: write

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
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### MessagerWaitMessage
messager


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

### OfflineDealImport


Perms: admin

Inputs:
```json
[
  {
    "Proposal": {
      "PieceCID": null,
      "PieceSize": 0,
      "VerifiedDeal": false,
      "Client": "\u003cempty\u003e",
      "Provider": "\u003cempty\u003e",
      "Label": "",
      "StartEpoch": 0,
      "EndEpoch": 0,
      "StoragePricePerEpoch": "\u003cnil\u003e",
      "ProviderCollateral": "\u003cnil\u003e",
      "ClientCollateral": "\u003cnil\u003e"
    },
    "ClientSignature": {
      "Type": 0,
      "Data": null
    },
    "ProposalCid": null,
    "AddFundsCid": null,
    "PublishCid": null,
    "Miner": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "Client": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "State": 42,
    "PiecePath": "",
    "PayloadSize": 0,
    "MetadataPath": "",
    "SlashEpoch": 0,
    "FastRetrieval": false,
    "Message": "string value",
    "FundsReserved": "\u003cnil\u003e",
    "Ref": {
      "TransferType": "",
      "Root": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      "PieceCid": null,
      "PieceSize": 0,
      "RawBlockSize": 0
    },
    "AvailableForRetrieval": false,
    "DealID": 0,
    "CreationTime": "0001-01-01T00:00:00Z",
    "TransferChannelId": null,
    "SectorNumber": 0,
    "Offset": 1032,
    "PieceStatus": "",
    "InboundCAR": "",
    "CreatedAt": 0,
    "UpdatedAt": 0
  }
]
```

Response: `{}`

### PaychVoucherList
Paych


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

### PiecesGetCIDInfo


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
  "CID": null,
  "PieceBlockLocations": null
}
```

### PiecesGetPieceInfo


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
  "PieceCID": null,
  "Deals": [
    {
      "DealID": 0,
      "SectorID": 0,
      "Offset": 1032,
      "Length": 1032
    }
  ]
}
```

### PiecesListCidInfos


Perms: read

Inputs: `[]`

Response:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

### PiecesListPieces


Perms: read

Inputs: `[]`

Response:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

### RemovePieceStorage


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response: `{}`

### ResponseMarketEvent
market event


Perms: read

Inputs:
```json
[
  {
    "Id": "00000000-0000-0000-0000-000000000000",
    "Payload": "Ynl0ZSBhcnJheQ==",
    "Error": "string value"
  }
]
```

Response: `{}`

### SectorGetExpectedSealDuration
SectorGetExpectedSealDuration gets the time that a newly-created sector
waits for more deals before it starts sealing


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `60000000000`

### SectorSetExpectedSealDuration
SectorSetExpectedSealDuration sets the expected time for a sector to seal


Perms: write

Inputs:
```json
[
  "f01234",
  60000000000
]
```

Response: `{}`

### UpdateDealOnPacking


Perms: write

Inputs:
```json
[
  "f01234",
  5432,
  9,
  1032
]
```

Response: `{}`

### UpdateDealStatus


Perms: write

Inputs:
```json
[
  "f01234",
  5432,
  "Undefine"
]
```

Response: `{}`

### UpdateStorageDealStatus


Perms: write

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  42,
  "Undefine"
]
```

Response: `{}`

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


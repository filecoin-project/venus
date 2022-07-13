# Groups

* [Market](#Market)
  * [ActorExist](#ActorExist)
  * [ActorList](#ActorList)
  * [ActorSectorSize](#ActorSectorSize)
  * [AddFsPieceStorage](#AddFsPieceStorage)
  * [AddS3PieceStorage](#AddS3PieceStorage)
  * [AssignUnPackedDeals](#AssignUnPackedDeals)
  * [DagstoreGC](#DagstoreGC)
  * [DagstoreInitializeAll](#DagstoreInitializeAll)
  * [DagstoreInitializeShard](#DagstoreInitializeShard)
  * [DagstoreInitializeStorage](#DagstoreInitializeStorage)
  * [DagstoreListShards](#DagstoreListShards)
  * [DagstoreRecoverShard](#DagstoreRecoverShard)
  * [DealsConsiderOfflineRetrievalDeals](#DealsConsiderOfflineRetrievalDeals)
  * [DealsConsiderOfflineStorageDeals](#DealsConsiderOfflineStorageDeals)
  * [DealsConsiderOnlineRetrievalDeals](#DealsConsiderOnlineRetrievalDeals)
  * [DealsConsiderOnlineStorageDeals](#DealsConsiderOnlineStorageDeals)
  * [DealsConsiderUnverifiedStorageDeals](#DealsConsiderUnverifiedStorageDeals)
  * [DealsConsiderVerifiedStorageDeals](#DealsConsiderVerifiedStorageDeals)
  * [DealsImportData](#DealsImportData)
  * [DealsPieceCidBlocklist](#DealsPieceCidBlocklist)
  * [DealsSetConsiderOfflineRetrievalDeals](#DealsSetConsiderOfflineRetrievalDeals)
  * [DealsSetConsiderOfflineStorageDeals](#DealsSetConsiderOfflineStorageDeals)
  * [DealsSetConsiderOnlineRetrievalDeals](#DealsSetConsiderOnlineRetrievalDeals)
  * [DealsSetConsiderOnlineStorageDeals](#DealsSetConsiderOnlineStorageDeals)
  * [DealsSetConsiderUnverifiedStorageDeals](#DealsSetConsiderUnverifiedStorageDeals)
  * [DealsSetConsiderVerifiedStorageDeals](#DealsSetConsiderVerifiedStorageDeals)
  * [DealsSetPieceCidBlocklist](#DealsSetPieceCidBlocklist)
  * [GetDeals](#GetDeals)
  * [GetPieceStorages](#GetPieceStorages)
  * [GetReadUrl](#GetReadUrl)
  * [GetUnPackedDeals](#GetUnPackedDeals)
  * [GetWriteUrl](#GetWriteUrl)
  * [ID](#ID)
  * [ImportV1Data](#ImportV1Data)
  * [ListenMarketEvent](#ListenMarketEvent)
  * [MarkDealsAsPacking](#MarkDealsAsPacking)
  * [MarketAddBalance](#MarketAddBalance)
  * [MarketCancelDataTransfer](#MarketCancelDataTransfer)
  * [MarketDataTransferUpdates](#MarketDataTransferUpdates)
  * [MarketGetAsk](#MarketGetAsk)
  * [MarketGetDealUpdates](#MarketGetDealUpdates)
  * [MarketGetReserved](#MarketGetReserved)
  * [MarketGetRetrievalAsk](#MarketGetRetrievalAsk)
  * [MarketImportDealData](#MarketImportDealData)
  * [MarketImportPublishedDeal](#MarketImportPublishedDeal)
  * [MarketListAsk](#MarketListAsk)
  * [MarketListDataTransfers](#MarketListDataTransfers)
  * [MarketListDeals](#MarketListDeals)
  * [MarketListIncompleteDeals](#MarketListIncompleteDeals)
  * [MarketListRetrievalAsk](#MarketListRetrievalAsk)
  * [MarketListRetrievalDeals](#MarketListRetrievalDeals)
  * [MarketPendingDeals](#MarketPendingDeals)
  * [MarketPublishPendingDeals](#MarketPublishPendingDeals)
  * [MarketReleaseFunds](#MarketReleaseFunds)
  * [MarketReserveFunds](#MarketReserveFunds)
  * [MarketRestartDataTransfer](#MarketRestartDataTransfer)
  * [MarketSetAsk](#MarketSetAsk)
  * [MarketSetRetrievalAsk](#MarketSetRetrievalAsk)
  * [MarketWithdraw](#MarketWithdraw)
  * [MessagerGetMessage](#MessagerGetMessage)
  * [MessagerPushMessage](#MessagerPushMessage)
  * [MessagerWaitMessage](#MessagerWaitMessage)
  * [NetAddrsListen](#NetAddrsListen)
  * [PaychVoucherList](#PaychVoucherList)
  * [PiecesGetCIDInfo](#PiecesGetCIDInfo)
  * [PiecesGetPieceInfo](#PiecesGetPieceInfo)
  * [PiecesListCidInfos](#PiecesListCidInfos)
  * [PiecesListPieces](#PiecesListPieces)
  * [RemovePieceStorage](#RemovePieceStorage)
  * [ResponseMarketEvent](#ResponseMarketEvent)
  * [SectorGetSealDelay](#SectorGetSealDelay)
  * [SectorSetExpectedSealDuration](#SectorSetExpectedSealDuration)
  * [UpdateDealOnPacking](#UpdateDealOnPacking)
  * [UpdateDealStatus](#UpdateDealStatus)
  * [UpdateStorageDealStatus](#UpdateStorageDealStatus)

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
  true,
  "string value",
  "string value"
]
```

Response: `{}`

### AddS3PieceStorage


Perms: admin

Inputs:
```json
[
  true,
  "string value",
  "string value",
  "string value",
  "string value",
  "string value"
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
    "MaxPiece": 123,
    "MaxPieceSize": 42,
    "MinPiece": 123,
    "MinPieceSize": 42,
    "MinUsedSpace": 42,
    "StartEpoch": 10101,
    "EndEpoch": 10101
  }
]
```

Response:
```json
[
  {
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
    "ClientCollateral": "0",
    "Offset": 1032,
    "Length": 1032,
    "PayloadSize": 42,
    "DealID": 5432,
    "TotalStorageFee": "0",
    "FastRetrieval": true,
    "PublishCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
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
    "MaxConcurrency": 123,
    "IncludeSealed": true
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
    "MaxConcurrency": 123,
    "IncludeSealed": true
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


Perms: admin

Inputs: `[]`

Response: `true`

### DealsConsiderOfflineStorageDeals


Perms: admin

Inputs: `[]`

Response: `true`

### DealsConsiderOnlineRetrievalDeals


Perms: admin

Inputs: `[]`

Response: `true`

### DealsConsiderOnlineStorageDeals


Perms: admin

Inputs: `[]`

Response: `true`

### DealsConsiderUnverifiedStorageDeals


Perms: admin

Inputs: `[]`

Response: `true`

### DealsConsiderVerifiedStorageDeals


Perms: admin

Inputs: `[]`

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

### DealsPieceCidBlocklist


Perms: admin

Inputs: `[]`

Response:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

### DealsSetConsiderOfflineRetrievalDeals


Perms: admin

Inputs:
```json
[
  true
]
```

Response: `{}`

### DealsSetConsiderOfflineStorageDeals


Perms: admin

Inputs:
```json
[
  true
]
```

Response: `{}`

### DealsSetConsiderOnlineRetrievalDeals


Perms: admin

Inputs:
```json
[
  true
]
```

Response: `{}`

### DealsSetConsiderOnlineStorageDeals


Perms: admin

Inputs:
```json
[
  true
]
```

Response: `{}`

### DealsSetConsiderUnverifiedStorageDeals


Perms: admin

Inputs:
```json
[
  true
]
```

Response: `{}`

### DealsSetConsiderVerifiedStorageDeals


Perms: admin

Inputs:
```json
[
  true
]
```

Response: `{}`

### DealsSetPieceCidBlocklist


Perms: admin

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
  ]
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
    "DealID": 5432,
    "SectorID": 9,
    "Offset": 1032,
    "Length": 1032,
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
    "ClientSignature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "TransferType": "string value",
    "Root": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "PublishCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "FastRetrieval": true,
    "Status": "Undefine"
  }
]
```

### GetPieceStorages


Perms: read

Inputs: `[]`

Response:
```json
{
  "FsStorage": [
    {
      "Path": "string value",
      "Name": "string value",
      "ReadOnly": true
    }
  ],
  "S3Storage": [
    {
      "Name": "string value",
      "ReadOnly": true,
      "EndPoint": "string value"
    }
  ]
}
```

### GetReadUrl
piece storage


Perms: read

Inputs:
```json
[
  "string value"
]
```

Response: `"string value"`

### GetUnPackedDeals


Perms: read

Inputs:
```json
[
  "f01234",
  {
    "MaxPiece": 123,
    "MaxPieceSize": 42,
    "MinPiece": 123,
    "MinPieceSize": 42,
    "MinUsedSpace": 42,
    "StartEpoch": 10101,
    "EndEpoch": 10101
  }
]
```

Response:
```json
[
  {
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
    "ClientCollateral": "0",
    "Offset": 1032,
    "Length": 1032,
    "PayloadSize": 42,
    "DealID": 5432,
    "TotalStorageFee": "0",
    "FastRetrieval": true,
    "PublishCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    }
  }
]
```

### GetWriteUrl


Perms: read

Inputs:
```json
[
  "string value"
]
```

Response: `"string value"`

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
  "Id": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
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

### MarketDataTransferUpdates


Perms: write

Inputs: `[]`

Response:
```json
{
  "TransferID": 3,
  "Status": 1,
  "BaseCID": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "IsInitiator": true,
  "IsSender": true,
  "Voucher": "string value",
  "Message": "string value",
  "OtherPeer": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
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
    "MinPieceSize": 1032,
    "MaxPieceSize": 1032,
    "Miner": "f01234",
    "Timestamp": 10101,
    "Expiry": 10101,
    "SeqNo": 42
  },
  "Signature": {
    "Type": 2,
    "Data": "Ynl0ZSBhcnJheQ=="
  }
}
```

### MarketGetDealUpdates


Perms: read

Inputs: `[]`

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
  "ClientSignature": {
    "Type": 2,
    "Data": "Ynl0ZSBhcnJheQ=="
  },
  "ProposalCid": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "AddFundsCid": null,
  "PublishCid": null,
  "Miner": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
  "Client": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
  "State": 42,
  "PiecePath": "/some/path",
  "PayloadSize": 42,
  "MetadataPath": "/some/path",
  "SlashEpoch": 10101,
  "FastRetrieval": true,
  "Message": "string value",
  "FundsReserved": "0",
  "Ref": {
    "TransferType": "string value",
    "Root": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "PieceCid": null,
    "PieceSize": 1024,
    "RawBlockSize": 42
  },
  "AvailableForRetrieval": true,
  "DealID": 5432,
  "CreationTime": "0001-01-01T00:00:00Z",
  "TransferChannelId": {
    "Initiator": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "Responder": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "ID": 3
  },
  "SectorNumber": 9,
  "Offset": 1032,
  "PieceStatus": "Undefine",
  "InboundCAR": "string value"
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
  "PaymentInterval": 42,
  "PaymentIntervalIncrease": 42
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
    "ClientSignature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "ProposalCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "AddFundsCid": null,
    "PublishCid": null,
    "Miner": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "Client": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "State": 42,
    "PiecePath": "/some/path",
    "PayloadSize": 42,
    "MetadataPath": "/some/path",
    "SlashEpoch": 10101,
    "FastRetrieval": true,
    "Message": "string value",
    "FundsReserved": "0",
    "Ref": {
      "TransferType": "string value",
      "Root": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      "PieceCid": null,
      "PieceSize": 1024,
      "RawBlockSize": 42
    },
    "AvailableForRetrieval": true,
    "DealID": 5432,
    "CreationTime": "0001-01-01T00:00:00Z",
    "TransferChannelId": {
      "Initiator": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
      "Responder": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
      "ID": 3
    },
    "SectorNumber": 9,
    "Offset": 1032,
    "PieceStatus": "Undefine",
    "InboundCAR": "string value"
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
      "MinPieceSize": 1032,
      "MaxPieceSize": 1032,
      "Miner": "f01234",
      "Timestamp": 10101,
      "Expiry": 10101,
      "SeqNo": 42
    },
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    }
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
    "TransferID": 3,
    "Status": 1,
    "BaseCID": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "IsInitiator": true,
    "IsSender": true,
    "Voucher": "string value",
    "Message": "string value",
    "OtherPeer": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
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
      "SlashEpoch": 10101
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
    "ClientSignature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "ProposalCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "AddFundsCid": null,
    "PublishCid": null,
    "Miner": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "Client": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "State": 42,
    "PiecePath": "/some/path",
    "PayloadSize": 42,
    "MetadataPath": "/some/path",
    "SlashEpoch": 10101,
    "FastRetrieval": true,
    "Message": "string value",
    "FundsReserved": "0",
    "Ref": {
      "TransferType": "string value",
      "Root": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      "PieceCid": null,
      "PieceSize": 1024,
      "RawBlockSize": 42
    },
    "AvailableForRetrieval": true,
    "DealID": 5432,
    "CreationTime": "0001-01-01T00:00:00Z",
    "TransferChannelId": {
      "Initiator": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
      "Responder": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
      "ID": 3
    },
    "SectorNumber": 9,
    "Offset": 1032,
    "PieceStatus": "Undefine",
    "InboundCAR": "string value"
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
    "PaymentInterval": 42,
    "PaymentIntervalIncrease": 42
  }
]
```

### MarketListRetrievalDeals


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
    "PayloadCID": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "ID": 5,
    "Selector": {
      "Raw": "Ynl0ZSBhcnJheQ=="
    },
    "PieceCID": null,
    "PricePerByte": "0",
    "PaymentInterval": 42,
    "PaymentIntervalIncrease": 42,
    "UnsealPrice": "0",
    "StoreID": 42,
    "SelStorageProposalCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "ChannelID": {
      "Initiator": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
      "Responder": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
      "ID": 3
    },
    "Status": 0,
    "Receiver": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "TotalSent": 42,
    "FundsReceived": "0",
    "Message": "string value",
    "CurrentInterval": 42,
    "LegacyProtocol": true
  }
]
```

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
        "ClientSignature": {
          "Type": 2,
          "Data": "Ynl0ZSBhcnJheQ=="
        }
      }
    ],
    "PublishPeriodStart": "0001-01-01T00:00:00Z",
    "PublishPeriod": 60000000000
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

### MarketSetRetrievalAsk


Perms: admin

Inputs:
```json
[
  "f01234",
  {
    "PricePerByte": "0",
    "UnsealPrice": "0",
    "PaymentInterval": 42,
    "PaymentIntervalIncrease": 42
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

### MessagerPushMessage


Perms: write

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
    "GasUsed": 9
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
  "CID": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "PieceBlockLocations": [
    {
      "RelOffset": 42,
      "BlockSize": 42,
      "PieceCID": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      }
    }
  ]
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
  "PieceCID": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Deals": [
    {
      "DealID": 5432,
      "SectorID": 9,
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
    "Id": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
    "Payload": "Ynl0ZSBhcnJheQ==",
    "Error": "string value"
  }
]
```

Response: `{}`

### SectorGetSealDelay
SectorGetSealDelay gets the time that a newly-created sector
waits for more deals before it starts sealing


Perms: read

Inputs: `[]`

Response: `60000000000`

### SectorSetExpectedSealDuration
SectorSetExpectedSealDuration sets the expected time for a sector to seal


Perms: write

Inputs:
```json
[
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


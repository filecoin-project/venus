# Groups

* [MarketClient](#marketclient)
  * [ClientCalcCommP](#clientcalccommp)
  * [ClientCancelDataTransfer](#clientcanceldatatransfer)
  * [ClientCancelRetrievalDeal](#clientcancelretrievaldeal)
  * [ClientDataTransferUpdates](#clientdatatransferupdates)
  * [ClientDealPieceCID](#clientdealpiececid)
  * [ClientDealSize](#clientdealsize)
  * [ClientExport](#clientexport)
  * [ClientFindData](#clientfinddata)
  * [ClientGenCar](#clientgencar)
  * [ClientGetDealInfo](#clientgetdealinfo)
  * [ClientGetDealStatus](#clientgetdealstatus)
  * [ClientGetDealUpdates](#clientgetdealupdates)
  * [ClientGetRetrievalUpdates](#clientgetretrievalupdates)
  * [ClientHasLocal](#clienthaslocal)
  * [ClientImport](#clientimport)
  * [ClientListDataTransfers](#clientlistdatatransfers)
  * [ClientListDeals](#clientlistdeals)
  * [ClientListImports](#clientlistimports)
  * [ClientListRetrievals](#clientlistretrievals)
  * [ClientMinerQueryOffer](#clientminerqueryoffer)
  * [ClientQueryAsk](#clientqueryask)
  * [ClientRemoveImport](#clientremoveimport)
  * [ClientRestartDataTransfer](#clientrestartdatatransfer)
  * [ClientRetrieve](#clientretrieve)
  * [ClientRetrieveTryRestartInsufficientFunds](#clientretrievetryrestartinsufficientfunds)
  * [ClientRetrieveWait](#clientretrievewait)
  * [ClientStartDeal](#clientstartdeal)
  * [ClientStatelessDeal](#clientstatelessdeal)
  * [DefaultAddress](#defaultaddress)
  * [MarketAddBalance](#marketaddbalance)
  * [MarketGetReserved](#marketgetreserved)
  * [MarketReleaseFunds](#marketreleasefunds)
  * [MarketReserveFunds](#marketreservefunds)
  * [MarketWithdraw](#marketwithdraw)
  * [MessagerGetMessage](#messagergetmessage)
  * [MessagerPushMessage](#messagerpushmessage)
  * [MessagerWaitMessage](#messagerwaitmessage)
  * [Version](#version)

## MarketClient

### ClientCalcCommP
ClientCalcCommP calculates the CommP for a specified file


Perms: write

Inputs:
```json
[
  "string value"
]
```

Response:
```json
{
  "Root": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Size": 1024
}
```

### ClientCancelDataTransfer
ClientCancelDataTransfer cancels a data transfer with the given transfer ID and other peer


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

### ClientCancelRetrievalDeal
ClientCancelRetrievalDeal cancels an ongoing retrieval deal based on DealID


Perms: write

Inputs:
```json
[
  5
]
```

Response: `{}`

### ClientDataTransferUpdates


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

### ClientDealPieceCID
ClientCalcCommP calculates the CommP and data size of the specified CID


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
  "PayloadSize": 0,
  "PieceSize": 0,
  "PieceCID": null
}
```

### ClientDealSize
ClientDealSize calculates real deal data size


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
  "PayloadSize": 0,
  "PieceSize": 0
}
```

### ClientExport
ClientExport exports a file stored in the local filestore to a system file


Perms: admin

Inputs:
```json
[
  {
    "Root": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "DAGs": null,
    "FromLocalCAR": "",
    "DealID": 0
  },
  {
    "Path": "string value",
    "IsCAR": false
  }
]
```

Response: `{}`

### ClientFindData
ClientFindData identifies peers that have a certain file, and returns QueryOffers (one per peer).


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
[
  {
    "Err": "string value",
    "Root": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "Piece": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "Size": 42,
    "MinPrice": "0",
    "UnsealPrice": "0",
    "PricePerByte": "0",
    "PaymentInterval": 0,
    "PaymentIntervalIncrease": 0,
    "Miner": "f01234",
    "MinerPeer": {
      "Address": "\u003cempty\u003e",
      "ID": "",
      "PieceCID": null
    }
  }
]
```

### ClientGenCar
ClientGenCar generates a CAR file for the specified file.


Perms: write

Inputs:
```json
[
  {
    "Path": "string value",
    "IsCAR": false
  },
  "string value"
]
```

Response: `{}`

### ClientGetDealInfo
ClientGetDealInfo returns the latest information about a given deal.


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
  "ProposalCid": null,
  "State": 42,
  "Message": "string value",
  "DealStages": null,
  "Provider": "f01234",
  "DataRef": null,
  "PieceCID": null,
  "Size": 42,
  "PricePerEpoch": "0",
  "Duration": 42,
  "DealID": 0,
  "CreationTime": "0001-01-01T00:00:00Z",
  "Verified": true,
  "TransferChannelID": null,
  "DataTransfer": null
}
```

### ClientGetDealStatus
ClientGetDealStatus returns status given a code


Perms: read

Inputs:
```json
[
  42
]
```

Response: `"string value"`

### ClientGetDealUpdates
ClientGetDealUpdates returns the status of updated deals


Perms: write

Inputs: `[]`

Response:
```json
{
  "ProposalCid": null,
  "State": 42,
  "Message": "string value",
  "DealStages": null,
  "Provider": "f01234",
  "DataRef": null,
  "PieceCID": null,
  "Size": 42,
  "PricePerEpoch": "\u003cnil\u003e",
  "Duration": 42,
  "DealID": 0,
  "CreationTime": "0001-01-01T00:00:00Z",
  "Verified": true,
  "TransferChannelID": null,
  "DataTransfer": null
}
```

### ClientGetRetrievalUpdates
ClientGetRetrievalUpdates returns status of updated retrieval deals


Perms: write

Inputs: `[]`

Response:
```json
{
  "PayloadCID": null,
  "ID": 0,
  "PieceCID": null,
  "PricePerByte": "\u003cnil\u003e",
  "UnsealPrice": "\u003cnil\u003e",
  "Status": 0,
  "Message": "string value",
  "Provider": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
  "BytesReceived": 0,
  "BytesPaidFor": 0,
  "TotalPaid": "\u003cnil\u003e",
  "TransferChannelID": null,
  "DataTransfer": null,
  "Event": 5
}
```

### ClientHasLocal
ClientHasLocal indicates whether a certain CID is locally stored.


Perms: write

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

Response: `true`

### ClientImport
ClientImport imports file under the specified path into filestore.


Perms: admin

Inputs:
```json
[
  {
    "Path": "string value",
    "IsCAR": false
  }
]
```

Response:
```json
{
  "Root": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "ImportID": 0
}
```

### ClientListDataTransfers
ClientListTransfers returns the status of all ongoing transfers of data


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

### ClientListDeals
ClientListDeals returns information about the deals made by the local client.


Perms: write

Inputs: `[]`

Response:
```json
[
  {
    "ProposalCid": null,
    "State": 42,
    "Message": "string value",
    "DealStages": null,
    "Provider": "f01234",
    "DataRef": null,
    "PieceCID": null,
    "Size": 42,
    "PricePerEpoch": "0",
    "Duration": 42,
    "DealID": 0,
    "CreationTime": "0001-01-01T00:00:00Z",
    "Verified": true,
    "TransferChannelID": null,
    "DataTransfer": null
  }
]
```

### ClientListImports
ClientUnimport removes references to the specified file from filestore
ClientUnimport(path string)


Perms: write

Inputs: `[]`

Response:
```json
[
  {
    "Key": 1234,
    "Err": "string value",
    "Root": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "Source": "string value",
    "FilePath": "",
    "CARPath": ""
  }
]
```

### ClientListRetrievals


Perms: write

Inputs: `[]`

Response:
```json
[
  {
    "PayloadCID": null,
    "ID": 0,
    "PieceCID": null,
    "PricePerByte": "0",
    "UnsealPrice": "0",
    "Status": 0,
    "Message": "string value",
    "Provider": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "BytesReceived": 0,
    "BytesPaidFor": 0,
    "TotalPaid": "0",
    "TransferChannelID": null,
    "DataTransfer": null,
    "Event": 5
  }
]
```

### ClientMinerQueryOffer
ClientMinerQueryOffer returns a QueryOffer for the specific miner and file.


Perms: read

Inputs:
```json
[
  "f01234",
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
  "Err": "string value",
  "Root": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Piece": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Size": 42,
  "MinPrice": "\u003cnil\u003e",
  "UnsealPrice": "\u003cnil\u003e",
  "PricePerByte": "\u003cnil\u003e",
  "PaymentInterval": 0,
  "PaymentIntervalIncrease": 0,
  "Miner": "f01234",
  "MinerPeer": {
    "Address": "\u003cempty\u003e",
    "ID": "",
    "PieceCID": null
  }
}
```

### ClientQueryAsk
ClientQueryAsk returns a signed StorageAsk from the specified miner.


Perms: read

Inputs:
```json
[
  "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
  "f01234"
]
```

Response:
```json
{
  "Price": "0",
  "VerifiedPrice": "0",
  "MinPieceSize": 0,
  "MaxPieceSize": 0,
  "Miner": "f01234",
  "Timestamp": 10101,
  "Expiry": 10101,
  "SeqNo": 0
}
```

### ClientRemoveImport
ClientRemoveImport removes file import


Perms: admin

Inputs:
```json
[
  1234
]
```

Response: `{}`

### ClientRestartDataTransfer
ClientRestartDataTransfer attempts to restart a data transfer with the given transfer ID and other peer


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

### ClientRetrieve
ClientRetrieve initiates the retrieval of a file, as specified in the order.


Perms: admin

Inputs:
```json
[
  {
    "Root": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "Piece": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "DataSelector": null,
    "Size": 42,
    "Total": "0",
    "UnsealPrice": "\u003cnil\u003e",
    "PaymentInterval": 0,
    "PaymentIntervalIncrease": 0,
    "Client": "f01234",
    "Miner": "f01234",
    "MinerPeer": null
  }
]
```

Response:
```json
{
  "DealID": 0
}
```

### ClientRetrieveTryRestartInsufficientFunds
ClientRetrieveTryRestartInsufficientFunds attempts to restart stalled retrievals on a given payment channel
which are stuck due to insufficient funds


Perms: write

Inputs:
```json
[
  "f01234"
]
```

Response: `{}`

### ClientRetrieveWait
ClientRetrieveWait waits for retrieval to be complete


Perms: admin

Inputs:
```json
[
  5
]
```

Response: `{}`

### ClientStartDeal
ClientStartDeal proposes a deal with a miner.


Perms: admin

Inputs:
```json
[
  {
    "Data": {
      "TransferType": "",
      "Root": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      "PieceCid": null,
      "PieceSize": 0,
      "RawBlockSize": 0
    },
    "Wallet": "f01234",
    "Miner": "f01234",
    "EpochPrice": "0",
    "MinBlocksDuration": 0,
    "ProviderCollateral": "0",
    "DealStartEpoch": 0,
    "FastRetrieval": false,
    "VerifiedDeal": false
  }
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### ClientStatelessDeal
ClientStatelessDeal fire-and-forget-proposes an offline deal to a miner without subsequent tracking.


Perms: write

Inputs:
```json
[
  {
    "Data": {
      "TransferType": "",
      "Root": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      "PieceCid": null,
      "PieceSize": 0,
      "RawBlockSize": 0
    },
    "Wallet": "f01234",
    "Miner": "f01234",
    "EpochPrice": "0",
    "MinBlocksDuration": 0,
    "ProviderCollateral": "0",
    "DealStartEpoch": 0,
    "FastRetrieval": false,
    "VerifiedDeal": false
  }
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### DefaultAddress


Perms: read

Inputs: `[]`

Response: `"f01234"`

### MarketAddBalance


Perms: write

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

### MarketGetReserved


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `"0"`

### MarketReleaseFunds


Perms: write

Inputs:
```json
[
  "f01234",
  "0"
]
```

Response: `{}`

### MarketReserveFunds


Perms: write

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

### MarketWithdraw


Perms: write

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


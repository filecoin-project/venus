# Groups

* [Gateway](#gateway)
  * [Version](#version)
* [MarketClient](#marketclient)
  * [IsUnsealed](#isunsealed)
  * [ListMarketConnectionsState](#listmarketconnectionsstate)
  * [SectorsUnsealPiece](#sectorsunsealpiece)
* [MarketServiceProvider](#marketserviceprovider)
  * [ListenMarketEvent](#listenmarketevent)
  * [ResponseMarketEvent](#responsemarketevent)
* [ProofClient](#proofclient)
  * [ComputeProof](#computeproof)
  * [ListConnectedMiners](#listconnectedminers)
  * [ListMinerConnection](#listminerconnection)
* [ProofServiceProvider](#proofserviceprovider)
  * [ListenProofEvent](#listenproofevent)
  * [ResponseProofEvent](#responseproofevent)
* [WalletClient](#walletclient)
  * [ListWalletInfo](#listwalletinfo)
  * [ListWalletInfoByWallet](#listwalletinfobywallet)
  * [WalletHas](#wallethas)
  * [WalletSign](#walletsign)
* [WalletServiceProvider](#walletserviceprovider)
  * [AddNewAddress](#addnewaddress)
  * [ListenWalletEvent](#listenwalletevent)
  * [RemoveAddress](#removeaddress)
  * [ResponseWalletEvent](#responsewalletevent)
  * [SupportNewAccount](#supportnewaccount)

## Gateway

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

## MarketClient

### IsUnsealed


Perms: admin

Inputs:
```json
[
  "f01234",
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  {
    "ID": {
      "Miner": 0,
      "Number": 0
    },
    "ProofType": 0
  },
  10,
  1032
]
```

Response: `true`

### ListMarketConnectionsState


Perms: admin

Inputs: `[]`

Response:
```json
[
  {
    "Addr": "f01234",
    "Conn": {
      "Connections": [
        {
          "Addrs": [
            "f01234"
          ],
          "ChannelId": "00000000-0000-0000-0000-000000000000",
          "Ip": "",
          "RequestCount": 0,
          "CreateTime": "0001-01-01T00:00:00Z"
        }
      ],
      "ConnectionCount": 0
    }
  }
]
```

### SectorsUnsealPiece


Perms: admin

Inputs:
```json
[
  "f01234",
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  {
    "ID": {
      "Miner": 0,
      "Number": 0
    },
    "ProofType": 0
  },
  10,
  1032,
  "string value"
]
```

Response: `{}`

## MarketServiceProvider

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

### ResponseMarketEvent


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

## ProofClient

### ComputeProof


Perms: admin

Inputs:
```json
[
  "f01234",
  [
    {
      "SealProof": 0,
      "SectorNumber": 0,
      "SectorKey": null,
      "SealedCID": null
    }
  ],
  "Bw==",
  10101,
  17
]
```

Response:
```json
[
  {
    "PoStProof": 0,
    "ProofBytes": null
  }
]
```

### ListConnectedMiners


Perms: admin

Inputs: `[]`

Response:
```json
[
  "f01234"
]
```

### ListMinerConnection


Perms: admin

Inputs:
```json
[
  "f01234"
]
```

Response:
```json
{
  "Connections": [
    {
      "Addrs": [
        "f01234"
      ],
      "ChannelId": "00000000-0000-0000-0000-000000000000",
      "Ip": "",
      "RequestCount": 0,
      "CreateTime": "0001-01-01T00:00:00Z"
    }
  ],
  "ConnectionCount": 0
}
```

## ProofServiceProvider

### ListenProofEvent


Perms: read

Inputs:
```json
[
  {
    "MinerAddress": "\u003cempty\u003e"
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

### ResponseProofEvent


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

## WalletClient

### ListWalletInfo


Perms: admin

Inputs: `[]`

Response:
```json
[
  {
    "Account": "string value",
    "SupportAccounts": null,
    "ConnectStates": null
  }
]
```

### ListWalletInfoByWallet


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response:
```json
{
  "Account": "string value",
  "SupportAccounts": null,
  "ConnectStates": null
}
```

### WalletHas


Perms: admin

Inputs:
```json
[
  "string value",
  "f01234"
]
```

Response: `true`

### WalletSign


Perms: admin

Inputs:
```json
[
  "string value",
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

## WalletServiceProvider

### AddNewAddress


Perms: read

Inputs:
```json
[
  "e26f1e5c-47f7-4561-a11d-18fab6e748af",
  [
    "f01234"
  ]
]
```

Response: `{}`

### ListenWalletEvent


Perms: read

Inputs:
```json
[
  {
    "SupportAccounts": null,
    "SignBytes": null
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

### RemoveAddress


Perms: read

Inputs:
```json
[
  "e26f1e5c-47f7-4561-a11d-18fab6e748af",
  [
    "f01234"
  ]
]
```

Response: `{}`

### ResponseWalletEvent


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

### SupportNewAccount


Perms: read

Inputs:
```json
[
  "e26f1e5c-47f7-4561-a11d-18fab6e748af",
  "string value"
]
```

Response: `{}`

